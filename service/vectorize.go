package service

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/basketikun/infinite-canvas/config"
)

const (
	vectorizeMaxInputBytes = 40 << 20
	vectorizeMimeType      = "image/svg+xml"
)

type VectorizeInput struct {
	ImageURL string
	DataURL  string
}

type VectorizeResult struct {
	Content  string `json:"content"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Bytes    int    `json:"bytes"`
	MimeType string `json:"mimeType"`
	Engine   string `json:"engine"`
}

func VectorizeImage(input VectorizeInput) (VectorizeResult, error) {
	data, ext, err := readVectorizeInput(input)
	if err != nil {
		return VectorizeResult{}, err
	}
	tempDir, err := os.MkdirTemp("", "infinite-canvas-vectorize-*")
	if err != nil {
		return VectorizeResult{}, err
	}
	defer os.RemoveAll(tempDir)

	inputPath := filepath.Join(tempDir, "input"+ext)
	outputPath := filepath.Join(tempDir, "output.svg")
	if err := os.WriteFile(inputPath, data, 0o600); err != nil {
		return VectorizeResult{}, err
	}

	timeout := time.Duration(config.Cfg.VTracerTimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	args := []string{
		"--input", inputPath,
		"--output", outputPath,
		"--preset", "poster",
		"--mode", "spline",
		"--colormode", "color",
		"--hierarchical", "stacked",
		"--filter_speckle", "4",
		"--color_precision", "6",
		"--gradient_step", "16",
		"--corner_threshold", "60",
		"--segment_length", "4",
		"--splice_threshold", "45",
		"--path_precision", "3",
	}
	vtracerPath := strings.TrimSpace(config.Cfg.VTracerPath)
	if vtracerPath == "" {
		vtracerPath = "vtracer"
	}
	cmd := exec.CommandContext(ctx, vtracerPath, args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return VectorizeResult{}, safeMessageError{message: "VTracer 转换超时，请稍后重试"}
		}
		if _, lookErr := exec.LookPath(vtracerPath); lookErr != nil {
			return VectorizeResult{}, safeMessageError{message: "后端未安装 VTracer，请配置 VTRACER_PATH"}
		}
		return VectorizeResult{}, fmt.Errorf("vtracer failed: %w: %s", err, strings.TrimSpace(string(output)))
	}

	svg, err := os.ReadFile(outputPath)
	if err != nil {
		return VectorizeResult{}, err
	}
	width, height := readSVGSizeText(string(svg))
	return VectorizeResult{
		Content:  string(svg),
		Width:    width,
		Height:   height,
		Bytes:    len(svg),
		MimeType: vectorizeMimeType,
		Engine:   "vtracer",
	}, nil
}

func readVectorizeInput(input VectorizeInput) ([]byte, string, error) {
	dataURL := strings.TrimSpace(input.DataURL)
	imageURL := strings.TrimSpace(input.ImageURL)
	if dataURL != "" {
		if strings.HasPrefix(strings.ToLower(dataURL), "http://") || strings.HasPrefix(strings.ToLower(dataURL), "https://") {
			return readVectorizeURL(dataURL)
		}
		return readVectorizeDataURL(dataURL)
	}
	if imageURL != "" {
		return readVectorizeURL(imageURL)
	}
	return nil, "", safeMessageError{message: "缺少需要转 SVG 的图片"}
}

func readVectorizeDataURL(value string) ([]byte, string, error) {
	header, body, ok := strings.Cut(strings.TrimSpace(value), ",")
	if !ok || !strings.HasPrefix(strings.ToLower(header), "data:image/") {
		return nil, "", safeMessageError{message: "图片数据格式不支持"}
	}
	if !strings.Contains(strings.ToLower(header), ";base64") {
		return nil, "", safeMessageError{message: "图片数据必须是 base64"}
	}
	if len(body) > vectorizeMaxInputBytes*2 {
		return nil, "", safeMessageError{message: "图片过大，无法转 SVG"}
	}
	data, err := base64.StdEncoding.DecodeString(body)
	if err != nil {
		return nil, "", safeMessageError{message: "图片数据解析失败"}
	}
	if len(data) > vectorizeMaxInputBytes {
		return nil, "", safeMessageError{message: "图片过大，无法转 SVG"}
	}
	return data, imageExtFromMime(header), nil
}

func readVectorizeURL(value string) ([]byte, string, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, "", safeMessageError{message: "图片地址格式不支持"}
	}
	client := http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(parsed.String())
	if err != nil {
		return nil, "", safeMessageError{message: "读取图片失败"}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", safeMessageError{message: "读取图片失败"}
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, vectorizeMaxInputBytes+1))
	if err != nil {
		return nil, "", err
	}
	if len(data) > vectorizeMaxInputBytes {
		return nil, "", safeMessageError{message: "图片过大，无法转 SVG"}
	}
	return data, imageExtFromMime(resp.Header.Get("Content-Type")), nil
}

func imageExtFromMime(value string) string {
	lower := strings.ToLower(value)
	switch {
	case strings.Contains(lower, "jpeg"), strings.Contains(lower, "jpg"):
		return ".jpg"
	case strings.Contains(lower, "webp"):
		return ".webp"
	default:
		return ".png"
	}
}

func readSVGSizeText(svg string) (int, int) {
	width := parsePositiveIntAttribute(svg, "width")
	height := parsePositiveIntAttribute(svg, "height")
	if width > 0 && height > 0 {
		return width, height
	}
	return 1024, 768
}

func parsePositiveIntAttribute(svg string, name string) int {
	prefix := name + "=\""
	start := strings.Index(svg, prefix)
	if start < 0 {
		return 0
	}
	start += len(prefix)
	end := strings.Index(svg[start:], "\"")
	if end < 0 {
		return 0
	}
	var value int
	for _, ch := range svg[start : start+end] {
		if ch < '0' || ch > '9' {
			break
		}
		value = value*10 + int(ch-'0')
	}
	return value
}
