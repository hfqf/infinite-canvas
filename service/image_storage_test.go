package service

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

func TestOSSImageStorageSaveUploadsObjectAndReturnsURL(t *testing.T) {
	bucket := &fakeImageOSSBucket{}
	storage := &ossImageStorage{
		endpoint:      "oss-cn-hangzhou.aliyuncs.com",
		bucketName:    "haotushow3",
		publicBaseURL: "https://oss.haotushow.com",
		bucket:        bucket,
	}

	result, err := storage.Save(context.Background(), imageUploadRequest{
		FileName:    "../poster image.png",
		ContentType: "image/png",
		Reader:      strings.NewReader("png-bytes"),
	})
	if err != nil {
		t.Fatal(err)
	}

	if bucket.putCalls != 1 {
		t.Fatalf("PutObject calls = %d, want 1", bucket.putCalls)
	}
	if !strings.HasPrefix(bucket.objectKey, "canvas/images/") || !strings.HasSuffix(bucket.objectKey, "-poster-image.png") {
		t.Fatalf("object key = %q, want canvas/images dated path with sanitized filename", bucket.objectKey)
	}
	if bucket.body != "png-bytes" {
		t.Fatalf("uploaded body = %q, want png-bytes", bucket.body)
	}
	if bucket.contentType != "image/png" {
		t.Fatalf("content type = %q, want image/png", bucket.contentType)
	}
	if result.URL != "https://oss.haotushow.com/"+bucket.objectKey {
		t.Fatalf("URL = %q, want public base URL plus object key", result.URL)
	}
	if result.StorageKey != "oss:"+bucket.objectKey {
		t.Fatalf("StorageKey = %q, want oss object key", result.StorageKey)
	}
	if result.Bytes != int64(len("png-bytes")) {
		t.Fatalf("Bytes = %d, want %d", result.Bytes, len("png-bytes"))
	}
}

func TestSanitizeImageFileNameDefaultsWhenEmpty(t *testing.T) {
	for _, name := range []string{"", "...", "---", ".-.-"} {
		if got := sanitizeImageFileName(name); got != "image.png" {
			t.Fatalf("sanitizeImageFileName(%q) = %q, want image.png", name, got)
		}
	}
}

type fakeImageOSSBucket struct {
	putCalls    int
	objectKey   string
	body        string
	contentType string
}

func (b *fakeImageOSSBucket) PutObject(objectKey string, reader io.Reader, options ...oss.Option) error {
	b.putCalls++
	b.objectKey = objectKey
	body, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	b.body = string(body)
	value, err := oss.FindOption(options, oss.HTTPHeaderContentType, "")
	if err != nil {
		return err
	}
	b.contentType, _ = value.(string)
	return nil
}
