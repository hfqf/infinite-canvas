package handler

import (
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/basketikun/infinite-canvas/config"
)

// GetOSSImage 把 oss: 存储键（如 "oss:canvas/images/2026/06/26/abc.png"）解析成
// 公网 URL 后代理返回图片本体。主要给前端 /image-proxy 在 source 字段意外是 oss:
// 存储键时（老画布数据 / 字段误用）兜底用。
//
// 查询参数：
//   - key=oss:canvas/images/...     （推荐，存储键）
//   - objectKey=canvas/images/...   （备用，直接传对象 key）
func GetOSSImage(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimSpace(r.URL.Query().Get("key"))
	objectKey := strings.TrimSpace(r.URL.Query().Get("objectKey"))

	switch {
	case strings.HasPrefix(key, "oss:"):
		objectKey = strings.TrimPrefix(key, "oss:")
	case key != "" && !strings.Contains(key, "://"):
		// 兼容只传了相对路径的情况
		objectKey = key
	}

	objectKey = strings.TrimLeft(objectKey, "/")
	if objectKey == "" {
		Fail(w, "oss key is required")
		return
	}

	publicBase := strings.TrimRight(strings.TrimSpace(config.Cfg.AliyunOSSPublicBaseURL), "/")
	if publicBase == "" {
		Fail(w, "OSS public base url not configured")
		return
	}
	targetURL := publicBase + "/" + objectKey

	request, err := http.NewRequestWithContext(r.Context(), http.MethodGet, targetURL, nil)
	if err != nil {
		FailError(w, err)
		return
	}
	client := &http.Client{Timeout: 60 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		FailError(w, err)
		return
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		http.Error(w, "oss object not found", response.StatusCode)
		return
	}

	contentType := strings.TrimSpace(strings.Split(response.Header.Get("Content-Type"), ";")[0])
	if !strings.HasPrefix(strings.ToLower(contentType), "image/") {
		contentType = "application/octet-stream"
	}

	headers := w.Header()
	headers.Set("content-type", contentType)
	headers.Set("cache-control", "private, max-age=3600")
	w.WriteHeader(response.StatusCode)
	_, _ = io.Copy(w, response.Body)
}
