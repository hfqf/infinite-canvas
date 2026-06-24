package handler

import (
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/basketikun/infinite-canvas/service"
)

type imageUploadResult struct {
	URL        string `json:"url"`
	StorageKey string `json:"storageKey"`
	Path       string `json:"path"`
	Bytes      int64  `json:"bytes"`
	MimeType   string `json:"mimeType"`
}

func UploadImage(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, referenceImageMaxBytes+1)
	if err := r.ParseMultipartForm(referenceImageMaxBytes); err != nil {
		Fail(w, "图片过大或上传格式不正确")
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		Fail(w, "请上传图片")
		return
	}
	defer file.Close()
	if header.Size > referenceImageMaxBytes {
		Fail(w, referenceMediaSizeMessage("image/"))
		return
	}

	mimeType, _, ok := normalizeReferenceMediaType(header.Header.Get("Content-Type"), filepath.Ext(header.Filename))
	if !ok || !strings.HasPrefix(mimeType, "image/") {
		Fail(w, "图片格式不支持，请使用 "+referenceImageAllowedText)
		return
	}
	reader := io.LimitReader(file, referenceImageMaxBytes+1)
	result, err := service.SaveUploadedImage(r.Context(), header.Filename, mimeType, reader)
	if err != nil {
		FailError(w, err)
		return
	}
	if result.Bytes <= 0 {
		Fail(w, "图片为空")
		return
	}
	if result.Bytes > referenceImageMaxBytes {
		Fail(w, referenceMediaSizeMessage(mimeType))
		return
	}
	OK(w, imageUploadResult{
		URL:        result.URL,
		StorageKey: result.StorageKey,
		Path:       result.Path,
		Bytes:      result.Bytes,
		MimeType:   result.MimeType,
	})
}
