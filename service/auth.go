package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/basketikun/infinite-canvas/config"
	"github.com/basketikun/infinite-canvas/model"
	"github.com/basketikun/infinite-canvas/repository"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type TokenClaims struct {
	UserID   string         `json:"userId"`
	Username string         `json:"username"`
	Role     model.UserRole `json:"role"`
	jwt.RegisteredClaims
}

type userExtra struct {
	LinuxDo any `json:"linuxDo,omitempty"`
}

const (
	registerGiftCredits     = 30
	inviteRegisterBonusRate = 10
)

func EnsureDefaultAdmin() error {
	if strings.TrimSpace(config.Cfg.AdminUsername) == "" || strings.TrimSpace(config.Cfg.AdminPassword) == "" {
		return nil
	}
	WarnDefaultSecurityConfig()
	hasAdmin, err := repository.HasAdmin()
	if err != nil || hasAdmin {
		return err
	}
	hash, err := hashPassword(config.Cfg.AdminPassword)
	if err != nil {
		return err
	}
	_, err = repository.SaveUser(model.User{
		ID:        newID("user"),
		Username:  strings.TrimSpace(config.Cfg.AdminUsername),
		Password:  hash,
		Role:      model.UserRoleAdmin,
		AffCode:   newAffCode(),
		Status:    model.UserStatusActive,
		CreatedAt: now(),
		UpdatedAt: now(),
	})
	return err
}

func Register(username string, password string, email string, verificationCode string, inviteCodes ...string) (model.AuthSession, error) {
	settings, err := repository.GetSettings()
	if err != nil {
		return model.AuthSession{}, err
	}
	normalizedSettings := normalizeSettings(settings)
	if normalizedSettings.Public.Auth.AllowRegister != nil && !*normalizedSettings.Public.Auth.AllowRegister {
		return model.AuthSession{}, safeMessageError{message: "当前未开放注册"}
	}
	username = strings.TrimSpace(username)
	if strings.ContainsAny(username, " \t\r\n") {
		return model.AuthSession{}, safeMessageError{message: "用户名不能包含空格"}
	}
	if username == "" || password == "" {
		return model.AuthSession{}, safeMessageError{message: "用户名和密码不能为空"}
	}
	email, err = normalizeVerificationEmail(email)
	if err != nil {
		return model.AuthSession{}, err
	}
	if _, ok, err := repository.GetUserByUsername(username); err != nil || ok {
		if err != nil {
			return model.AuthSession{}, err
		}
		return model.AuthSession{}, safeMessageError{message: "用户名已存在"}
	}
	if _, ok, err := repository.GetUserByEmail(email); err != nil || ok {
		if err != nil {
			return model.AuthSession{}, err
		}
		return model.AuthSession{}, safeMessageError{message: "邮箱已注册"}
	}
	var inviter model.User
	inviteCode := ""
	if len(inviteCodes) > 0 {
		inviteCode = strings.ToUpper(strings.TrimSpace(inviteCodes[0]))
	}
	if inviteCode != "" {
		var ok bool
		inviter, ok, err = repository.GetUserByAffCode(inviteCode)
		if err != nil {
			return model.AuthSession{}, err
		}
		if !ok {
			return model.AuthSession{}, safeMessageError{message: "邀请码无效"}
		}
	}
	if err := consumeVerificationCode(email, model.VerificationPurposeRegister, verificationCode); err != nil {
		return model.AuthSession{}, err
	}
	hash, err := hashPassword(password)
	if err != nil {
		return model.AuthSession{}, err
	}
	current := now()
	inviteBonusCredits := 0
	if inviter.ID != "" {
		inviteBonusCredits = registerGiftCredits * inviteRegisterBonusRate / 100
	}
	userValue := model.User{
		ID:        newID("user"),
		Username:  username,
		Password:  hash,
		Email:     email,
		Role:      model.UserRoleUser,
		AffCode:   newAffCode(),
		Status:    model.UserStatusActive,
		Credits:   registerGiftCredits + inviteBonusCredits,
		CreatedAt: current,
		UpdatedAt: current,
	}
	if inviter.ID != "" {
		userValue.InviterID = inviter.ID
	}
	user, err := repository.SaveUser(userValue)
	if err != nil {
		return model.AuthSession{}, err
	}
	if inviter.ID != "" {
		inviteRewardCredits := 0
		if normalizedSettings.Public.Auth.InviteRewardCredits != nil {
			inviteRewardCredits = *normalizedSettings.Public.Auth.InviteRewardCredits
		}
		if _, err := repository.RewardUserInvitation(inviter.ID, user.ID, inviteRewardCredits, current); err != nil {
			return model.AuthSession{}, err
		}
	}
	_, _ = repository.SaveCreditLog(model.CreditLog{
		ID:        newID("credit"),
		UserID:    user.ID,
		Type:      model.CreditLogTypeRegisterGift,
		Amount:    registerGiftCredits,
		Balance:   registerGiftCredits,
		Remark:    "新用户注册赠送",
		CreatedAt: current,
	})
	if inviteBonusCredits > 0 {
		_, _ = repository.SaveCreditLog(model.CreditLog{
			ID:        newID("credit"),
			UserID:    user.ID,
			Type:      model.CreditLogTypeInviteRegisterBonus,
			Amount:    inviteBonusCredits,
			Balance:   user.Credits,
			RelatedID: inviter.ID,
			Remark:    "邀请注册额外赠送",
			CreatedAt: current,
		})
	}
	return newSession(user)
}

func Login(username string, password string) (model.AuthSession, error) {
	user, ok, err := repository.GetUserByUsername(strings.TrimSpace(username))
	if err != nil {
		return model.AuthSession{}, err
	}
	if !ok || bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)) != nil {
		return model.AuthSession{}, safeMessageError{message: "用户名或密码错误"}
	}
	if user.Status == model.UserStatusBan {
		return model.AuthSession{}, safeMessageError{message: "账号已被禁用"}
	}
	normalizeUserDefaults(&user)
	user.LastLoginAt = now()
	user.UpdatedAt = now()
	user, err = repository.SaveUser(user)
	if err != nil {
		return model.AuthSession{}, err
	}
	return newSession(user)
}

func LinuxDoAuthorizeURL(r *http.Request, redirect string) (string, error) {
	settings, err := repository.GetSettings()
	if err != nil {
		return "", err
	}
	settings = normalizeSettings(settings)
	linuxDo := settings.Private.Auth.LinuxDo
	if !settings.Public.Auth.LinuxDo.Enabled {
		return "", safeMessageError{message: "Linux.do 登录未开启"}
	}
	if strings.TrimSpace(linuxDo.ClientID) == "" || strings.TrimSpace(linuxDo.ClientSecret) == "" {
		return "", safeMessageError{message: "Linux.do 登录未配置"}
	}
	values := url.Values{}
	values.Set("client_id", linuxDo.ClientID)
	values.Set("redirect_uri", linuxDoRedirectURI(r))
	values.Set("response_type", "code")
	values.Set("scope", "read")
	values.Set("state", base64.RawURLEncoding.EncodeToString([]byte(redirect)))
	return config.Cfg.LinuxDoAuthorizeURL + "?" + values.Encode(), nil
}

func LoginWithLinuxDo(r *http.Request, code string, state string) (model.AuthSession, string, error) {
	redirect := decodeState(state)
	settings, err := repository.GetSettings()
	if err != nil {
		return model.AuthSession{}, redirect, err
	}
	settings = normalizeSettings(settings)
	linuxDo := settings.Private.Auth.LinuxDo
	if !settings.Public.Auth.LinuxDo.Enabled {
		return model.AuthSession{}, redirect, safeMessageError{message: "Linux.do 登录未开启"}
	}
	token, err := linuxDoAccessToken(r, code, linuxDo)
	if err != nil {
		return model.AuthSession{}, redirect, err
	}
	profile, err := linuxDoProfile(token)
	if err != nil {
		return model.AuthSession{}, redirect, err
	}
	linuxDoID := fmt.Sprint(profile.ID)
	if strings.TrimSpace(linuxDoID) == "" || linuxDoID == "0" {
		return model.AuthSession{}, redirect, safeMessageError{message: "Linux.do 用户信息无效"}
	}
	user, ok, err := repository.GetUserByLinuxDoID(linuxDoID)
	if err != nil {
		return model.AuthSession{}, redirect, err
	}
	if !ok {
		if settings.Public.Auth.AllowRegister != nil && !*settings.Public.Auth.AllowRegister {
			return model.AuthSession{}, redirect, safeMessageError{message: "当前未开放注册"}
		}
		user = model.User{
			ID:          newID("user"),
			Username:    linuxDoUsername(profile.Username, linuxDoID),
			DisplayName: strings.TrimSpace(profile.Name),
			AvatarURL:   linuxDoAvatar(profile.AvatarTemplate),
			Role:        model.UserRoleUser,
			AffCode:     newAffCode(),
			LinuxDoID:   linuxDoID,
			Status:      model.UserStatusActive,
			Credits:     30,
			CreatedAt:   now(),
		}
	} else if user.Status == model.UserStatusBan {
		return model.AuthSession{}, redirect, safeMessageError{message: "账号已被禁用"}
	}
	user.DisplayName = firstNonEmpty(profile.Name, user.DisplayName)
	user.AvatarURL = firstNonEmpty(linuxDoAvatar(profile.AvatarTemplate), user.AvatarURL)
	user.LastLoginAt = now()
	user.UpdatedAt = now()
	extra, _ := json.Marshal(userExtra{LinuxDo: profile})
	user.Extra = string(extra)
	isNewUser := !ok
	user, err = repository.SaveUser(user)
	if err != nil {
		return model.AuthSession{}, redirect, err
	}
	if isNewUser {
		_, _ = repository.SaveCreditLog(model.CreditLog{
			ID:        newID("credit"),
			UserID:    user.ID,
			Type:      model.CreditLogTypeRegisterGift,
			Amount:    30,
			Balance:   30,
			Remark:    "新用户注册赠送",
			CreatedAt: now(),
		})
	}
	session, err := newSession(user)
	return session, redirect, err
}

func ParseToken(tokenText string) (TokenClaims, error) {
	claims := TokenClaims{}
	token, err := jwt.ParseWithClaims(tokenText, &claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("登录状态无效")
		}
		return []byte(config.Cfg.JWTSecret), nil
	})
	if err != nil || !token.Valid {
		return TokenClaims{}, errors.New("登录状态无效")
	}
	return claims, nil
}

func CurrentAuthUser(tokenText string) (model.AuthUser, bool) {
	claims, err := ParseToken(tokenText)
	if err != nil {
		return model.AuthUser{}, false
	}
	user, ok, err := repository.GetUserByID(claims.UserID)
	if err != nil || !ok {
		return model.AuthUser{}, false
	}
	if user.Status == model.UserStatusBan {
		return model.AuthUser{}, false
	}
	if user.AffCode == "" {
		normalizeUserDefaults(&user)
		user.UpdatedAt = now()
		user, err = repository.SaveUser(user)
		if err != nil {
			return model.AuthUser{}, false
		}
	}
	return model.PublicUser(user), true
}

func ListUsers(q model.Query) (model.UserList, error) {
	users, total, err := repository.ListUsers(q)
	if err != nil {
		return model.UserList{}, err
	}
	for i := range users {
		users[i].Password = ""
		normalizeUserDefaults(&users[i])
	}
	return model.UserList{Items: users, Total: int(total)}, nil
}

func ListInvitationRecords(q model.Query) (model.InvitationRecordList, error) {
	records, total, err := repository.ListInvitationRecords("", q)
	if err != nil {
		return model.InvitationRecordList{}, err
	}
	return model.InvitationRecordList{Items: records, Total: int(total)}, nil
}

func ListUserInvitationRecords(userID string, q model.Query) (model.InvitationRecordList, error) {
	records, total, err := repository.ListInvitationRecords(userID, q)
	if err != nil {
		return model.InvitationRecordList{}, err
	}
	return model.InvitationRecordList{Items: records, Total: int(total)}, nil
}

func SaveUser(user model.User, password string) (model.User, error) {
	user.Username = strings.TrimSpace(user.Username)
	if strings.ContainsAny(user.Username, " \t\r\n") {
		return user, safeMessageError{message: "用户名不能包含空格"}
	}
	if user.Username == "" {
		return user, safeMessageError{message: "用户名不能为空"}
	}
	user.Email = strings.ToLower(strings.TrimSpace(user.Email))
	if user.Email != "" {
		if _, err := normalizeVerificationEmail(user.Email); err != nil {
			return user, err
		}
	}
	if user.Role == "" || user.Role == model.UserRoleGuest {
		user.Role = model.UserRoleUser
	}
	if user.Status == "" {
		user.Status = model.UserStatusActive
	}
	if saved, ok, err := repository.GetUserByUsername(user.Username); err != nil {
		return user, err
	} else if ok && saved.ID != user.ID {
		return user, safeMessageError{message: "用户名已存在"}
	}
	if user.Email != "" {
		if saved, ok, err := repository.GetUserByEmail(user.Email); err != nil {
			return user, err
		} else if ok && saved.ID != user.ID {
			return user, safeMessageError{message: "邮箱已注册"}
		}
	}
	isCreate := user.ID == ""
	if isCreate {
		user.ID = newID("user")
		user.AffCode = newAffCode()
		user.CreatedAt = now()
	} else if saved, ok, err := repository.GetUserByID(user.ID); err != nil {
		return user, err
	} else if ok {
		user.CreatedAt = saved.CreatedAt
		user.Password = saved.Password
		user.AvatarURL = saved.AvatarURL
		user.Credits = saved.Credits
		user.Extra = saved.Extra
		if user.AffCode == "" {
			user.AffCode = saved.AffCode
		}
		if user.AffCode == "" {
			user.AffCode = newAffCode()
		}
		if user.LinuxDoID == "" {
			user.LinuxDoID = saved.LinuxDoID
		}
		user.LastLoginAt = saved.LastLoginAt
	}
	if password != "" {
		hash, err := hashPassword(password)
		if err != nil {
			return user, err
		}
		user.Password = hash
	}
	if isCreate && user.Password == "" {
		return user, safeMessageError{message: "密码不能为空"}
	}
	user.UpdatedAt = now()
	user, err := repository.SaveUser(user)
	user.Password = ""
	return user, err
}

func AdjustUserCredits(id string, credits int) (model.User, error) {
	user, ok, err := repository.GetUserByID(id)
	if err != nil || !ok {
		if err != nil {
			return user, err
		}
		return user, safeMessageError{message: "用户不存在"}
	}
	oldCredits := user.Credits
	user.Credits = credits
	user.UpdatedAt = now()
	user, err = repository.SaveUser(user)
	if err == nil && oldCredits != credits {
		_, err = repository.SaveCreditLog(model.CreditLog{
			ID:        newID("credit"),
			UserID:    user.ID,
			Type:      model.CreditLogTypeAdminAdjust,
			Amount:    credits - oldCredits,
			Balance:   credits,
			Remark:    "后台手动调整",
			CreatedAt: now(),
		})
	}
	user.Password = ""
	return user, err
}

func ConsumeUserCredits(userID string, modelName string, credits int, path string) error {
	if credits <= 0 {
		return nil
	}
	user, ok, err := repository.ConsumeUserCredits(userID, credits, now())
	if err != nil {
		return err
	}
	if !ok {
		return safeMessageError{message: "积分不足"}
	}
	extra, _ := json.Marshal(map[string]string{"model": modelName, "path": path})
	_, err = repository.SaveCreditLog(model.CreditLog{
		ID:        newID("credit"),
		UserID:    userID,
		Type:      model.CreditLogTypeAIConsume,
		Amount:    -credits,
		Balance:   user.Credits - user.FrozenCredits,
		Remark:    "调用模型 " + modelName,
		Extra:     string(extra),
		CreatedAt: now(),
	})
	return err
}

func EnsureUserCredits(userID string, credits int) error {
	if credits <= 0 {
		return nil
	}
	user, ok, err := repository.GetUserByID(userID)
	if err != nil {
		return err
	}
	if !ok {
		return safeMessageError{message: "用户不存在"}
	}
	if user.Credits-user.FrozenCredits < credits {
		return safeMessageError{message: "积分不足"}
	}
	return nil
}

func FreezeAIImageCredits(userID string, modelName string, credits int, path string, prompt string, size string, quality string, count int, referenceCount int) (model.AIImageTask, error) {
	current := now()
	task := model.AIImageTask{
		ID:             newID("ai_image_task"),
		UserID:         userID,
		Model:          modelName,
		Path:           path,
		Prompt:         prompt,
		Credits:        credits,
		Size:           size,
		Quality:        quality,
		Count:          count,
		ReferenceCount: referenceCount,
		Status:         "reserved",
		CreatedAt:      current,
		UpdatedAt:      current,
	}
	task.TaskID = task.ID
	task, ok, err := repository.FreezeAIImageTask(task, current)
	if err != nil {
		return task, err
	}
	if !ok {
		return task, safeMessageError{message: "积分不足"}
	}
	return task, nil
}

func ConsumeAIImageCredits(userID string, modelName string, credits int, path string, prompt string, imageURL string, taskID string) error {
	if credits <= 0 {
		return nil
	}
	user, ok, err := repository.ConsumeUserCredits(userID, credits, now())
	if err != nil {
		return err
	}
	if !ok {
		return safeMessageError{message: "积分不足"}
	}
	if strings.TrimSpace(taskID) == "" {
		taskID = newID("sync_image")
	}
	extra, _ := json.Marshal(map[string]string{"model": modelName, "path": path, "prompt": prompt, "imageUrl": imageURL, "taskId": taskID})
	_, err = repository.SaveCreditLog(model.CreditLog{
		ID:        newID("credit"),
		UserID:    userID,
		Type:      model.CreditLogTypeAIConsume,
		Amount:    -credits,
		Balance:   user.Credits - user.FrozenCredits,
		RelatedID: taskID,
		Remark:    "图片生成 " + modelName,
		Extra:     string(extra),
		CreatedAt: now(),
	})
	return err
}

func RecordAIImageTask(userID string, taskID string, modelName string, path string, prompt string, credits int, status string, channel model.ModelChannel) error {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil
	}
	if saved, ok, err := repository.GetAIImageTaskByTaskID(taskID); err != nil {
		return err
	} else if ok {
		saved.Status = firstNonEmpty(status, saved.Status)
		saved.UpdatedAt = now()
		_, err = repository.SaveAIImageTask(saved)
		return err
	}
	current := now()
	_, err := repository.SaveAIImageTask(model.AIImageTask{
		ID:          newID("ai_image_task"),
		TaskID:      taskID,
		UserID:      userID,
		Model:       modelName,
		Path:        path,
		Prompt:      prompt,
		Credits:     credits,
		Status:      status,
		ChannelName: channel.Name,
		ChannelURL:  strings.TrimRight(strings.TrimSpace(channel.BaseURL), "/"),
		CreatedAt:   current,
		UpdatedAt:   current,
	})
	return err
}

func AttachAIImageTask(reservedTaskID string, upstreamTaskID string, status string, imageURL string, channel model.ModelChannel) (model.AIImageTask, error) {
	return repository.AttachAIImageTask(reservedTaskID, upstreamTaskID, status, imageURL, channel, now())
}

func GetAIImageTask(taskID string, userID string) (model.AIImageTask, bool, error) {
	task, ok, err := repository.GetAIImageTaskByTaskID(taskID)
	if err != nil || !ok {
		return task, ok, err
	}
	return task, task.UserID == userID, nil
}

func CompleteAIImageTaskSuccess(taskID string, userID string, status string, imageURL string) error {
	_, _, err := repository.CompleteAIImageTaskSuccess(taskID, userID, status, imageURL, now())
	if err != nil && err.Error() == "积分不足" {
		return safeMessageError{message: "积分不足"}
	}
	return err
}

func ReleaseAIImageTask(taskID string, userID string, status string) error {
	_, _, err := repository.ReleaseAIImageTask(taskID, userID, status, now())
	return err
}

func CheckFrozenAIImageTasks() {
	tasks, err := repository.ListFrozenAIImageTasks(100)
	if err != nil {
		log.Printf("list frozen AI image tasks failed err=%v", err)
		return
	}
	for _, task := range tasks {
		if err := checkFrozenAIImageTask(task); err != nil {
			log.Printf("check frozen AI image task failed task=%s user=%s err=%v", task.TaskID, task.UserID, err)
		}
	}
}

// frozenTaskReleaseGracePeriod 冻结任务连续状态查询出错多久后默认按失败释放，
// 避免上游 API 返 404/“生成失败”后积分永远被冻住卡住。
const frozenTaskReleaseGracePeriod = 30 * time.Minute

func checkFrozenAIImageTask(task model.AIImageTask) error {
	channel, err := FindModelChannel(task.Model, task.ChannelName, task.ChannelURL)
	if err != nil {
		return err
	}
	status, imageURL, err := fetchAIImageTaskState(channel, task)
	if err != nil {
		// 4xx 可能意味着上游明确告诉“这个 task 失败了 / 找不到了”；
		// 也可能只是临时限流 / 5xx。为了避免积分永远被冻住，超过 frozenTaskReleaseGracePeriod 后
		// 且是 4xx，则直接按失败释放；其他情况保留任务等下次 cron 重试。
		if !shouldReleaseFrozenTaskOnError(task, err) {
			return err
		}
		log.Printf("frozen AI image task exceeded grace period, releasing as failed task=%s err=%v", task.TaskID, err)
		return ReleaseAIImageTask(task.TaskID, task.UserID, "upstream_unreachable")
	}
	if isPendingAIImageTaskStatus(status) {
		return nil
	}
	if isFailedAIImageTaskStatus(status) {
		return ReleaseAIImageTask(task.TaskID, task.UserID, firstNonEmpty(status, "failed"))
	}
	if strings.TrimSpace(imageURL) != "" {
		ossURL, err := saveAIImageTaskResultToOSS(imageURL)
		if err != nil {
			log.Printf("save scheduled AI image result to OSS failed: task=%s url=%s err=%v", task.TaskID, imageURL, err)
			return ReleaseAIImageTask(task.TaskID, task.UserID, "oss_upload_failed")
		}
		return CompleteAIImageTaskSuccess(task.TaskID, task.UserID, firstNonEmpty(status, "succeeded"), ossURL)
	}
	if status != "" {
		return ReleaseAIImageTask(task.TaskID, task.UserID, "response_unrecognized")
	}
	return nil
}

// shouldReleaseFrozenTaskOnError 判断上游查询出错时，是否超过宽限期应该默认按失败释放。
// 仅在上游返 4xx 且任务已冻住超过 frozenTaskReleaseGracePeriod 时返回 true。
func shouldReleaseFrozenTaskOnError(task model.AIImageTask, fetchErr error) bool {
	if task.FrozenAt == "" || fetchErr == nil {
		return false
	}
	frozenAt, err := time.Parse(time.RFC3339Nano, task.FrozenAt)
	if err != nil {
		frozenAt, err = time.Parse(time.RFC3339, task.FrozenAt)
		if err != nil {
			return false
		}
	}
	if time.Since(frozenAt) < frozenTaskReleaseGracePeriod {
		return false
	}
	return strings.Contains(fetchErr.Error(), "upstream task status=4")
}

func fetchAIImageTaskState(channel model.ModelChannel, task model.AIImageTask) (string, string, error) {
	path := task.Path
	if strings.TrimSpace(path) == "" {
		path = "/image-tasks/" + task.TaskID
	} else if strings.HasSuffix(path, "/images/generations") {
		path = "/images/generations/" + task.TaskID
	} else if strings.HasSuffix(path, "/images/edits") {
		path = "/image-tasks/" + task.TaskID
	} else if !strings.Contains(path, task.TaskID) {
		path = strings.TrimRight(path, "/") + "/" + url.PathEscape(task.TaskID)
	}
	request, err := http.NewRequest(http.MethodGet, BuildModelChannelURL(channel, path), nil)
	if err != nil {
		return "", "", err
	}
	request.Header.Set("Authorization", "Bearer "+channel.APIKey)
	client := &http.Client{Timeout: 30 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return "", "", err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 30<<20))
	if err != nil {
		return "", "", err
	}
	if response.StatusCode >= http.StatusBadRequest {
		return "", "", fmt.Errorf("upstream task status=%d body=%s", response.StatusCode, safeUpstreamTaskText(body))
	}
	return parseAIImageTaskState(body), parseAIImageTaskImageURL(body), nil
}

func saveAIImageTaskResultToOSS(imageURL string) (string, error) {
	if strings.TrimSpace(imageURL) == "" || imageURL == "[b64_json]" {
		return imageURL, nil
	}
	uploaded, err := SaveRemoteImage(context.Background(), imageURL)
	if err != nil {
		return "", err
	}
	return uploaded.URL, nil
}

func parseAIImageTaskState(body []byte) string {
	var payload struct {
		Status string `json:"status"`
	}
	_ = json.Unmarshal(body, &payload)
	return strings.ToLower(strings.TrimSpace(payload.Status))
}

func parseAIImageTaskImageURL(body []byte) string {
	var payload struct {
		Data   []map[string]any `json:"data"`
		Result struct {
			Data []map[string]any `json:"data"`
		} `json:"result"`
	}
	_ = json.Unmarshal(body, &payload)
	items := payload.Data
	if len(items) == 0 {
		items = payload.Result.Data
	}
	for _, item := range items {
		if value, ok := item["url"].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
		if value, ok := item["b64_json"].(string); ok && strings.TrimSpace(value) != "" {
			return "[b64_json]"
		}
	}
	return ""
}

func isPendingAIImageTaskStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "queued", "running", "in_progress", "processing", "pending":
		return true
	default:
		return false
	}
}

func isFailedAIImageTaskStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "failed", "error", "canceled":
		return true
	default:
		return false
	}
}

func safeUpstreamTaskText(body []byte) string {
	text := strings.TrimSpace(string(body))
	if len([]rune(text)) <= 300 {
		return text
	}
	return string([]rune(text)[:300]) + "..."
}

func RefundUserCredits(userID string, modelName string, credits int, path string) error {
	if credits <= 0 {
		return nil
	}
	user, ok, err := repository.RefundUserCredits(userID, credits, now())
	if err != nil {
		return err
	}
	if !ok {
		return safeMessageError{message: "用户不存在"}
	}
	extra, _ := json.Marshal(map[string]string{"model": modelName, "path": path})
	_, err = repository.SaveCreditLog(model.CreditLog{
		ID:        newID("credit"),
		UserID:    userID,
		Type:      model.CreditLogTypeAIRefund,
		Amount:    credits,
		Balance:   user.Credits,
		Remark:    "模型调用失败返还 " + modelName,
		Extra:     string(extra),
		CreatedAt: now(),
	})
	return err
}

func ListCreditLogs(q model.Query) (model.CreditLogList, error) {
	logs, total, err := repository.ListCreditLogs(q)
	if err != nil {
		return model.CreditLogList{}, err
	}
	return model.CreditLogList{Items: logs, Total: int(total)}, nil
}

func ListAIDeductionLogs(q model.Query) (model.CreditLogList, error) {
	logs, total, err := repository.ListCreditLogsByTypes(deductionLogTypes(), q)
	if err != nil {
		return model.CreditLogList{}, err
	}
	return model.CreditLogList{Items: logs, Total: int(total)}, nil
}

func ListUserAIDeductionLogs(userID string, q model.Query) (model.CreditLogList, error) {
	logs, total, err := repository.ListUserCreditLogsByTypes(userID, deductionLogTypes(), q)
	if err != nil {
		return model.CreditLogList{}, err
	}
	return model.CreditLogList{Items: logs, Total: int(total)}, nil
}

func deductionLogTypes() []model.CreditLogType {
	return []model.CreditLogType{model.CreditLogTypeAIFreeze, model.CreditLogTypeAIConsume, model.CreditLogTypeAIFreezeRelease, model.CreditLogTypeInviteRegisterBonus, model.CreditLogTypeInviteReward}
}

func ListUserAIImageTasks(userID string, q model.Query) (model.AIImageTaskList, error) {
	tasks, total, err := repository.ListUserAIImageTasks(userID, q)
	if err != nil {
		return model.AIImageTaskList{}, err
	}
	return model.AIImageTaskList{Items: tasks, Total: int(total)}, nil
}

func ListAIImageTasks(q model.Query) (model.AIImageTaskList, error) {
	tasks, total, err := repository.ListAIImageTasks(q)
	if err != nil {
		return model.AIImageTaskList{}, err
	}
	return model.AIImageTaskList{Items: tasks, Total: int(total)}, nil
}

func ListFeaturedAIImageTasks(q model.Query) (model.AIImageTaskList, error) {
	tasks, total, err := repository.ListFeaturedAIImageTasks(q)
	if err != nil {
		return model.AIImageTaskList{}, err
	}
	return model.AIImageTaskList{Items: tasks, Total: int(total)}, nil
}

func UpdateAIImageTaskFeatured(taskID string, featured bool) (model.AIImageTask, error) {
	task, ok, err := repository.UpdateAIImageTaskFeatured(taskID, featured, now())
	if err != nil {
		return model.AIImageTask{}, err
	}
	if !ok {
		return model.AIImageTask{}, safeMessageError{message: "生图记录不存在"}
	}
	return task, nil
}

func SaveCreditLog(log model.CreditLog) (model.CreditLog, error) {
	if log.ID == "" {
		log.ID = newID("credit")
		log.CreatedAt = now()
	}
	return repository.SaveCreditLog(log)
}

func DeleteCreditLog(id string) error {
	return repository.DeleteCreditLog(id)
}

func DeleteUser(id string) error {
	return repository.DeleteUser(id)
}

func GuestUser() model.AuthUser {
	return model.AuthUser{ID: "", Username: "guest", Role: model.UserRoleGuest}
}

func newSession(user model.User) (model.AuthSession, error) {
	token, err := newToken(user)
	if err != nil {
		return model.AuthSession{}, err
	}
	return model.AuthSession{Token: token, User: model.PublicUser(user)}, nil
}

func newToken(user model.User) (string, error) {
	expireHours := config.Cfg.JWTExpireHours
	if expireHours <= 0 {
		expireHours = 168
	}
	claims := TokenClaims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(expireHours) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   user.ID,
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(config.Cfg.JWTSecret))
}

func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

func now() string {
	return time.Now().Format(time.RFC3339)
}

func newID(prefix string) string {
	return prefix + "-" + uuid.NewString()
}

func newAffCode() string {
	return strings.ToUpper(strings.ReplaceAll(uuid.NewString()[:8], "-", ""))
}

func normalizeUserDefaults(user *model.User) {
	if user.Status == "" {
		user.Status = model.UserStatusActive
	}
	if user.AffCode == "" {
		user.AffCode = newAffCode()
	}
}

type linuxDoTokenResponse struct {
	AccessToken string `json:"access_token"`
}

type linuxDoUserResponse struct {
	ID             int64  `json:"id"`
	Username       string `json:"username"`
	Name           string `json:"name"`
	AvatarTemplate string `json:"avatar_template"`
}

func linuxDoAccessToken(r *http.Request, code string, setting model.PrivateLinuxDoAuthSetting) (string, error) {
	values := url.Values{}
	values.Set("client_id", setting.ClientID)
	values.Set("client_secret", setting.ClientSecret)
	values.Set("grant_type", "authorization_code")
	values.Set("code", code)
	values.Set("redirect_uri", linuxDoRedirectURI(r))
	req, _ := http.NewRequest(http.MethodPost, config.Cfg.LinuxDoTokenURL, strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	var payload linuxDoTokenResponse
	if err := doLinuxDoJSON(req, &payload); err != nil {
		return "", err
	}
	if strings.TrimSpace(payload.AccessToken) == "" {
		return "", safeMessageError{message: "Linux.do 登录失败"}
	}
	return payload.AccessToken, nil
}

func linuxDoRedirectURI(r *http.Request) string {
	return RequestOrigin(r) + "/api/auth/linux-do/callback"
}

func linuxDoProfile(token string) (linuxDoUserResponse, error) {
	req, _ := http.NewRequest(http.MethodGet, config.Cfg.LinuxDoUserInfoURL, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	var payload linuxDoUserResponse
	err := doLinuxDoJSON(req, &payload)
	return payload, err
}

func doLinuxDoJSON(req *http.Request, payload any) error {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return safeMessageError{message: "Linux.do 登录失败"}
	}
	return json.NewDecoder(bytes.NewReader(body)).Decode(payload)
}

func linuxDoUsername(username string, id string) string {
	base := strings.TrimSpace(username)
	if base == "" {
		base = "linuxdo-" + id
	}
	if _, ok, err := repository.GetUserByUsername(base); err != nil || !ok {
		return base
	}
	return base + "-" + id
}

func linuxDoAvatar(template string) string {
	if strings.TrimSpace(template) == "" {
		return ""
	}
	if strings.HasPrefix(template, "//") {
		template = "https:" + template
	}
	if strings.HasPrefix(template, "/") {
		template = "https://linux.do" + template
	}
	return strings.ReplaceAll(template, "{size}", "120")
}

func decodeState(state string) string {
	data, err := base64.RawURLEncoding.DecodeString(state)
	if err != nil {
		return "/"
	}
	return safeRedirectPath(string(data))
}

// safeRedirectPath 仅放行站内相对路径，拦截开放重定向。浏览器会忽略 URL 中的
// Tab/换行/回车，并把 //host 或 /\host 解析为协议相对的跨站地址，因此先剥离这些
// 控制字符，再拒绝 // 与 /\ 前缀。
func safeRedirectPath(redirect string) string {
	cleaned := strings.Map(func(r rune) rune {
		if r == '\t' || r == '\n' || r == '\r' {
			return -1
		}
		return r
	}, redirect)
	if !strings.HasPrefix(cleaned, "/") || strings.HasPrefix(cleaned, "//") || strings.HasPrefix(cleaned, "/\\") {
		return "/"
	}
	return cleaned
}

func RequestOrigin(r *http.Request) string {
	host := strings.TrimSpace(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = r.Host
	}
	proto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto"))
	if proto == "" {
		proto = "http"
	}
	return proto + "://" + host
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func WarnDefaultSecurityConfig() {
	if config.Cfg.AdminUsername == "admin" && config.Cfg.AdminPassword == "infinite-canvas" {
		log.Println("WARNING: using default admin credentials, please set ADMIN_USERNAME and ADMIN_PASSWORD to safer values before deployment")
	}
}
