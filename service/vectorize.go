package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
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
	recraftMaxInputBytes   = 10 << 20
	vectorizeMimeType      = "image/svg+xml"
)

type VectorizeInput struct {
	ImageURL string `json:"imageUrl"`
	DataURL  string `json:"dataUrl"`
	Mode     string `json:"mode"`
}

type VectorizeResult struct {
	Content  string           `json:"content"`
	Width    int              `json:"width"`
	Height   int              `json:"height"`
	Bytes    int              `json:"bytes"`
	MimeType string           `json:"mimeType"`
	Engine   string           `json:"engine"`
	Preset   *VectorizePreset `json:"preset,omitempty"`
}

type VectorizePreset struct {
	Name              string  `json:"name"`
	Engine            string  `json:"engine"`
	Colors            int     `json:"colors,omitempty"`
	LongEdge          int     `json:"longEdge"`
	MinComponentRatio float64 `json:"minComponentRatio"`
	MaxHoleRatio      float64 `json:"maxHoleRatio"`
	MergeDistance     int     `json:"mergeDistance"`
	MergeHueDistance  int     `json:"mergeHueDistance"`
	MergeLightness    int     `json:"mergeLightness"`
	MergeSaturation   float64 `json:"mergeSaturation"`
	LightMinAreaRatio float64 `json:"lightMinAreaRatio"`
	MaskCloseRadius   int     `json:"maskCloseRadius"`
	LightDilateRadius int     `json:"lightDilateRadius"`
	DarkDilateRadius  int     `json:"darkDilateRadius"`
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

	timeout := vectorizeTimeout(input.Mode)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	engine := vectorizeEngine(input.Mode)
	preset := vectorizePreset(input.Mode)
	if shouldUseRecraftVectorize() {
		if err := runRecraftVectorize(ctx, data, ext, outputPath); err != nil {
			return VectorizeResult{}, err
		}
		engine = "recraft-vectorize"
		preset = nil
	} else if isCleanLogoVectorizeMode(input.Mode) {
		if err := runCleanLogoVectorize(ctx, inputPath, outputPath); err != nil {
			return VectorizeResult{}, err
		}
	} else if isIllustrationVectorizeMode(input.Mode) {
		if err := runIllustrationVectorize(ctx, inputPath, outputPath); err != nil {
			return VectorizeResult{}, err
		}
	} else {
		if err := runPng2SVGClean(ctx, inputPath, outputPath); err != nil {
			return VectorizeResult{}, err
		}
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
		Engine:   engine,
		Preset:   preset,
	}, nil
}

func vectorizeTimeout(mode string) time.Duration {
	seconds := config.Cfg.Png2SVGCleanTimeoutSec
	if isCleanLogoVectorizeMode(mode) {
		seconds = config.Cfg.VectorizeLogoTimeoutSec
	} else if isIllustrationVectorizeMode(mode) {
		seconds = config.Cfg.VectorizeIllustrationTimeoutSec
	}
	if seconds <= 0 {
		if isIllustrationVectorizeMode(mode) {
			seconds = 180
		} else if isCleanLogoVectorizeMode(mode) {
			seconds = 120
		} else {
			seconds = 90
		}
	}
	return time.Duration(seconds) * time.Second
}

func runIllustrationVectorize(ctx context.Context, inputPath string, outputPath string) error {
	return runCleanLogoPotraceVectorize(ctx, inputPath, outputPath, illustrationVectorizeOptions())
}

type recraftVectorizeResponse struct {
	Image struct {
		URL string `json:"url"`
	} `json:"image"`
	URL      string `json:"url"`
	ImageURL string `json:"image_url"`
}

func shouldUseRecraftVectorize() bool {
	provider := strings.ToLower(strings.TrimSpace(config.Cfg.VectorizeProvider))
	return provider == "recraft" && strings.TrimSpace(config.Cfg.RecraftAPIKey) != ""
}

func runRecraftVectorize(ctx context.Context, data []byte, ext string, outputPath string) error {
	apiKey := strings.TrimSpace(config.Cfg.RecraftAPIKey)
	if apiKey == "" {
		return safeMessageError{message: "未配置 Recraft API Key，无法使用外部转矢量服务"}
	}
	if len(data) > recraftMaxInputBytes {
		return safeMessageError{message: "图片超过 Recraft 10MB 限制，请压缩后重试"}
	}
	baseURL := strings.TrimRight(strings.TrimSpace(config.Cfg.RecraftAPIBaseURL), "/")
	if baseURL == "" {
		baseURL = "https://external.api.recraft.ai/v1"
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	filename := "input" + ext
	if ext == "" {
		filename = "input.png"
	}
	fileWriter, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return err
	}
	if _, err := fileWriter.Write(data); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/images/vectorize", &body)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+apiKey)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	client := http.Client{Timeout: vectorizeTimeout("recraft")}
	response, err := client.Do(request)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return safeMessageError{message: "Recraft 转矢量超时，请稍后重试"}
		}
		return safeMessageError{message: "Recraft 转矢量请求失败"}
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return safeMessageError{message: fmt.Sprintf("Recraft 转矢量失败，状态码 %d", response.StatusCode)}
	}
	var payload recraftVectorizeResponse
	if err := json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(&payload); err != nil {
		return safeMessageError{message: "Recraft 转矢量响应解析失败"}
	}
	svgURL := strings.TrimSpace(payload.Image.URL)
	if svgURL == "" {
		svgURL = strings.TrimSpace(payload.URL)
	}
	if svgURL == "" {
		svgURL = strings.TrimSpace(payload.ImageURL)
	}
	if svgURL == "" {
		return safeMessageError{message: "Recraft 转矢量响应缺少 SVG 地址"}
	}
	svg, err := downloadRecraftSVG(ctx, svgURL)
	if err != nil {
		return err
	}
	return os.WriteFile(outputPath, svg, 0o600)
}

func downloadRecraftSVG(ctx context.Context, svgURL string) ([]byte, error) {
	parsed, err := url.Parse(strings.TrimSpace(svgURL))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, safeMessageError{message: "Recraft 返回的 SVG 地址格式不支持"}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, safeMessageError{message: "下载 Recraft SVG 超时，请稍后重试"}
		}
		return nil, safeMessageError{message: "下载 Recraft SVG 失败"}
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, safeMessageError{message: "下载 Recraft SVG 失败"}
	}
	svg, err := io.ReadAll(io.LimitReader(response.Body, vectorizeMaxInputBytes+1))
	if err != nil {
		return nil, err
	}
	if len(svg) > vectorizeMaxInputBytes {
		return nil, safeMessageError{message: "Recraft SVG 过大，无法处理"}
	}
	return svg, nil
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
	if isCleanLogoVectorizeMode(mode) {
		return "clean-logo-potrace"
	}
	if isIllustrationVectorizeMode(mode) {
		return "illustration-potrace"
	}
	return "png2svg-clean-node"
}

func vectorizePreset(mode string) *VectorizePreset {
	if isCleanLogoVectorizeMode(mode) {
		return cleanLogoVectorizePreset()
	}
	if isIllustrationVectorizeMode(mode) {
		return illustrationVectorizePreset()
	}
	return nil
}

func isIllustrationVectorizeMode(mode string) bool {
	normalized := strings.ToLower(strings.TrimSpace(mode))
	return normalized == "illustration" || normalized == "material" || normalized == "asset"
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
