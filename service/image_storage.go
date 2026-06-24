package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/basketikun/infinite-canvas/config"
)

const ossImagePrefix = "canvas/images"

type UploadedImage struct {
	URL        string `json:"url"`
	StorageKey string `json:"storageKey"`
	Path       string `json:"path"`
	Bytes      int64  `json:"bytes"`
	MimeType   string `json:"mimeType"`
}

type imageUploadRequest struct {
	FileName    string
	ContentType string
	Reader      io.Reader
}

type ossImageStorage struct {
	endpoint      string
	bucketName    string
	publicBaseURL string
	bucket        imageOSSBucket
}

type imageOSSBucket interface {
	PutObject(objectKey string, reader io.Reader, options ...oss.Option) error
}

func SaveUploadedImage(ctx context.Context, fileName string, contentType string, reader io.Reader) (UploadedImage, error) {
	storage, err := newOSSImageStorageFromConfig()
	if err != nil {
		return UploadedImage{}, err
	}
	return storage.Save(ctx, imageUploadRequest{FileName: fileName, ContentType: contentType, Reader: reader})
}

func newOSSImageStorageFromConfig() (*ossImageStorage, error) {
	if strings.ToLower(strings.TrimSpace(config.Cfg.ImageStorageDriver)) != "oss" {
		return nil, safeMessageError{message: "图片存储未启用 OSS"}
	}
	endpoint := strings.TrimSpace(config.Cfg.AliyunOSSEndpoint)
	bucketName := strings.TrimSpace(config.Cfg.AliyunOSSBucket)
	accessKeyID := strings.TrimSpace(config.Cfg.AliyunOSSAccessKeyID)
	accessKeySecret := strings.TrimSpace(config.Cfg.AliyunOSSAccessKeySecret)
	publicBaseURL := strings.TrimRight(strings.TrimSpace(config.Cfg.AliyunOSSPublicBaseURL), "/")
	if endpoint == "" || bucketName == "" || accessKeyID == "" || accessKeySecret == "" || publicBaseURL == "" {
		return nil, safeMessageError{message: "OSS 图片存储配置不完整"}
	}
	client, err := oss.New(endpoint, accessKeyID, accessKeySecret)
	if err != nil {
		return nil, fmt.Errorf("initialize OSS client: %w", err)
	}
	bucket, err := client.Bucket(bucketName)
	if err != nil {
		return nil, fmt.Errorf("initialize OSS bucket: %w", err)
	}
	return &ossImageStorage{endpoint: endpoint, bucketName: bucketName, publicBaseURL: publicBaseURL, bucket: bucket}, nil
}

func (s *ossImageStorage) Save(ctx context.Context, req imageUploadRequest) (UploadedImage, error) {
	if req.Reader == nil {
		return UploadedImage{}, errors.New("image reader is required")
	}
	if s == nil || s.bucket == nil {
		return UploadedImage{}, errors.New("OSS 图片存储不可用")
	}
	select {
	case <-ctx.Done():
		return UploadedImage{}, ctx.Err()
	default:
	}

	fileName := sanitizeImageFileName(req.FileName)
	objectKey := path.Join(ossImagePrefix, time.Now().In(time.Local).Format("2006/01/02"), uniqueImageName(fileName))
	contentType := strings.TrimSpace(req.ContentType)
	reader := &countingReader{r: req.Reader}
	options := []oss.Option{oss.WithContext(ctx)}
	if contentType != "" {
		options = append(options, oss.ContentType(contentType))
	}
	if err := s.bucket.PutObject(objectKey, reader, options...); err != nil {
		return UploadedImage{}, err
	}
	return UploadedImage{
		URL:        s.publicBaseURL + "/" + objectKey,
		StorageKey: "oss:" + objectKey,
		Path:       objectKey,
		Bytes:      reader.n,
		MimeType:   contentType,
	}, nil
}

type countingReader struct {
	r io.Reader
	n int64
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	r.n += int64(n)
	return n, err
}

func sanitizeImageFileName(name string) string {
	name = path.Base(strings.ReplaceAll(strings.TrimSpace(name), "\\", "/"))
	if name == "." || name == "/" || name == "" {
		return "image.png"
	}
	name = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '.', r == '-', r == '_':
			return r
		default:
			return '-'
		}
	}, name)
	name = strings.Trim(name, ".-")
	if name == "" {
		return "image.png"
	}
	return name
}

func uniqueImageName(fileName string) string {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return fmt.Sprintf("%d-%s", time.Now().UnixNano(), fileName)
	}
	return hex.EncodeToString(buf[:]) + "-" + fileName
}
