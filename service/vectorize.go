package service

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/basketikun/infinite-canvas/config"
)

const (
	vectorizeMaxInputBytes = 40 << 20
	vectorizeMimeType      = "image/svg+xml"
	logoLayerExcludeRadius = 2
	logoSmallMaxWidth      = 300
	logoSmallMaxHeight     = 300
	logoSmallMaxArea       = 40000
	logoPotraceScale       = 4
)

var svgFillHexPattern = regexp.MustCompile(`fill="#([0-9A-Fa-f]{6})"`)
var svgPathPattern = regexp.MustCompile(`(?s)<path\b[^>]*>`)
var svgGroupPattern = regexp.MustCompile(`(?s)<g\b.*</g>`)

type logoPaletteColor struct {
	Count int
	R     int
	G     int
	B     int
	Hex   string
}

type logoColorLayer struct {
	Colors []logoPaletteColor
	R      int
	G      int
	B      int
	Hex    string
	Count  int
}

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

	timeout := time.Duration(config.Cfg.VTracerTimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if isLogoVectorizeMode(input.Mode) {
		if err := vectorizeLogoImage(ctx, tempDir, inputPath, outputPath); err != nil {
			return VectorizeResult{}, err
		}
	} else {
		args := vectorizeArgs(inputPath, outputPath)
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
	}

	svg, err := os.ReadFile(outputPath)
	if err != nil {
		return VectorizeResult{}, err
	}
	if isLogoVectorizeMode(input.Mode) {
		svg = normalizeLogoSVG(svg)
	}
	if !strings.Contains(strings.ToLower(string(svg[:min(len(svg), 512)])), "<svg") {
		return VectorizeResult{}, safeMessageError{message: "VTracer 没有生成有效 SVG"}
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

func vectorizeArgs(inputPath string, outputPath string) []string {
	args := []string{
		"--input", inputPath,
		"--output", outputPath,
		"--preset", "poster",
		"--mode", "spline",
		"--colormode", "color",
	}
	return append(args,
		"--hierarchical", "stacked",
		"--filter_speckle", "4",
		"--color_precision", "6",
		"--gradient_step", "16",
		"--corner_threshold", "60",
		"--segment_length", "4",
		"--splice_threshold", "45",
		"--path_precision", "3",
	)
}

func isLogoVectorizeMode(mode string) bool {
	return strings.EqualFold(strings.TrimSpace(mode), "logo")
}

func vectorizeLogoImage(ctx context.Context, tempDir string, inputPath string, outputPath string) error {
	imageMagickPath, err := resolveImageMagickPath()
	if err != nil {
		return err
	}
	vtracerPath := strings.TrimSpace(config.Cfg.VTracerPath)
	if vtracerPath == "" {
		vtracerPath = "vtracer"
	}
	if _, err := exec.LookPath(vtracerPath); err != nil {
		return safeMessageError{message: "后端未安装 VTracer，请配置 VTRACER_PATH"}
	}
	mkbitmapPath, err := resolveRequiredCommand("mkbitmap", "后端未安装 mkbitmap，请安装 potrace")
	if err != nil {
		return err
	}
	potracePath, err := resolveRequiredCommand("potrace", "后端未安装 potrace，请安装 potrace")
	if err != nil {
		return err
	}

	quantizedPath := filepath.Join(tempDir, "logo-quantized.png")
	if err := preprocessLogoImage(ctx, imageMagickPath, inputPath, quantizedPath); err != nil {
		return err
	}
	palette, err := readLogoPalette(ctx, imageMagickPath, quantizedPath)
	if err != nil {
		return err
	}
	layers := buildLogoColorLayers(palette)
	if len(layers) == 0 {
		return safeMessageError{message: "Logo 图片没有可追踪的有效色块"}
	}

	width, height, err := readPNGSize(quantizedPath)
	if err != nil {
		return err
	}
	graySourcePath := filepath.Join(tempDir, "logo-source-gray.pgm")
	if err := createLogoGraySource(ctx, imageMagickPath, inputPath, width, height, graySourcePath); err != nil {
		return err
	}
	sort.SliceStable(layers, func(i, j int) bool {
		return logoLuma(layers[i].R, layers[i].G, layers[i].B) > logoLuma(layers[j].R, layers[j].G, layers[j].B)
	})

	var body strings.Builder
	body.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	body.WriteString(fmt.Sprintf(`<svg version="1.1" xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`, width, height, width, height) + "\n")
	body.WriteString(fmt.Sprintf(`<rect width="%d" height="%d" fill="#FFFFFF"/>`, width, height) + "\n")
	for index, layer := range layers {
		maskPath := filepath.Join(tempDir, fmt.Sprintf("logo-layer-%d.png", index))
		if err := createLogoLayerMask(ctx, imageMagickPath, quantizedPath, layer.Colors, darkerLogoLayerColors(layers[index+1:]), maskPath); err != nil {
			return err
		}
		largeMaskPath := filepath.Join(tempDir, fmt.Sprintf("logo-layer-%d-large.png", index))
		smallMaskPath := filepath.Join(tempDir, fmt.Sprintf("logo-layer-%d-small.png", index))
		hasLarge, hasSmall, err := splitLogoLayerComponents(maskPath, largeMaskPath, smallMaskPath)
		if err != nil {
			return err
		}
		if hasLarge {
			layerSVGPath := filepath.Join(tempDir, fmt.Sprintf("logo-layer-%d-large.svg", index))
			if err := runLogoLayerVTracer(ctx, vtracerPath, largeMaskPath, layerSVGPath); err != nil {
				return err
			}
			paths, err := readLogoLayerPaths(layerSVGPath, layer.Hex)
			if err != nil {
				return err
			}
			body.WriteString(paths)
		}
		if hasSmall {
			smallGrayPath := filepath.Join(tempDir, fmt.Sprintf("logo-layer-%d-small-gray.pgm", index))
			smallSVGPath := filepath.Join(tempDir, fmt.Sprintf("logo-layer-%d-small.svg", index))
			if err := createLogoSmallGraySource(ctx, imageMagickPath, graySourcePath, smallMaskPath, smallGrayPath); err != nil {
				return err
			}
			if err := runLogoSmallPotrace(ctx, imageMagickPath, mkbitmapPath, potracePath, smallGrayPath, smallSVGPath, layer.Hex); err != nil {
				return err
			}
			group, err := readLogoPotraceGroup(smallSVGPath, layer.Hex, logoPotraceScale)
			if err != nil {
				return err
			}
			body.WriteString(group)
		}
	}
	body.WriteString("</svg>\n")
	return os.WriteFile(outputPath, []byte(body.String()), 0o600)
}

func preprocessLogoImage(ctx context.Context, imageMagickPath string, inputPath string, outputPath string) error {
	maxColors := config.Cfg.LogoVectorizeColors
	if maxColors < 2 {
		maxColors = 2
	}
	if maxColors > 24 {
		maxColors = 24
	}
	args := []string{
		inputPath,
		"-auto-orient",
		"-background", "white",
		"-alpha", "remove",
		"-alpha", "off",
		// Point resize creates stair-step logo edges that VTracer preserves as jagged paths.
		"-filter", "Lanczos",
		"-resize", "400%",
		"-resize", "4096x4096>",
		"-colorspace", "sRGB",
		"-fuzz", "3%",
		"-fill", "white",
		"-opaque", "white",
		"-colors", strconv.Itoa(maxColors),
		"-dither", "none",
		"-strip",
		outputPath,
	}
	cmd := exec.CommandContext(ctx, imageMagickPath, args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return safeMessageError{message: "Logo 图片预处理超时，请稍后重试"}
		}
		return fmt.Errorf("imagemagick failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func createLogoGraySource(ctx context.Context, imageMagickPath string, inputPath string, width int, height int, outputPath string) error {
	args := []string{
		inputPath,
		"-auto-orient",
		"-background", "white",
		"-alpha", "remove",
		"-alpha", "off",
		"-resize", fmt.Sprintf("%dx%d!", width, height),
		"-colorspace", "Gray",
		outputPath,
	}
	cmd := exec.CommandContext(ctx, imageMagickPath, args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return safeMessageError{message: "Logo 灰度源生成超时，请稍后重试"}
		}
		return fmt.Errorf("imagemagick gray source failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func readLogoPalette(ctx context.Context, imageMagickPath string, imagePath string) ([]logoPaletteColor, error) {
	cmd := exec.CommandContext(ctx, imageMagickPath, imagePath, "-format", "%c", "-depth", "8", "histogram:info:-")
	output, err := cmd.Output()
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, safeMessageError{message: "Logo 图片取色超时，请稍后重试"}
		}
		return nil, err
	}
	lines := strings.Split(string(output), "\n")
	pattern := regexp.MustCompile(`^\s*(\d+):\s+\((\d+),(\d+),(\d+)`)
	colors := make([]logoPaletteColor, 0, len(lines))
	for _, line := range lines {
		match := pattern.FindStringSubmatch(line)
		if len(match) != 5 {
			continue
		}
		count, _ := strconv.Atoi(match[1])
		r, _ := strconv.Atoi(match[2])
		g, _ := strconv.Atoi(match[3])
		b, _ := strconv.Atoi(match[4])
		colors = append(colors, logoPaletteColor{
			Count: count,
			R:     r,
			G:     g,
			B:     b,
			Hex:   fmt.Sprintf("#%02X%02X%02X", r, g, b),
		})
	}
	sort.SliceStable(colors, func(i, j int) bool {
		return colors[i].Count > colors[j].Count
	})
	return colors, nil
}

func buildLogoColorLayers(colors []logoPaletteColor) []logoColorLayer {
	layers := make([]logoColorLayer, 0, len(colors))
	for _, color := range colors {
		if isLogoBackgroundColor(color.R, color.G, color.B) {
			continue
		}
		nearestIndex := -1
		nearestDistance := math.MaxFloat64
		for index, layer := range layers {
			distance := logoColorDistance(color.R, color.G, color.B, layer.R, layer.G, layer.B)
			if distance < nearestDistance {
				nearestDistance = distance
				nearestIndex = index
			}
		}
		if nearestIndex >= 0 && nearestDistance <= 96 {
			layer := &layers[nearestIndex]
			layer.Colors = append(layer.Colors, color)
			layer.Count += color.Count
			continue
		}
		layers = append(layers, logoColorLayer{
			Colors: []logoPaletteColor{color},
			R:      color.R,
			G:      color.G,
			B:      color.B,
			Hex:    color.Hex,
			Count:  color.Count,
		})
	}
	return layers
}

func darkerLogoLayerColors(layers []logoColorLayer) []logoPaletteColor {
	colors := make([]logoPaletteColor, 0, len(layers))
	for _, layer := range layers {
		colors = append(colors, layer.Colors...)
	}
	return colors
}

func splitLogoLayerComponents(inputPath string, largePath string, smallPath string) (bool, bool, error) {
	file, err := os.Open(inputPath)
	if err != nil {
		return false, false, err
	}
	defer file.Close()
	img, _, err := image.Decode(file)
	if err != nil {
		return false, false, err
	}
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	visited := make([]bool, width*height)
	large := image.NewRGBA(image.Rect(0, 0, width, height))
	small := image.NewRGBA(image.Rect(0, 0, width, height))
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	black := color.RGBA{A: 255}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			large.Set(x, y, white)
			small.Set(x, y, white)
		}
	}
	hasLarge := false
	hasSmall := false
	queue := make([]image.Point, 0, 1024)
	component := make([]image.Point, 0, 1024)
	directions := [...]image.Point{{X: 1}, {X: -1}, {Y: 1}, {Y: -1}}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			index := y*width + x
			if visited[index] || !isLogoMaskPixel(img.At(bounds.Min.X+x, bounds.Min.Y+y)) {
				continue
			}
			visited[index] = true
			queue = append(queue[:0], image.Point{X: x, Y: y})
			component = component[:0]
			minX, maxX, minY, maxY := x, x, y, y
			for len(queue) > 0 {
				point := queue[len(queue)-1]
				queue = queue[:len(queue)-1]
				component = append(component, point)
				if point.X < minX {
					minX = point.X
				}
				if point.X > maxX {
					maxX = point.X
				}
				if point.Y < minY {
					minY = point.Y
				}
				if point.Y > maxY {
					maxY = point.Y
				}
				for _, direction := range directions {
					next := image.Point{X: point.X + direction.X, Y: point.Y + direction.Y}
					if next.X < 0 || next.X >= width || next.Y < 0 || next.Y >= height {
						continue
					}
					nextIndex := next.Y*width + next.X
					if visited[nextIndex] || !isLogoMaskPixel(img.At(bounds.Min.X+next.X, bounds.Min.Y+next.Y)) {
						continue
					}
					visited[nextIndex] = true
					queue = append(queue, next)
				}
			}
			target := large
			if maxX-minX+1 <= logoSmallMaxWidth && maxY-minY+1 <= logoSmallMaxHeight && len(component) <= logoSmallMaxArea {
				target = small
				hasSmall = true
			} else {
				hasLarge = true
			}
			for _, point := range component {
				target.Set(point.X, point.Y, black)
			}
		}
	}
	if err := writePNG(largePath, large); err != nil {
		return false, false, err
	}
	if err := writePNG(smallPath, small); err != nil {
		return false, false, err
	}
	return hasLarge, hasSmall, nil
}

func isLogoMaskPixel(c color.Color) bool {
	r, g, b, _ := c.RGBA()
	return r < 0x8000 && g < 0x8000 && b < 0x8000
}

func writePNG(path string, img image.Image) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return png.Encode(file, img)
}

func createLogoLayerMask(ctx context.Context, imageMagickPath string, inputPath string, colors []logoPaletteColor, excludeColors []logoPaletteColor, outputPath string) error {
	const maskMarkerColor = "#010203"
	args := []string{inputPath}
	for _, color := range colors {
		args = append(args, "-fill", maskMarkerColor, "-opaque", color.Hex)
	}
	args = append(args,
		"-fill", "white", "+opaque", maskMarkerColor,
		"-fill", "black", "-opaque", maskMarkerColor,
		"-type", "bilevel",
		"-strip",
		outputPath,
	)
	cmd := exec.CommandContext(ctx, imageMagickPath, args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return safeMessageError{message: "Logo 色层清理超时，请稍后重试"}
		}
		return fmt.Errorf("imagemagick mask failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	if len(excludeColors) > 0 {
		excludePath := strings.TrimSuffix(outputPath, filepath.Ext(outputPath)) + "-exclude.png"
		args = []string{inputPath}
		for _, color := range excludeColors {
			args = append(args, "-fill", maskMarkerColor, "-opaque", color.Hex)
		}
		args = append(args,
			"-fill", "black", "+opaque", maskMarkerColor,
			"-fill", "white", "-opaque", maskMarkerColor,
			"-type", "bilevel",
			"-morphology", "Dilate", fmt.Sprintf("Disk:%d", logoLayerExcludeRadius),
			"-strip",
			excludePath,
		)
		cmd = exec.CommandContext(ctx, imageMagickPath, args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return safeMessageError{message: "Logo 色层贴边清理超时，请稍后重试"}
			}
			return fmt.Errorf("imagemagick exclude mask failed: %w: %s", err, strings.TrimSpace(string(output)))
		}
		cmd = exec.CommandContext(ctx, imageMagickPath, outputPath, excludePath, "-compose", "Lighten", "-composite", "-type", "bilevel", outputPath)
		if output, err := cmd.CombinedOutput(); err != nil {
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return safeMessageError{message: "Logo 色层贴边清理超时，请稍后重试"}
			}
			return fmt.Errorf("imagemagick exclude composite failed: %w: %s", err, strings.TrimSpace(string(output)))
		}
	}
	cmd = exec.CommandContext(ctx, imageMagickPath, outputPath, "-blur", "0x0.8", "-threshold", "50%", "-type", "bilevel", outputPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return safeMessageError{message: "Logo 色层几何平滑超时，请稍后重试"}
		}
		return fmt.Errorf("imagemagick smooth mask failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func createLogoSmallGraySource(ctx context.Context, imageMagickPath string, graySourcePath string, maskPath string, outputPath string) error {
	expandedMaskPath := strings.TrimSuffix(outputPath, filepath.Ext(outputPath)) + "-mask.png"
	alphaPath := strings.TrimSuffix(outputPath, filepath.Ext(outputPath)) + "-alpha.png"
	cmd := exec.CommandContext(ctx, imageMagickPath, maskPath, "-negate", "-morphology", "Dilate", "Disk:2", "-negate", expandedMaskPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return safeMessageError{message: "Logo 小组件遮罩扩展超时，请稍后重试"}
		}
		return fmt.Errorf("imagemagick small mask failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	cmd = exec.CommandContext(ctx, imageMagickPath, expandedMaskPath, "-negate", alphaPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return safeMessageError{message: "Logo 小组件遮罩扩展超时，请稍后重试"}
		}
		return fmt.Errorf("imagemagick small alpha failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	cmd = exec.CommandContext(ctx, imageMagickPath, graySourcePath, alphaPath, "-alpha", "off", "-compose", "CopyOpacity", "-composite", "-background", "white", "-alpha", "remove", outputPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return safeMessageError{message: "Logo 小组件灰度源生成超时，请稍后重试"}
		}
		return fmt.Errorf("imagemagick small gray failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func runLogoSmallPotrace(ctx context.Context, imageMagickPath string, mkbitmapPath string, potracePath string, inputPath string, outputPath string, fill string) error {
	blurredPath := strings.TrimSuffix(outputPath, filepath.Ext(outputPath)) + "-gray.pgm"
	bitmapPath := strings.TrimSuffix(outputPath, filepath.Ext(outputPath)) + ".pbm"
	cmd := exec.CommandContext(ctx, imageMagickPath, inputPath, "-blur", "0x0.15", blurredPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return safeMessageError{message: "Logo 小组件灰度平滑超时，请稍后重试"}
		}
		return fmt.Errorf("imagemagick small blur failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	args := []string{"-x", "-n", "-s", strconv.Itoa(logoPotraceScale), "-3", "-t", "0.57", "-o", bitmapPath, blurredPath}
	cmd = exec.CommandContext(ctx, mkbitmapPath, args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return safeMessageError{message: "mkbitmap 小组件转换超时，请稍后重试"}
		}
		return fmt.Errorf("mkbitmap failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	args = []string{bitmapPath, "-s", "--group", "--flat", "-t", "10", "-a", "0.60", "-O", "0.95", "-u", "20", "-C", fill, "-o", outputPath}
	cmd = exec.CommandContext(ctx, potracePath, args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return safeMessageError{message: "potrace 小组件描摹超时，请稍后重试"}
		}
		return fmt.Errorf("potrace failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func runLogoLayerVTracer(ctx context.Context, vtracerPath string, inputPath string, outputPath string) error {
	args := []string{
		"--input", inputPath,
		"--output", outputPath,
		"--preset", "bw",
		"--mode", "spline",
		"--colormode", "bw",
		"--filter_speckle", "16",
		"--corner_threshold", "80",
		"--segment_length", "8",
		"--splice_threshold", "75",
		"--path_precision", "3",
	}
	cmd := exec.CommandContext(ctx, vtracerPath, args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return safeMessageError{message: "VTracer 转换超时，请稍后重试"}
		}
		return fmt.Errorf("vtracer layer failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func readLogoLayerPaths(svgPath string, fill string) (string, error) {
	data, err := os.ReadFile(svgPath)
	if err != nil {
		return "", err
	}
	matches := svgPathPattern.FindAllString(string(data), -1)
	if len(matches) == 0 {
		return "", nil
	}
	var body strings.Builder
	body.WriteString(fmt.Sprintf(`<g fill="%s">`, fill) + "\n")
	for _, path := range matches {
		path = svgFillHexPattern.ReplaceAllString(path, fmt.Sprintf(`fill="%s"`, fill))
		body.WriteString(path)
		body.WriteString("\n")
	}
	body.WriteString("</g>\n")
	return body.String(), nil
}

func readLogoPotraceGroup(svgPath string, fill string, scale int) (string, error) {
	data, err := os.ReadFile(svgPath)
	if err != nil {
		return "", err
	}
	group := svgGroupPattern.FindString(string(data))
	if group == "" {
		return "", nil
	}
	group = svgFillHexPattern.ReplaceAllString(group, fmt.Sprintf(`fill="%s"`, fill))
	return fmt.Sprintf(`<g transform="scale(%.10f)">`, 1/float64(scale)) + "\n" + group + "\n</g>\n", nil
}

func readPNGSize(imagePath string) (int, int, error) {
	file, err := os.Open(imagePath)
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()
	config, _, err := image.DecodeConfig(file)
	if err != nil {
		return 0, 0, err
	}
	return config.Width, config.Height, nil
}

func isLogoBackgroundColor(r int, g int, b int) bool {
	maxChannel := max(r, max(g, b))
	minChannel := min(r, min(g, b))
	return r >= 248 && g >= 248 && b >= 248 || logoLuma(r, g, b) >= 235 && maxChannel-minChannel <= 35
}

func logoColorDistance(r1 int, g1 int, b1 int, r2 int, g2 int, b2 int) float64 {
	dr := float64(r1 - r2)
	dg := float64(g1 - g2)
	db := float64(b1 - b2)
	return math.Sqrt(dr*dr + dg*dg + db*db)
}

func logoLuma(r int, g int, b int) float64 {
	return 0.2126*float64(r) + 0.7152*float64(g) + 0.0722*float64(b)
}

func normalizeLogoSVG(svg []byte) []byte {
	return []byte(svgFillHexPattern.ReplaceAllStringFunc(string(svg), func(token string) string {
		hex := token[len(`fill="#`) : len(token)-1]
		rgb, err := strconv.ParseUint(hex, 16, 32)
		if err != nil {
			return token
		}
		r := uint8(rgb >> 16)
		g := uint8(rgb >> 8)
		b := uint8(rgb)
		if r >= 248 && g >= 248 && b >= 248 {
			return `fill="#FFFFFF"`
		}
		return token
	}))
}

func resolveImageMagickPath() (string, error) {
	if configured := strings.TrimSpace(config.Cfg.ImageMagickPath); configured != "" {
		if _, err := exec.LookPath(configured); err != nil {
			return "", safeMessageError{message: "后端未找到 ImageMagick，请检查 IMAGE_MAGICK_PATH"}
		}
		return configured, nil
	}
	if path, err := exec.LookPath("magick"); err == nil {
		return path, nil
	}
	if path, err := exec.LookPath("convert"); err == nil {
		return path, nil
	}
	return "", safeMessageError{message: "后端未安装 ImageMagick，无法执行 Logo 高清放大和限色清理"}
}

func resolveRequiredCommand(name string, message string) (string, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", safeMessageError{message: message}
	}
	return path, nil
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
