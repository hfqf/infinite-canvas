package handler

import (
	"bytes"
	"strings"
	"testing"
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

func TestRewriteAIRequestModelJSON(t *testing.T) {
	body := []byte(`{"model":"gpt-image-2-2k","prompt":"画一只猫"}`)
	next, contentType, err := rewriteAIRequestModel(body, "application/json", "gpt-image-2")
	if err != nil {
		t.Fatal(err)
	}
	if contentType != "application/json" {
		t.Fatalf("contentType = %s, want application/json", contentType)
	}
	if !bytes.Contains(next, []byte(`"model":"gpt-image-2"`)) {
		t.Fatalf("body not rewritten: %s", string(next))
	}
	if bytes.Contains(next, []byte(`gpt-image-2-2k`)) {
		t.Fatalf("public model leaked upstream: %s", string(next))
	}
}
