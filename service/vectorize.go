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
	ImageURL string `json:"imageUrl"`
	DataURL  string `json:"dataUrl"`
	Mode     string `json:"mode"`
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

	timeout := time.Duration(config.Cfg.Png2SVGCleanTimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := runPng2SVGClean(ctx, inputPath, outputPath); err != nil {
		return VectorizeResult{}, err
	}

	svg, err := os.ReadFile(outputPath)
	if err != nil {
		return VectorizeResult{}, err
	}
	if !strings.Contains(strings.ToLower(string(svg[:min(len(svg), 512)])), "<svg") {
		return VectorizeResult{}, safeMessageError{message: "转 SVG 工具没有生成有效 SVG"}
	}
	width, height := readSVGSizeText(string(svg))
	return VectorizeResult{
		Content:  string(svg),
		Width:    width,
		Height:   height,
		Bytes:    len(svg),
		MimeType: vectorizeMimeType,
		Engine:   vectorizeEngine(input.Mode),
	}, nil
}

func runPng2SVGClean(ctx context.Context, inputPath string, outputPath string) error {
	nodePath := strings.TrimSpace(config.Cfg.Png2SVGCleanNodePath)
	if nodePath == "" {
		nodePath = "node"
	}
	toolDir, err := resolvePng2SVGCleanToolDir()
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, nodePath, png2SVGCleanArgs(inputPath, outputPath)...)
	cmd.Dir = toolDir
	if output, err := cmd.CombinedOutput(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return safeMessageError{message: "转 SVG 超时，请稍后重试"}
		}
		if _, lookErr := exec.LookPath(nodePath); lookErr != nil {
			return safeMessageError{message: "后端未安装 Node.js，请配置 PNG2SVG_CLEAN_NODE_PATH"}
		}
		return fmt.Errorf("png2svg clean failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func png2SVGCleanArgs(inputPath string, outputPath string) []string {
	bin := strings.TrimSpace(config.Cfg.Png2SVGCleanBin)
	if bin == "" {
		bin = "bin/png2svg-generic-85.mjs"
	}
	profile := strings.TrimSpace(config.Cfg.Png2SVGCleanProfile)
	if profile == "" {
		profile = "generic-85"
	}
	return []string{
		bin,
		inputPath,
		outputPath,
		"--profile",
		profile,
	}
}

func resolvePng2SVGCleanToolDir() (string, error) {
	toolDir := strings.TrimSpace(config.Cfg.Png2SVGCleanToolDir)
	if toolDir == "" {
		toolDir = "png2svg-clean-node"
	}
	if filepath.IsAbs(toolDir) {
		return toolDir, nil
	}
	absolute, err := filepath.Abs(toolDir)
	if err != nil {
		return "", err
	}
	return absolute, nil
}

func vectorizeEngine(mode string) string {
	return "png2svg-clean-node"
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
