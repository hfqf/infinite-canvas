package handler

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/basketikun/infinite-canvas/model"
)

func TestAIUpstreamErrorDetail(t *testing.T) {
	got := aiUpstreamErrorDetail([]byte(`{"error":{"code":"InvalidParameter","message":"reference video fps is invalid"}}`))
	if got != "InvalidParameter reference video fps is invalid" {
		t.Fatalf("detail = %q", got)
	}
}

func TestAIUpstreamErrorDetailExplainsSensitiveVideo(t *testing.T) {
	got := aiUpstreamErrorDetail([]byte(`{"error":{"code":"InputVideoSensitiveContentDetected.PrivacyInformation","message":"The request failed because the input video may contain real person."}}`))
	if !strings.Contains(got, "参考视频疑似包含真人") || !strings.Contains(got, "asset://") {
		t.Fatalf("detail = %q", got)
	}
}

func TestSafeUpstreamTextTruncates(t *testing.T) {
	got := safeUpstreamText(strings.Repeat("错", 320))
	if len([]rune(got)) != 303 {
		t.Fatalf("truncated rune length = %d", len([]rune(got)))
	}
}

func TestRetryableAIProxyStatus(t *testing.T) {
	for _, status := range []int{429, 500, 502, 503, 504} {
		if !isRetryableAIProxyStatus(status) {
			t.Fatalf("status %d should retry", status)
		}
	}
	for _, status := range []int{400, 401, 403, 404, 422} {
		if isRetryableAIProxyStatus(status) {
			t.Fatalf("status %d should not retry", status)
		}
	}
}

func TestAIImageRequestTimeoutSecondsUsesBaseDuration(t *testing.T) {
	if got := aiImageRequestTimeoutSeconds([]byte(`{"size":"1024x1024"}`), "application/json"); got != 120 {
		t.Fatalf("non-4k cooldown = %d, want 120", got)
	}
	if got := aiImageRequestTimeoutSeconds([]byte(`{"size":"3840x2160"}`), "application/json"); got != 180 {
		t.Fatalf("4k cooldown = %d, want 180", got)
	}
}

func TestAIImageRequestTimeoutSecondsAddsReferenceImageDuration(t *testing.T) {
	body, contentType := multipartImageRequestBody(t, 3, "1024x1024")
	if got := aiImageRequestTimeoutSeconds(body, contentType); got != 300 {
		t.Fatalf("non-4k reference timeout = %d, want 300", got)
	}
	body, contentType = multipartImageRequestBody(t, 2, "3840x2160")
	if got := aiImageRequestTimeoutSeconds(body, contentType); got != 300 {
		t.Fatalf("4k reference timeout = %d, want 300", got)
	}
}

func TestImageRequestCreditsAddsReferenceImageSurchargeAfterFirstReference(t *testing.T) {
	if got := imageRequestCredits(2, false, 2, 3); got != 12 {
		t.Fatalf("non-4k credits = %d, want (2 + 2*2) * 2 = 12", got)
	}
	if got := imageRequestCredits(2, true, 1, 3); got != 10 {
		t.Fatalf("4k credits = %d, want 6 + 2*2 = 10", got)
	}
	if got := imageRequestCredits(2, false, 4, 1); got != 8 {
		t.Fatalf("single reference credits = %d, want 2 * 4 = 8", got)
	}
}

func TestCopyAIImageResponseWithFallbackRetriesBackupOnServerError(t *testing.T) {
	primaryHits := 0
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		primaryHits++
		http.Error(w, `{"error":{"message":"upstream down"}}`, http.StatusBadGateway)
	}))
	defer primary.Close()
	backupHits := 0
	backup := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backupHits++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"ok"}]}`))
	}))
	defer backup.Close()
	recorder := httptest.NewRecorder()

	copyAIImageResponseWithFallback(
		recorder,
		[]model.ModelChannel{
			{Name: "primary-test", BaseURL: primary.URL, APIKey: "primary-key"},
			{Name: "backup-test", BaseURL: backup.URL, APIKey: "backup-key"},
		},
		"gpt-image-2",
		"/images/generations",
		[]byte(`{"model":"gpt-image-2","prompt":"hi"}`),
		"application/json",
		func() { t.Fatal("should not refund when backup succeeds") },
	)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", recorder.Code, recorder.Body.String())
	}
	if primaryHits != 1 || backupHits != 1 {
		t.Fatalf("hits primary=%d backup=%d, want 1 and 1", primaryHits, backupHits)
	}
	if !strings.Contains(recorder.Body.String(), `"b64_json":"ok"`) {
		t.Fatalf("body = %s", recorder.Body.String())
	}
}

func multipartImageRequestBody(t *testing.T, imageCount int, size string) ([]byte, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("model", "gpt-image-2")
	_ = writer.WriteField("prompt", "hi")
	_ = writer.WriteField("size", size)
	for i := 0; i < imageCount; i++ {
		part, err := writer.CreateFormFile("image", "ref.png")
		if err != nil {
			t.Fatal(err)
		}
		_, _ = part.Write([]byte("png"))
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return body.Bytes(), writer.FormDataContentType()
}
