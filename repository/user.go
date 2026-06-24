package repository

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/basketikun/infinite-canvas/model"
	"gorm.io/gorm"
)

// ListUsers 分页查询用户。
func ListUsers(q model.Query) ([]model.User, int64, error) {
	db, err := DB()
	if err != nil {
		return nil, 0, err
	}
	q.Normalize()
	tx := db.Model(&model.User{})
	if keyword := strings.TrimSpace(q.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		tx = tx.Where("id LIKE ? OR username LIKE ? OR display_name LIKE ? OR email LIKE ? OR linux_do_id LIKE ?", like, like, like, like, like)
	}

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var users []model.User
	err = tx.Order("created_at desc").Offset(q.Offset()).Limit(q.PageSize).Find(&users).Error
	return users, total, err
}

// CountUsers 返回用户总数。
func CountUsers() (int64, error) {
	db, err := DB()
	if err != nil {
		return 0, err
	}
	var total int64
	return total, db.Model(&model.User{}).Count(&total).Error
}

// HasAdmin 判断系统中是否存在管理员。
func HasAdmin() (bool, error) {
	db, err := DB()
	if err != nil {
		return false, err
	}
	var total int64
	err = db.Model(&model.User{}).Where("role = ?", model.UserRoleAdmin).Count(&total).Error
	return total > 0, err
}

// GetUserByID 根据 ID 查询用户。
func GetUserByID(id string) (model.User, bool, error) {
	db, err := DB()
	if err != nil {
		return model.User{}, false, err
	}
	return findUser(db, "id = ?", id)
}

// GetUserByUsername 根据用户名查询用户。
func GetUserByUsername(username string) (model.User, bool, error) {
	db, err := DB()
	if err != nil {
		return model.User{}, false, err
	}
	return findUser(db, "username = ?", username)
}

// GetUserByEmail 根据邮箱查询用户。
func GetUserByEmail(email string) (model.User, bool, error) {
	db, err := DB()
	if err != nil {
		return model.User{}, false, err
	}
	return findUser(db, "email = ?", email)
}

// SaveUser 保存用户信息。
func SaveUser(user model.User) (model.User, error) {
	db, err := DB()
	if err != nil {
		return user, err
	}
	return user, db.Save(&user).Error
}

func ConsumeUserCredits(id string, credits int, now string) (model.User, bool, error) {
	db, err := DB()
	if err != nil {
		return model.User{}, false, err
	}
	if credits <= 0 {
		user, ok, err := GetUserByID(id)
		return user, ok, err
	}
	tx := db.Model(&model.User{}).Where("id = ? AND credits - frozen_credits >= ?", id, credits).Updates(map[string]any{
		"credits":    gorm.Expr("credits - ?", credits),
		"updated_at": now,
	})
	if tx.Error != nil {
		return model.User{}, false, tx.Error
	}
	user, ok, err := GetUserByID(id)
	return user, ok && tx.RowsAffected > 0, err
}

func RefundUserCredits(id string, credits int, now string) (model.User, bool, error) {
	db, err := DB()
	if err != nil {
		return model.User{}, false, err
	}
	if credits <= 0 {
		user, ok, err := GetUserByID(id)
		return user, ok, err
	}
	tx := db.Model(&model.User{}).Where("id = ?", id).Updates(map[string]any{
		"credits":    gorm.Expr("credits + ?", credits),
		"updated_at": now,
	})
	if tx.Error != nil {
		return model.User{}, false, tx.Error
	}
	user, ok, err := GetUserByID(id)
	return user, ok && tx.RowsAffected > 0, err
}

// SaveCreditLog 保存算力点变更流水。
func SaveCreditLog(log model.CreditLog) (model.CreditLog, error) {
	db, err := DB()
	if err != nil {
		return log, err
	}
	return log, db.Save(&log).Error
}

func SaveAIImageTask(task model.AIImageTask) (model.AIImageTask, error) {
	db, err := DB()
	if err != nil {
		return task, err
	}
	return task, db.Save(&task).Error
}

func GetAIImageTaskByTaskID(taskID string) (model.AIImageTask, bool, error) {
	db, err := DB()
	if err != nil {
		return model.AIImageTask{}, false, err
	}
	task := model.AIImageTask{}
	if err := db.Where("task_id = ?", taskID).First(&task).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.AIImageTask{}, false, nil
		}
		return model.AIImageTask{}, false, err
	}
	return task, true, nil
}

func ListFrozenAIImageTasks(limit int) ([]model.AIImageTask, error) {
	db, err := DB()
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}
	var tasks []model.AIImageTask
	err = db.Where("frozen_at <> ? AND charged_at = ? AND released_at = ?", "", "", "").Order("frozen_at asc").Limit(limit).Find(&tasks).Error
	return tasks, err
}

func AttachAIImageTask(reservedTaskID string, upstreamTaskID string, status string, imageURL string, channel model.ModelChannel, now string) (model.AIImageTask, error) {
	db, err := DB()
	if err != nil {
		return model.AIImageTask{}, err
	}
	task := model.AIImageTask{}
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("task_id = ?", reservedTaskID).First(&task).Error; err != nil {
			return err
		}
		if strings.TrimSpace(upstreamTaskID) != "" {
			task.TaskID = strings.TrimSpace(upstreamTaskID)
		}
		if strings.TrimSpace(status) != "" {
			task.Status = status
		}
		if strings.TrimSpace(imageURL) != "" {
			task.ImageURL = imageURL
		}
		task.ChannelName = channel.Name
		task.ChannelURL = strings.TrimRight(strings.TrimSpace(channel.BaseURL), "/")
		task.UpdatedAt = now
		return tx.Save(&task).Error
	})
	return task, err
}

func UpdateAIImageTaskStatus(taskID string, userID string, status string, imageURL string, now string) (model.AIImageTask, bool, error) {
	db, err := DB()
	if err != nil {
		return model.AIImageTask{}, false, err
	}
	task := model.AIImageTask{}
	if err := db.Where("task_id = ? AND user_id = ?", taskID, userID).First(&task).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.AIImageTask{}, false, nil
		}
		return model.AIImageTask{}, false, err
	}
	if strings.TrimSpace(status) != "" {
		task.Status = status
	}
	if strings.TrimSpace(imageURL) != "" {
		task.ImageURL = imageURL
	}
	task.UpdatedAt = now
	return task, true, db.Save(&task).Error
}

func FreezeAIImageTask(task model.AIImageTask, now string) (model.AIImageTask, bool, error) {
	db, err := DB()
	if err != nil {
		return task, false, err
	}
	if task.Credits <= 0 {
		task.FrozenAt = now
		task.CreatedAt = now
		task.UpdatedAt = now
		saved, err := SaveAIImageTask(task)
		return saved, true, err
	}
	frozen := false
	err = db.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.User{}).Where("id = ? AND credits - frozen_credits >= ?", task.UserID, task.Credits).Updates(map[string]any{
			"frozen_credits": gorm.Expr("frozen_credits + ?", task.Credits),
			"updated_at":     now,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		var user model.User
		if err := tx.Where("id = ?", task.UserID).First(&user).Error; err != nil {
			return err
		}
		task.FrozenAt = now
		task.CreatedAt = now
		task.UpdatedAt = now
		if task.Status == "" {
			task.Status = "reserved"
		}
		if err := tx.Save(&task).Error; err != nil {
			return err
		}
		if err := tx.Save(&model.CreditLog{
			ID:        "credit_freeze_" + task.ID,
			UserID:    task.UserID,
			Type:      model.CreditLogTypeAIFreeze,
			Amount:    0,
			Balance:   user.Credits - user.FrozenCredits,
			RelatedID: task.TaskID,
			Remark:    "冻结图片生成算力",
			Extra:     aiImageCreditLogExtra(task, ""),
			CreatedAt: now,
		}).Error; err != nil {
			return err
		}
		frozen = true
		return nil
	})
	return task, frozen, err
}

func CompleteAIImageTaskSuccess(taskID string, userID string, status string, imageURL string, now string) (model.AIImageTask, bool, error) {
	db, err := DB()
	if err != nil {
		return model.AIImageTask{}, false, err
	}
	charged := false
	task := model.AIImageTask{}
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("task_id = ? AND user_id = ?", taskID, userID).First(&task).Error; err != nil {
			return err
		}
		if task.ReleasedAt != "" {
			return nil
		}
		task.Status = status
		task.ImageURL = imageURL
		task.UpdatedAt = now
		if task.ChargedAt != "" || task.Credits <= 0 {
			return tx.Save(&task).Error
		}
		result := tx.Model(&model.AIImageTask{}).Where("id = ? AND charged_at = ? AND released_at = ?", task.ID, "", "").Updates(map[string]any{
			"status":     status,
			"image_url":  imageURL,
			"charged_at": now,
			"updated_at": now,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return tx.Where("id = ?", task.ID).First(&task).Error
		}
		task.ChargedAt = now
		updates := map[string]any{"credits": gorm.Expr("credits - ?", task.Credits), "updated_at": now}
		query := tx.Model(&model.User{}).Where("id = ?", userID)
		if task.FrozenAt != "" {
			query = query.Where("credits >= ? AND frozen_credits >= ?", task.Credits, task.Credits)
			updates["frozen_credits"] = gorm.Expr("frozen_credits - ?", task.Credits)
		} else {
			query = query.Where("credits - frozen_credits >= ?", task.Credits)
		}
		userResult := query.Updates(updates)
		if userResult.Error != nil {
			return userResult.Error
		}
		if userResult.RowsAffected == 0 {
			return errors.New("算力点不足")
		}
		var user model.User
		if err := tx.Where("id = ?", userID).First(&user).Error; err != nil {
			return err
		}
		if err := tx.Save(&model.CreditLog{
			ID:        "credit_" + task.ID,
			UserID:    userID,
			Type:      model.CreditLogTypeAIConsume,
			Amount:    -task.Credits,
			Balance:   user.Credits - user.FrozenCredits,
			RelatedID: task.TaskID,
			Remark:    "图片生成 " + task.Model,
			Extra:     aiImageCreditLogExtra(task, task.ImageURL),
			CreatedAt: now,
		}).Error; err != nil {
			return err
		}
		charged = true
		return nil
	})
	return task, charged, err
}

func ReleaseAIImageTask(taskID string, userID string, status string, now string) (model.AIImageTask, bool, error) {
	db, err := DB()
	if err != nil {
		return model.AIImageTask{}, false, err
	}
	released := false
	task := model.AIImageTask{}
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("task_id = ? AND user_id = ?", taskID, userID).First(&task).Error; err != nil {
			return err
		}
		if task.ReleasedAt != "" || task.ChargedAt != "" {
			return nil
		}
		task.Status = status
		task.UpdatedAt = now
		if task.FrozenAt == "" || task.Credits <= 0 {
			return tx.Save(&task).Error
		}
		result := tx.Model(&model.AIImageTask{}).Where("id = ? AND charged_at = ? AND released_at = ?", task.ID, "", "").Updates(map[string]any{
			"status":      status,
			"released_at": now,
			"updated_at":  now,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return tx.Where("id = ?", task.ID).First(&task).Error
		}
		task.ReleasedAt = now
		userResult := tx.Model(&model.User{}).Where("id = ? AND frozen_credits >= ?", userID, task.Credits).Updates(map[string]any{
			"frozen_credits": gorm.Expr("frozen_credits - ?", task.Credits),
			"updated_at":     now,
		})
		if userResult.Error != nil {
			return userResult.Error
		}
		if userResult.RowsAffected == 0 {
			return errors.New("冻结算力点不足")
		}
		var user model.User
		if err := tx.Where("id = ?", userID).First(&user).Error; err != nil {
			return err
		}
		if err := tx.Save(&model.CreditLog{
			ID:        "credit_release_" + task.ID,
			UserID:    userID,
			Type:      model.CreditLogTypeAIFreezeRelease,
			Amount:    0,
			Balance:   user.Credits - user.FrozenCredits,
			RelatedID: task.TaskID,
			Remark:    "释放图片生成冻结算力",
			Extra:     aiImageCreditLogExtra(task, task.ImageURL),
			CreatedAt: now,
		}).Error; err != nil {
			return err
		}
		released = true
		return nil
	})
	return task, released, err
}

func aiImageCreditLogExtra(task model.AIImageTask, imageURL string) string {
	extra, _ := json.Marshal(map[string]any{
		"model":         task.Model,
		"path":          task.Path,
		"prompt":        task.Prompt,
		"imageUrl":      imageURL,
		"taskId":        task.TaskID,
		"frozenCredits": task.Credits,
	})
	return string(extra)
}

func SaveRechargeOrder(order model.RechargeOrder) (model.RechargeOrder, error) {
	db, err := DB()
	if err != nil {
		return order, err
	}
	return order, db.Save(&order).Error
}

func GetRechargeOrderByID(id string) (model.RechargeOrder, bool, error) {
	db, err := DB()
	if err != nil {
		return model.RechargeOrder{}, false, err
	}
	return findRechargeOrder(db, "id = ?", id)
}

func GetRechargeOrderByOutTradeNo(outTradeNo string) (model.RechargeOrder, bool, error) {
	db, err := DB()
	if err != nil {
		return model.RechargeOrder{}, false, err
	}
	return findRechargeOrder(db, "out_trade_no = ?", outTradeNo)
}

func CompleteRechargeOrderPaid(outTradeNo string, amountFen int, transactionID string, now string) (bool, error) {
	db, err := DB()
	if err != nil {
		return false, err
	}
	paid := false
	err = db.Transaction(func(tx *gorm.DB) error {
		var order model.RechargeOrder
		if err := tx.Where("out_trade_no = ?", outTradeNo).First(&order).Error; err != nil {
			return err
		}
		if order.AmountFen != amountFen {
			return errors.New("订单金额不匹配")
		}
		if order.Status == model.RechargeOrderStatusPaid {
			return nil
		}
		result := tx.Model(&model.RechargeOrder{}).Where("id = ? AND status <> ?", order.ID, model.RechargeOrderStatusPaid).Updates(map[string]any{
			"status":         model.RechargeOrderStatusPaid,
			"transaction_id": transactionID,
			"paid_at":        now,
			"updated_at":     now,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		if err := tx.Model(&model.User{}).Where("id = ?", order.UserID).Updates(map[string]any{
			"credits":                   gorm.Expr("credits + ?", order.Credits),
			"member_type":               order.MemberType,
			"member_level":              order.MemberLevel,
			"last_recharge_amount_yuan": order.AmountYuan,
			"last_recharged_at":         now,
			"updated_at":                now,
		}).Error; err != nil {
			return err
		}
		var user model.User
		if err := tx.Where("id = ?", order.UserID).First(&user).Error; err != nil {
			return err
		}
		if err := tx.Save(&model.CreditLog{
			ID:        "credit_" + order.ID,
			UserID:    order.UserID,
			Type:      model.CreditLogTypeRecharge,
			Amount:    order.Credits,
			Balance:   user.Credits,
			RelatedID: order.ID,
			Remark:    "微信充值",
			CreatedAt: now,
		}).Error; err != nil {
			return err
		}
		paid = true
		return nil
	})
	return paid, err
}

func ListCreditLogs(q model.Query) ([]model.CreditLog, int64, error) {
	db, err := DB()
	if err != nil {
		return nil, 0, err
	}
	q.Normalize()
	tx := db.Model(&model.CreditLog{})
	if keyword := strings.TrimSpace(q.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		tx = tx.Where("user_id LIKE ? OR type LIKE ? OR remark LIKE ? OR related_id LIKE ?", like, like, like, like)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var logs []model.CreditLog
	err = tx.Order("created_at desc").Offset(q.Offset()).Limit(q.PageSize).Find(&logs).Error
	return logs, total, err
}

func ListCreditLogsByType(logType model.CreditLogType, q model.Query) ([]model.CreditLog, int64, error) {
	return ListCreditLogsByTypes([]model.CreditLogType{logType}, q)
}

func ListCreditLogsByTypes(logTypes []model.CreditLogType, q model.Query) ([]model.CreditLog, int64, error) {
	return listCreditLogsByTypes("", logTypes, q)
}

func ListUserCreditLogsByTypes(userID string, logTypes []model.CreditLogType, q model.Query) ([]model.CreditLog, int64, error) {
	return listCreditLogsByTypes(userID, logTypes, q)
}

func listCreditLogsByTypes(userID string, logTypes []model.CreditLogType, q model.Query) ([]model.CreditLog, int64, error) {
	db, err := DB()
	if err != nil {
		return nil, 0, err
	}
	q.Normalize()
	tx := db.Model(&model.CreditLog{}).Where("type IN ?", logTypes)
	if userID != "" {
		tx = tx.Where("user_id = ?", userID)
	}
	if keyword := strings.TrimSpace(q.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		tx = tx.Where("user_id LIKE ? OR remark LIKE ? OR related_id LIKE ? OR extra LIKE ?", like, like, like, like)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var logs []model.CreditLog
	err = tx.Order("created_at desc").Offset(q.Offset()).Limit(q.PageSize).Find(&logs).Error
	return logs, total, err
}

func DeleteCreditLog(id string) error {
	db, err := DB()
	if err != nil {
		return err
	}
	return db.Delete(&model.CreditLog{}, "id = ?", id).Error
}

func SaveEmailVerificationCode(code model.EmailVerificationCode) (model.EmailVerificationCode, error) {
	db, err := DB()
	if err != nil {
		return code, err
	}
	return code, db.Save(&code).Error
}

func LatestEmailVerificationCode(email string, purpose string) (model.EmailVerificationCode, bool, error) {
	db, err := DB()
	if err != nil {
		return model.EmailVerificationCode{}, false, err
	}
	code := model.EmailVerificationCode{}
	err = db.Where("email = ? AND purpose = ? AND consumed_at = ?", email, purpose, "").Order("created_at desc").First(&code).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.EmailVerificationCode{}, false, nil
	}
	return code, err == nil, err
}

func DeleteEmailVerificationCode(id string) error {
	db, err := DB()
	if err != nil {
		return err
	}
	return db.Delete(&model.EmailVerificationCode{}, "id = ?", id).Error
}

// DeleteUser 删除指定用户。
func DeleteUser(id string) error {
	db, err := DB()
	if err != nil {
		return err
	}
	return db.Delete(&model.User{}, "id = ?", id).Error
}

// GetUserByLinuxDoID 根据 Linux.do ID 查询用户。
func GetUserByLinuxDoID(id string) (model.User, bool, error) {
	db, err := DB()
	if err != nil {
		return model.User{}, false, err
	}
	return findUser(db, "linux_do_id = ?", id)
}

// findUser 查询单个用户，并将未命中转换为 ok=false。
func findUser(db *gorm.DB, query string, args ...any) (model.User, bool, error) {
	user := model.User{}
	err := db.Where(query, args...).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.User{}, false, nil
	}
	return user, err == nil, err
}

func findRechargeOrder(db *gorm.DB, query string, args ...any) (model.RechargeOrder, bool, error) {
	order := model.RechargeOrder{}
	err := db.Where(query, args...).First(&order).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.RechargeOrder{}, false, nil
	}
	return order, err == nil, err
}
