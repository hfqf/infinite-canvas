package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/basketikun/infinite-canvas/model"
	"github.com/basketikun/infinite-canvas/service"
)

func AIImagesGenerations(w http.ResponseWriter, r *http.Request) {
	proxyAIRequest(w, r, "/images/generations")
}

func AIImagesEdits(w http.ResponseWriter, r *http.Request) {
	proxyAIRequest(w, r, "/images/edits")
}

func AIImageTask(w http.ResponseWriter, r *http.Request, id string) {
	proxyAIGetRequest(w, r, "/image-tasks/"+id)
}

func AIImagesGenerationTask(w http.ResponseWriter, r *http.Request, id string) {
	proxyAIGetRequest(w, r, "/images/generations/"+id)
}

func AIChatCompletions(w http.ResponseWriter, r *http.Request) {
	proxyAIRequest(w, r, "/chat/completions")
}

func AIAudioSpeech(w http.ResponseWriter, r *http.Request) {
	proxyAIRequest(w, r, "/audio/speech")
}

func AIVideos(w http.ResponseWriter, r *http.Request) {
	proxyAIRequest(w, r, "/videos")
}

func AIVideo(w http.ResponseWriter, r *http.Request, id string) {
	proxyAIGetRequest(w, r, "/videos/"+id)
}

func AIVideoContent(w http.ResponseWriter, r *http.Request, id string) {
	proxyAIGetRequest(w, r, "/videos/"+id+"/content")
}

func proxyAIGetRequest(w http.ResponseWriter, r *http.Request, path string) {
	modelName := r.URL.Query().Get("model")
	if strings.TrimSpace(modelName) == "" {
		modelName = "grok-imagine-video"
	}
	channel, err := service.SelectModelChannel(modelName)
	if err != nil {
		log.Printf("AI proxy select channel failed: model=%s err=%v", modelName, err)
		Fail(w, "AI 接口请求失败")
		return
	}
	path = resolveAIProxyPath(channel.BaseURL, modelName, path)
	request, err := http.NewRequest(http.MethodGet, service.BuildModelChannelURL(channel, path), nil)
	if err != nil {
		Fail(w, "AI 接口请求失败")
		return
	}
	request.Header.Set("Authorization", "Bearer "+channel.APIKey)
	copyAIResponse(w, request, nil)
}

func proxyAIRequest(w http.ResponseWriter, r *http.Request, path string) {
	body, contentType, modelName, err := readAIRequest(r)
	if err != nil {
		log.Printf("AI proxy request read failed: %v", err)
		Fail(w, "AI 接口请求失败")
		return
	}
	user, ok := service.UserFromContext(r.Context())
	if !ok {
		Fail(w, "未登录或权限不足")
		return
	}
	credits, err := requestCredits(modelName, path, body, contentType)
	if err != nil {
		log.Printf("AI proxy read model cost failed: model=%s err=%v", modelName, err)
		Fail(w, "AI 接口请求失败")
		return
	}
	var imageChannels []model.ModelChannel
	var request *http.Request
	if isImageRequestPath(path) {
		imageChannels, err = service.SelectModelChannels(modelName)
		if err != nil {
			log.Printf("AI proxy select channels failed: model=%s err=%v", modelName, err)
			Fail(w, "AI 接口请求失败")
			return
		}
	} else {
		channel, err := service.SelectModelChannel(modelName)
		if err != nil {
			log.Printf("AI proxy select channel failed: model=%s err=%v", modelName, err)
			Fail(w, "AI 接口请求失败")
			return
		}
		request, err = buildAIProxyRequest(channel, modelName, path, body, contentType)
		if err != nil {
			log.Printf("AI proxy build request failed: model=%s channel=%s err=%v", modelName, channel.Name, err)
			Fail(w, "AI 接口请求失败")
			return
		}
	}
	if err := service.ConsumeUserCredits(user.ID, modelName, credits, path); err != nil {
		FailError(w, err)
		return
	}
	refund := func() {
		if err := service.RefundUserCredits(user.ID, modelName, credits, path); err != nil {
			log.Printf("AI proxy refund credits failed: user=%s model=%s credits=%d err=%v", user.ID, modelName, credits, err)
		}
	}
	if isImageRequestPath(path) {
		copyAIImageResponseWithFallback(w, imageChannels, modelName, path, body, contentType, refund)
		return
	}
	copyAIResponse(w, request, refund)
}

type aiProxyAttemptResult struct {
	statusCode int
	header     http.Header
	body       []byte
	message    string
	retryable  bool
}

const (
	aiImageFailureCooldownSeconds   = 120
	aiImage4KFailureCooldownSeconds = 180
	aiImageReferenceTimeoutSeconds  = 60
)

func copyAIImageResponseWithFallback(w http.ResponseWriter, channels []model.ModelChannel, modelName string, path string, body []byte, contentType string, onFailure func()) {
	lastResult := aiProxyAttemptResult{message: "AI 接口请求失败"}
	timeoutSeconds := aiImageRequestTimeoutSeconds(body, contentType)
	for index, channel := range channels {
		request, err := buildAIProxyRequest(channel, modelName, path, body, contentType)
		if err != nil {
			log.Printf("AI proxy build request failed: model=%s channel=%s err=%v", modelName, channel.Name, err)
			service.RecordModelChannelFailureWithCooldown(channel, timeoutSeconds)
			lastResult = aiProxyAttemptResult{message: "AI 接口请求失败", retryable: true}
			continue
		}
		result := fetchAIImageResponse(request, time.Duration(timeoutSeconds)*time.Second)
		if result.statusCode < http.StatusBadRequest && result.message == "" {
			service.RecordModelChannelSuccess(channel)
			writeAIProxySuccess(w, result)
			return
		}
		lastResult = result
		if result.retryable {
			service.RecordModelChannelFailureWithCooldown(channel, timeoutSeconds)
			log.Printf("AI proxy retryable failure: model=%s channel=%s status=%d attempt=%d/%d", modelName, channel.Name, result.statusCode, index+1, len(channels))
			continue
		}
		onFailure()
		Fail(w, result.message)
		return
	}
	onFailure()
	Fail(w, lastResult.message)
}

func aiImageRequestTimeoutSeconds(body []byte, contentType string) int {
	seconds := readAIReferenceImageCount(body, contentType) * aiImageReferenceTimeoutSeconds
	if is4KImageRequest(body, contentType) {
		return seconds + aiImage4KFailureCooldownSeconds
	}
	return seconds + aiImageFailureCooldownSeconds
}

func buildAIProxyRequest(channel model.ModelChannel, modelName string, path string, body []byte, contentType string) (*http.Request, error) {
	resolvedPath := resolveAIProxyPath(channel.BaseURL, modelName, path)
	request, err := http.NewRequest(http.MethodPost, service.BuildModelChannelURL(channel, resolvedPath), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+channel.APIKey)
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	return request, nil
}

func fetchAIImageResponse(request *http.Request, timeout time.Duration) aiProxyAttemptResult {
	client := &http.Client{Timeout: timeout}
	response, err := client.Do(request)
	if err != nil {
		log.Printf("AI proxy request failed: url=%s err=%v", request.URL.String(), err)
		return aiProxyAttemptResult{message: "AI 接口请求失败", retryable: true}
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		log.Printf("AI upstream error: url=%s status=%d", request.URL.String(), response.StatusCode)
		return aiProxyAttemptResult{
			statusCode: response.StatusCode,
			message:    aiUpstreamStatusMessage(response.StatusCode, body),
			retryable:  isRetryableAIProxyStatus(response.StatusCode),
		}
	}
	body, readErr := io.ReadAll(io.LimitReader(response.Body, 16<<20))
	if readErr != nil {
		return aiProxyAttemptResult{message: "AI 接口请求失败", retryable: true}
	}
	if shouldNormalizeImagesResponse(request.URL.Path) {
		finalBody, polled := pollAicodemeImageTaskIfNeeded(request, body, response.Header)
		if polled {
			body = finalBody
		}
		if normalized, ok := normalizeAicodemeImagesResponse(body); ok {
			body = normalized
		}
	}
	return aiProxyAttemptResult{statusCode: response.StatusCode, header: response.Header, body: body}
}

func writeAIProxySuccess(w http.ResponseWriter, result aiProxyAttemptResult) {
	copyHeaders(w.Header(), result.header)
	if contentType := result.header.Get("Content-Type"); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	} else {
		w.Header().Set("Content-Type", "application/json")
	}
	w.WriteHeader(result.statusCode)
	_, _ = w.Write(result.body)
}

func isRetryableAIProxyStatus(status int) bool {
	return status == http.StatusTooManyRequests || status >= http.StatusInternalServerError
}

func copyAIResponse(w http.ResponseWriter, request *http.Request, onFailure func()) {
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		log.Printf("AI proxy request failed: url=%s err=%v", request.URL.String(), err)
		if onFailure != nil {
			onFailure()
		}
		Fail(w, "AI 接口请求失败")
		return
	}
	defer response.Body.Close()

	if response.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		log.Printf("AI upstream error: url=%s status=%d", request.URL.String(), response.StatusCode)
		if onFailure != nil {
			onFailure()
		}
		Fail(w, aiUpstreamStatusMessage(response.StatusCode, body))
		return
	}

	// /images/generations 上游可能是非 OpenAI 标准格式（如 aicodeme 返的是
	// {id, task_id, status, result: {data: [{url, ...}]}}，而非 OpenAI 的
	// {created, data: [{b64_json|url}]}）。在写响应前探测一次，是就转。
	if shouldNormalizeImagesResponse(request.URL.Path) {
		body, readErr := io.ReadAll(io.LimitReader(response.Body, 16<<20))
		if readErr == nil {
			// aicodeme 返的是异步 task（如 {status:"queued", task_id:"..."}）。
			// 后端代理轮询 task 直到 succeeded/failed/超时，再转 OpenAI 标准。
			// 轮询后的 body 走 normalizeAicodemeImagesResponse → OpenAI 标准。
			finalBody, polled := pollAicodemeImageTaskIfNeeded(request, body, response.Header)
			if polled {
				body = finalBody
			}
			if normalized, ok := normalizeAicodemeImagesResponse(body); ok {
				copyHeaders(w.Header(), response.Header)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(response.StatusCode)
				_, _ = w.Write(normalized)
				return
			}
			// 不是 aicodeme 格式（或无法识别），原样写回
			copyHeaders(w.Header(), response.Header)
			w.WriteHeader(response.StatusCode)
			_, _ = w.Write(body)
			return
		}
	}

	for key, values := range response.Header {
		if strings.EqualFold(key, "Content-Length") {
			continue
		}
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(response.StatusCode)
	_, _ = io.Copy(w, response.Body)
}

func copyHeaders(dst http.Header, src http.Header) {
	for key, values := range src {
		if strings.EqualFold(key, "Content-Length") || strings.EqualFold(key, "Content-Type") {
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

// aicodeme 等非标准上游 images/generations 返的格式是
// {id, task_id, status:"succeeded", result:{data:[{url,...}]}}，
// OpenAI 标准是 {created, data:[{b64_json|url}]}。这里转一次，让前端按 OpenAI 标准解析。
// 返回 (normalized, true) 表示转换成功；返回 (nil, false) 表示不是 aicodeme 格式，原样写回。
func normalizeAicodemeImagesResponse(body []byte) ([]byte, bool) {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(body, &probe); err != nil {
		return nil, false
	}
	// aicodeme 特征：有 task_id + result.data
	if _, ok := probe["task_id"]; !ok {
		return nil, false
	}
	if _, ok := probe["result"]; !ok {
		return nil, false
	}
	type aicodemeResult struct {
		Data []map[string]any `json:"data"`
	}
	type aicodemeResp struct {
		ID       string         `json:"id"`
		TaskID   string         `json:"task_id"`
		Status   string         `json:"status"`
		Progress int            `json:"progress"`
		Result   aicodemeResult `json:"result"`
	}
	var resp aicodemeResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, false
	}
	if len(resp.Result.Data) == 0 {
		return nil, false
	}
	type openAIDataItem struct {
		URL     string `json:"url,omitempty"`
		B64JSON string `json:"b64_json,omitempty"`
	}
	type openAIResp struct {
		Created int64            `json:"created"`
		Data    []openAIDataItem `json:"data"`
	}
	out := openAIResp{
		Created: time.Now().Unix(),
		Data:    make([]openAIDataItem, 0, len(resp.Result.Data)),
	}
	for _, item := range resp.Result.Data {
		entry := openAIDataItem{}
		if u, ok := item["url"].(string); ok && u != "" {
			entry.URL = u
		} else if b64, ok := item["b64_json"].(string); ok && b64 != "" {
			entry.B64JSON = b64
		} else {
			continue
		}
		out.Data = append(out.Data, entry)
	}
	if len(out.Data) == 0 {
		return nil, false
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		return nil, false
	}
	return encoded, true
}

func shouldNormalizeImagesResponse(path string) bool {
	return strings.HasSuffix(path, "/images/generations")
}

// aicodeme 的 images/generations 返 task 格式：{status, task_id, retry_after, result, ...}。
// 如果 status 是 queued/running 还没完，后端代理用 task_id 轮询
// GET /v1/images/generations/{task_id}，隔 retry_after 秒一次，
// 直到 status="succeeded"/"failed" 或超过 maxPollTimes。
// 成功后将完整的 task body 返出去（由 normalizeAicodemeImagesResponse 转 OpenAI 标准）。
//
// 注意：这是串行轮询，会让 canvas 前端感觉这个 HTTP 请求"很久"才回，
// 但前端既然能等（status code 还没返），就不必改前端。
// aicodeme retry_after 2s 默认，最坏情况 30 轮 × 2s = 60s。
const (
	aicodemeTaskPollMaxTimes = 30
	aicodemeTaskPollMinDelay = 1 * time.Second
)

func pollAicodemeImageTaskIfNeeded(req *http.Request, body []byte, upstreamHeader http.Header) ([]byte, bool) {
	var probe struct {
		TaskID     string `json:"task_id"`
		Status     string `json:"status"`
		RetryAfter int    `json:"retry_after"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		return body, false
	}
	if probe.TaskID == "" {
		return body, false
	}
	// OpenAI 标准任务 API 也可能返 task_id（status: in_progress），这里也轮询。
	// 但前端目前走的是同步逻辑，所以 OpenAI 标准任务 API 不会到这里。
	if probe.Status == "succeeded" {
		return body, true // 已经完成，让 normalize 转格式即可
	}
	if probe.Status == "failed" || probe.Status == "error" || probe.Status == "canceled" {
		return body, true // 终态，转给 normalize 处理（normalize 返 OpenAI 标准或 false）
	}
	if probe.Status != "" && probe.Status != "queued" && probe.Status != "running" && probe.Status != "in_progress" && probe.Status != "processing" && probe.Status != "pending" {
		// 不认识的状态，当成"已完成或不能轮询"，让 normalize 试着转
		return body, true
	}

	// 拼轮询 URL：把 path 的 "/images/generations" 替换成 "/images/generations/{task_id}"
	pollPath := req.URL.Path
	if idx := strings.LastIndex(pollPath, "/images/generations"); idx >= 0 {
		pollPath = pollPath[:idx] + "/images/generations/" + probe.TaskID
	} else {
		pollPath = "/v1/images/generations/" + probe.TaskID
	}
	pollURL := *req.URL
	pollURL.Path = pollPath

	authHeader := req.Header.Get("Authorization")

	delay := time.Duration(probe.RetryAfter) * time.Second
	if delay <= 0 {
		delay = aicodemeTaskPollMinDelay
	}
	client := &http.Client{Timeout: 30 * time.Second}
	for i := 0; i < aicodemeTaskPollMaxTimes; i++ {
		time.Sleep(delay)
		pollReq, _ := http.NewRequest(http.MethodGet, pollURL.String(), nil)
		if authHeader != "" {
			pollReq.Header.Set("Authorization", authHeader)
		}
		resp, err := client.Do(pollReq)
		if err != nil {
			log.Printf("AI task poll failed: url=%s err=%v", pollURL.String(), err)
			continue
		}
		raw, readErr := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		resp.Body.Close()
		if readErr != nil {
			continue
		}
		var next struct {
			Status     string          `json:"status"`
			RetryAfter int             `json:"retry_after"`
			Result     json.RawMessage `json:"result"`
		}
		if err := json.Unmarshal(raw, &next); err != nil {
			continue
		}
		log.Printf("AI task poll: task=%s status=%s progress=%d%% iter=%d", probe.TaskID, next.Status, i, i)
		if next.Status == "succeeded" {
			return raw, true
		}
		if next.Status == "failed" || next.Status == "error" || next.Status == "canceled" {
			return raw, true
		}
		// 更新下一轮 delay
		if d := time.Duration(next.RetryAfter) * time.Second; d > 0 {
			delay = d
		}
		if d := aicodemeTaskPollMinDelay; delay < d {
			delay = d
		}
	}
	log.Printf("AI task poll timeout: task=%s max=%d", probe.TaskID, aicodemeTaskPollMaxTimes)
	return body, true // 轮询超时也返原 body（queued 状态），让 normalize 决定怎么处理
}

// aicodeme 的 images/generations 默认返 task 格式。一种猜测是加上 async:true
// 后会返 OpenAI 标准 / 同步结果 —— 为了避免依赖此行为不确定，这里两个都做：
// 1. 转发时在 JSON body 上固定加 async:true（不动 multipart）
// 2. 响应端保留 normalizeAicodemeImagesResponse 作为兑底（处理仍返 task 格式的情况）
func ensureAsyncTrueOnImages(path string, body []byte, contentType string) ([]byte, string) {
	if !strings.HasSuffix(path, "/images/generations") {
		return body, contentType
	}
	if !strings.HasPrefix(contentType, "application/json") {
		return body, contentType
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return body, contentType
	}
	// 用户可能已带 async 参数，不覆盖
	if _, ok := payload["async"]; !ok {
		asyncTrue, _ := json.Marshal(true)
		payload["async"] = asyncTrue
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return body, contentType
	}
	return encoded, contentType
}

func readAIRequest(r *http.Request) ([]byte, string, string, error) {
	contentType := r.Header.Get("Content-Type")
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, "", "", err
	}
	modelName := ""
	if strings.HasPrefix(contentType, "multipart/form-data") {
		modelName = readMultipartModel(body, contentType)
	} else {
		var payload struct {
			Model string `json:"model"`
		}
		_ = json.Unmarshal(body, &payload)
		modelName = payload.Model
	}
	if strings.TrimSpace(modelName) == "" {
		return nil, "", "", errMissingModel
	}
	return body, contentType, modelName, nil
}

func readMultipartModel(body []byte, contentType string) string {
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return ""
	}
	reader := multipart.NewReader(bytes.NewReader(body), params["boundary"])
	form, err := reader.ReadForm(32 << 20)
	if err != nil {
		return ""
	}
	defer form.RemoveAll()
	if values := form.Value["model"]; len(values) > 0 {
		return values[0]
	}
	return ""
}

func requestCredits(modelName string, path string, body []byte, contentType string) (int, error) {
	credits, err := service.ModelCost(modelName)
	if err != nil {
		return 0, err
	}
	if isImageRequestPath(path) && is4KImageRequest(body, contentType) {
		credits = 6
	}
	return credits * readAIRequestCount(body, contentType), nil
}

func isImageRequestPath(path string) bool {
	return path == "/images/generations" || path == "/images/edits"
}

func is4KImageRequest(body []byte, contentType string) bool {
	size, quality := readAIImageRequestSizeQuality(body, contentType)
	return is4KImageValue(size) || strings.EqualFold(strings.TrimSpace(quality), "4k")
}

func readAIImageRequestSizeQuality(body []byte, contentType string) (string, string) {
	if strings.HasPrefix(contentType, "multipart/form-data") {
		_, params, err := mime.ParseMediaType(contentType)
		if err != nil {
			return "", ""
		}
		form, err := multipart.NewReader(bytes.NewReader(body), params["boundary"]).ReadForm(32 << 20)
		if err != nil {
			return "", ""
		}
		defer form.RemoveAll()
		return firstFormValue(form.Value["size"]), firstFormValue(form.Value["quality"])
	}
	var payload struct {
		Size    string `json:"size"`
		Quality string `json:"quality"`
	}
	_ = json.Unmarshal(body, &payload)
	return payload.Size, payload.Quality
}

func readAIReferenceImageCount(body []byte, contentType string) int {
	if !strings.HasPrefix(contentType, "multipart/form-data") {
		return 0
	}
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return 0
	}
	form, err := multipart.NewReader(bytes.NewReader(body), params["boundary"]).ReadForm(32 << 20)
	if err != nil {
		return 0
	}
	defer form.RemoveAll()
	return len(form.File["image"])
}

func firstFormValue(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func is4KImageValue(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	if strings.Contains(value, "4k") {
		return true
	}
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == 'x' || r == '×' || r == '*' || r == ' '
	})
	if len(parts) != 2 {
		return false
	}
	var width, height int
	if _, err := fmt.Sscan(parts[0], &width); err != nil {
		return false
	}
	if _, err := fmt.Sscan(parts[1], &height); err != nil {
		return false
	}
	return width >= 3840 || height >= 3840
}

func readAIRequestCount(body []byte, contentType string) int {
	count := 1
	if strings.HasPrefix(contentType, "multipart/form-data") {
		_, params, err := mime.ParseMediaType(contentType)
		if err != nil {
			return count
		}
		form, err := multipart.NewReader(bytes.NewReader(body), params["boundary"]).ReadForm(32 << 20)
		if err != nil {
			return count
		}
		defer form.RemoveAll()
		if values := form.Value["n"]; len(values) > 0 {
			_, _ = fmt.Sscan(values[0], &count)
		}
	} else {
		var payload struct {
			N int `json:"n"`
		}
		_ = json.Unmarshal(body, &payload)
		count = payload.N
	}
	if count < 1 {
		return 1
	}
	return count
}

var errMissingModel = &aiError{"缺少模型名称"}

func resolveAIProxyPath(baseURL string, modelName string, path string) string {
	if !isArkSeedanceVideo(baseURL, modelName) {
		return path
	}
	if path == "/videos" {
		return "/contents/generations/tasks"
	}
	if strings.HasPrefix(path, "/videos/") && !strings.HasSuffix(path, "/content") {
		return "/contents/generations/tasks/" + strings.TrimPrefix(path, "/videos/")
	}
	return path
}

func isArkSeedanceVideo(baseURL string, modelName string) bool {
	base := strings.ToLower(baseURL)
	model := strings.ToLower(modelName)
	return strings.Contains(model, "seedance") || strings.Contains(model, "doubao-seedance") || strings.Contains(base, "/api/plan/v3")
}

func aiStatusMessage(statusCode int) string {
	switch statusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return "AI 接口鉴权失败，请检查 API Key、套餐权限或模型权限"
	case http.StatusTooManyRequests:
		return "AI 接口限流或额度不足，请稍后重试或检查额度"
	default:
		return "AI 接口请求失败"
	}
}

func aiUpstreamStatusMessage(statusCode int, body []byte) string {
	base := aiStatusMessage(statusCode)
	detail := aiUpstreamErrorDetail(body)
	if detail == "" {
		return base
	}
	return base + "：" + detail
}

func aiUpstreamErrorDetail(body []byte) string {
	text := strings.TrimSpace(string(body))
	if text == "" {
		return ""
	}
	var payload struct {
		Msg     string `json:"msg"`
		Message string `json:"message"`
		Error   struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err == nil {
		if payload.Error.Message != "" {
			if detail := friendlyUpstreamError(payload.Error.Code, payload.Error.Message); detail != "" {
				return safeUpstreamText(detail)
			}
			if payload.Error.Code != "" {
				return safeUpstreamText(payload.Error.Code + " " + payload.Error.Message)
			}
			return safeUpstreamText(payload.Error.Message)
		}
		if payload.Msg != "" {
			return safeUpstreamText(payload.Msg)
		}
		if payload.Message != "" {
			return safeUpstreamText(payload.Message)
		}
	}
	return safeUpstreamText(text)
}

func friendlyUpstreamError(code string, message string) string {
	lowerCode := strings.ToLower(strings.TrimSpace(code))
	if strings.Contains(lowerCode, "inputvideosensitivecontentdetected") || strings.Contains(lowerCode, "privacyinformation") {
		return strings.TrimSpace(code + " 参考视频疑似包含真人或隐私信息，火山方舟拒绝使用普通 URL 作为真人视频参考；请改用不含真人的视频、官方允许的模型产物，或已授权的 asset:// 素材。原始错误：" + message)
	}
	return ""
}

func safeUpstreamText(text string) string {
	text = strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	runes := []rune(text)
	if len(runes) > 300 {
		return string(runes[:300]) + "..."
	}
	return text
}

type aiError struct {
	message string
}

func (err *aiError) Error() string {
	return err.message
}
