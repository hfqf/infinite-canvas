package handler

import (
	"bytes"
	"context"
	"mime"
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

func TestAIImageRequestTimeoutSecondsAddsJSONReferenceImageDuration(t *testing.T) {
	body := []byte(`{"size":"1024x1024","image":["https://cdn.example.com/a.png","data:image/png;base64,abc"]}`)
	if got := aiImageRequestTimeoutSeconds(body, "application/json"); got != 240 {
		t.Fatalf("json reference timeout = %d, want 240", got)
	}
}

func TestReadAIReferenceImageCountFromJSONAliases(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{name: "image string", body: `{"image":"https://cdn.example.com/a.png"}`, want: 1},
		{name: "image array", body: `{"image":["a","b"]}`, want: 2},
		{name: "images array", body: `{"images":["a","b","c"]}`, want: 3},
		{name: "ref assets object array", body: `{"ref_assets":[{"url":"a"},{"image_url":"b"}]}`, want: 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := readAIReferenceImageCount([]byte(tc.body), "application/json"); got != tc.want {
				t.Fatalf("count = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestImageRequestCreditsAddsReferenceImageSurchargeAfterFirstReference(t *testing.T) {
	if got := imageRequestCredits(5, true, false, 2, 3); got != 14 {
		t.Fatalf("non-4k credits = %d, want (5 + 2*1) * 2 = 14", got)
	}
	if got := imageRequestCredits(9, true, false, 1, 0); got != 9 {
		t.Fatalf("non-4k base credits = %d, want configured 9", got)
	}
	if got := imageRequestCredits(5, true, true, 1, 3); got != 10 {
		t.Fatalf("4k credits = %d, want 5 + 3 + 2*1 = 10", got)
	}
	if got := imageRequestCredits(5, false, true, 1, 0); got != 5 {
		t.Fatalf("unsupported 4k credits = %d, want non-4k configured 5", got)
	}
	if got := imageRequestCredits(5, true, false, 4, 1); got != 20 {
		t.Fatalf("single reference credits = %d, want 5 * 4 = 20", got)
	}
}

func TestEnsureAsyncTrueOnImagesForcesURLResponseFormat(t *testing.T) {
	body, contentType := ensureAsyncTrueOnImages("/images/generations", []byte(`{"model":"gpt-image-2","prompt":"hi","response_format":"b64_json"}`), "application/json")
	if contentType != "application/json" {
		t.Fatalf("contentType = %q, want application/json", contentType)
	}
	if !strings.Contains(string(body), `"async":true`) {
		t.Fatalf("body missing async=true: %s", body)
	}
	if !strings.Contains(string(body), `"response_format":"url"`) {
		t.Fatalf("body missing response_format=url: %s", body)
	}
	if !strings.Contains(string(body), `"output_format":"png"`) {
		t.Fatalf("body missing output_format=png: %s", body)
	}
}

func TestEnsureAsyncTrueOnImagesAddsPNGOutputFormatToEditsMultipart(t *testing.T) {
	body, contentType := multipartImageRequestBody(t, 1, "1024x1024")
	body, contentType = ensureAsyncTrueOnImages("/images/edits", body, contentType)
	if got := readMultipartTestValue(t, body, contentType, "output_format"); got != "png" {
		t.Fatalf("output_format = %q, want png", got)
	}
	if got := readMultipartTestValue(t, body, contentType, "model"); got != "gpt-image-2" {
		t.Fatalf("model = %q, want preserved gpt-image-2", got)
	}
}

func TestAIImageResponseLimitsAre30MB(t *testing.T) {
	if aiImageResponseMaxBytes != 30<<20 {
		t.Fatalf("aiImageResponseMaxBytes = %d, want 30MiB", aiImageResponseMaxBytes)
	}
	if aiImageTaskPollResponseMaxBytes != 30<<20 {
		t.Fatalf("aiImageTaskPollResponseMaxBytes = %d, want 30MiB", aiImageTaskPollResponseMaxBytes)
	}
}

func TestFetchAIImageResponseRejectsOversizeSuccessBody(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(strings.Repeat("x", aiImageResponseMaxBytes+1)))
	}))
	defer upstream.Close()
	request, err := http.NewRequest(http.MethodGet, upstream.URL, nil)
	if err != nil {
		t.Fatal(err)
	}

	result := fetchAIImageResponse(request, 0)

	if result.statusCode != http.StatusOK || result.message == "" || result.retryable {
		t.Fatalf("result=%#v, want explicit non-retryable oversize failure", result)
	}
}

func TestImageBillingOutcomeRequiresImageURLBeforeCharge(t *testing.T) {
	if got := imageBillingOutcome("succeeded", ""); got != imageBillingRelease {
		t.Fatalf("succeeded without image outcome = %s, want release", got)
	}
	if got := imageBillingOutcome("succeeded", "https://cdn.example.com/image.png"); got != imageBillingCharge {
		t.Fatalf("succeeded with image outcome = %s, want charge", got)
	}
	if got := imageBillingOutcome("running", ""); got != imageBillingPending {
		t.Fatalf("running outcome = %s, want pending", got)
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
		context.Background(),
		recorder,
		[]model.ModelChannel{
			{Name: "primary-test", BaseURL: primary.URL, APIKey: "primary-key"},
			{Name: "backup-test", BaseURL: backup.URL, APIKey: "backup-key"},
		},
		model.AIImageTask{ID: "task_test", TaskID: "task_test", UserID: "user_test"},
		"gpt-image-2",
		"/images/generations",
		[]byte(`{"model":"gpt-image-2","prompt":"hi"}`),
		"application/json",
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

func readMultipartTestValue(t *testing.T, body []byte, contentType string, name string) string {
	t.Helper()
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatal(err)
	}
	form, err := multipart.NewReader(bytes.NewReader(body), params["boundary"]).ReadForm(32 << 20)
	if err != nil {
		t.Fatal(err)
	}
	defer form.RemoveAll()
	if values := form.Value[name]; len(values) > 0 {
		return values[0]
	}
	return ""
}
