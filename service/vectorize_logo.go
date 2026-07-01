package service

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/basketikun/infinite-canvas/config"
)

const (
	cleanLogoLongEdge             = 1024
	illustrationLongEdge          = 1024
	illustrationColors            = 64
	illustrationMaxTraceLayers    = 64
	cleanLogoMinComponentRatio    = 0.00005
	illustrationMinComponentRatio = 0.000005
	cleanLogoMaxHoleRatio         = 0.00008
	illustrationMaxHoleRatio      = 0.00001
	cleanLogoMergeDistance        = 64
	illustrationMergeDistance     = 18
	cleanLogoMergeHueDistance     = 16
	illustrationMergeHueDistance  = 8
	cleanLogoMergeLightness       = 80
	illustrationMergeLightness    = 32
	cleanLogoMergeSaturation      = 0.55
	illustrationMergeSaturation   = 0.3
	cleanLogoLightMinAreaRatio    = 0.004
	illustrationLightMinAreaRatio = 0.0005
)

var (
	cleanLogoGroupPattern     = regexp.MustCompile(`(?s)<g\b.*</g>`)
	cleanLogoHistogramPattern = regexp.MustCompile(`^\s*(\d+):\s+\(([^,]+),([^,]+),([^)]+)\)`)
)

type cleanLogoPaletteColor struct {
	Hex   string
	R     uint8
	G     uint8
	B     uint8
	Count int
}

type cleanLogoTraceLayer struct {
	Hex   string
	R     uint8
	G     uint8
	B     uint8
	Count int
	Keys  map[uint32]struct{}
}

type cleanLogoStats struct {
	ComponentsRemoved int
	PixelsRemoved     int
	HolesFilled       int
	HolePixelsFilled  int
	RemainingBlack    int
}

type cleanLogoHistogramColor struct {
	Count int
	R     uint8
	G     uint8
	B     uint8
}

type cleanLogoVectorizeOptions struct {
	Name                  string
	Engine                string
	Colors                int
	LongEdge              int
	MinComponentRatio     float64
	MaxHoleRatio          float64
	MergeDistance         float64
	MergeHueDistance      float64
	MergeLightness        float64
	MergeSaturation       float64
	LightMinAreaRatio     float64
	MaxTraceLayers        int
	PreservePaletteLayers bool
	RemoveSpeckles        bool
	FillSmallHoles        bool
}

func isCleanLogoVectorizeMode(mode string) bool {
	normalized := strings.ToLower(strings.TrimSpace(mode))
	return normalized == "logo" || normalized == "cleanlogo" || normalized == "clean-logo"
}

func cleanLogoVectorizePreset() *VectorizePreset {
	options := cleanLogoVectorizeOptionsPreset()
	return &VectorizePreset{
		Name:              options.Name,
		Engine:            options.Engine,
		Colors:            options.Colors,
		LongEdge:          options.LongEdge,
		MinComponentRatio: options.MinComponentRatio,
		MaxHoleRatio:      options.MaxHoleRatio,
		MergeDistance:     int(options.MergeDistance),
		MergeHueDistance:  int(options.MergeHueDistance),
		MergeLightness:    int(options.MergeLightness),
		MergeSaturation:   options.MergeSaturation,
		LightMinAreaRatio: options.LightMinAreaRatio,
		MaskCloseRadius:   0,
		LightDilateRadius: 0,
		DarkDilateRadius:  0,
	}
}

func illustrationVectorizePreset() *VectorizePreset {
	options := illustrationVectorizeOptions()
	return &VectorizePreset{
		Name:              options.Name,
		Engine:            options.Engine,
		Colors:            options.Colors,
		LongEdge:          options.LongEdge,
		MinComponentRatio: options.MinComponentRatio,
		MaxHoleRatio:      options.MaxHoleRatio,
		MergeDistance:     int(options.MergeDistance),
		MergeHueDistance:  int(options.MergeHueDistance),
		MergeLightness:    int(options.MergeLightness),
		MergeSaturation:   options.MergeSaturation,
		LightMinAreaRatio: options.LightMinAreaRatio,
		MaskCloseRadius:   0,
		LightDilateRadius: 0,
		DarkDilateRadius:  0,
	}
}

func cleanLogoVectorizeOptionsPreset() cleanLogoVectorizeOptions {
	return cleanLogoVectorizeOptions{
		Name:              "cleanLogo",
		Engine:            "clean-logo-potrace",
		LongEdge:          cleanLogoLongEdge,
		MinComponentRatio: cleanLogoMinComponentRatio,
		MaxHoleRatio:      cleanLogoMaxHoleRatio,
		MergeDistance:     cleanLogoMergeDistance,
		MergeHueDistance:  cleanLogoMergeHueDistance,
		MergeLightness:    cleanLogoMergeLightness,
		MergeSaturation:   cleanLogoMergeSaturation,
		LightMinAreaRatio: cleanLogoLightMinAreaRatio,
		RemoveSpeckles:    true,
		FillSmallHoles:    true,
	}
}

func illustrationVectorizeOptions() cleanLogoVectorizeOptions {
	return cleanLogoVectorizeOptions{
		Name:                  "illustration",
		Engine:                "illustration-potrace",
		Colors:                illustrationColors,
		LongEdge:              illustrationLongEdge,
		MinComponentRatio:     illustrationMinComponentRatio,
		MaxHoleRatio:          illustrationMaxHoleRatio,
		MergeDistance:         illustrationMergeDistance,
		MergeHueDistance:      illustrationMergeHueDistance,
		MergeLightness:        illustrationMergeLightness,
		MergeSaturation:       illustrationMergeSaturation,
		LightMinAreaRatio:     illustrationLightMinAreaRatio,
		MaxTraceLayers:        illustrationMaxTraceLayers,
		PreservePaletteLayers: true,
		RemoveSpeckles:        false,
		FillSmallHoles:        false,
	}
}

func runCleanLogoVectorize(ctx context.Context, inputPath string, outputPath string) error {
	return runCleanLogoPotraceVectorize(ctx, inputPath, outputPath, cleanLogoVectorizeOptionsPreset())
}

func runCleanLogoPotraceVectorize(ctx context.Context, inputPath string, outputPath string, options cleanLogoVectorizeOptions) error {
	magickPath, err := requireVectorizeCommand(config.Cfg.ImageMagickPath, "magick")
	if err != nil {
		return err
	}
	potracePath, err := requireVectorizeCommand(config.Cfg.PotracePath, "potrace")
	if err != nil {
		return err
	}

	workDir := filepath.Dir(outputPath)
	normalizedPath := filepath.Join(workDir, "clean-logo-normalized.png")
	quantizedPath := filepath.Join(workDir, "clean-logo-quantized.png")
	colors := options.Colors
	if colors <= 0 {
		colors = estimateCleanLogoVectorizeColors(ctx, magickPath, inputPath)
	}

	if err := cleanLogoNormalizePNG(ctx, magickPath, inputPath, normalizedPath, options.LongEdge); err != nil {
		return err
	}
	if err := cleanLogoQuantizePNG(ctx, magickPath, normalizedPath, quantizedPath, colors); err != nil {
		return err
	}
	if err := cleanLogoRestoreLightNeutralAccents(normalizedPath, quantizedPath); err != nil {
		return err
	}

	img, width, height, err := cleanLogoReadPNG(quantizedPath)
	if err != nil {
		return err
	}
	palette := cleanLogoReadPalette(img)
	background := cleanLogoDetectBackgroundWithOptions(img, palette, options)
	layers := cleanLogoBuildTraceLayersWithOptions(palette, width*height, background.Keys, options)
	if options.MaxTraceLayers > 0 && len(layers) > options.MaxTraceLayers {
		layers = layers[:options.MaxTraceLayers]
	}
	if len(layers) == 0 {
		return safeMessageError{message: "未识别到可转 SVG 的图层"}
	}
	sort.SliceStable(layers, func(i, j int) bool {
		return cleanLogoLuma(layers[i].R, layers[i].G, layers[i].B) > cleanLogoLuma(layers[j].R, layers[j].G, layers[j].B)
	})
	if sourceBackground, ok := cleanLogoReadSourceBorderBackground(inputPath); ok {
		background.Hex = cleanLogoHex(sourceBackground.R, sourceBackground.G, sourceBackground.B)
		background.R = sourceBackground.R
		background.G = sourceBackground.G
		background.B = sourceBackground.B
	}

	var body strings.Builder
	body.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	body.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`, width, height, width, height) + "\n")
	body.WriteString(fmt.Sprintf(`<rect width="100%%" height="100%%" fill="%s"/>`, background.Hex) + "\n")

	minComponentArea := int(math.Max(4, float64(width*height)*options.MinComponentRatio))
	maxHoleArea := int(math.Max(4, float64(width*height)*options.MaxHoleRatio))
	for index, layer := range layers {
		if err := ctx.Err(); err != nil {
			return vectorizeCommandError(options.Name, "处理图层", err, "")
		}
		maskPath := filepath.Join(workDir, fmt.Sprintf("clean-logo-layer-%02d-mask.png", index))
		layerSVGPath := filepath.Join(workDir, fmt.Sprintf("clean-logo-layer-%02d.svg", index))
		if err := cleanLogoWriteLayerMask(img, layer, maskPath); err != nil {
			return fmt.Errorf("%s write layer mask %02d failed: %w", options.Name, index, err)
		}
		layerMinComponentArea := cleanLogoMinComponentAreaForLayerWithOptions(layer, width*height, minComponentArea, options)
		stats, err := cleanLogoCleanMaskWithOptions(maskPath, options.RemoveSpeckles, options.FillSmallHoles, layerMinComponentArea, maxHoleArea)
		if err != nil {
			return err
		}
		if stats.RemainingBlack == 0 {
			continue
		}
		if err := cleanLogoRunPotrace(ctx, magickPath, potracePath, maskPath, layerSVGPath, layer.Hex); err != nil {
			return err
		}
		group, err := cleanLogoExtractPotraceGroup(layerSVGPath)
		if err != nil {
			return err
		}
		body.WriteString(group)
		body.WriteString("\n")
	}
	body.WriteString("</svg>\n")
	return os.WriteFile(outputPath, []byte(body.String()), 0o600)
}

func requireVectorizeCommand(configured string, fallback string) (string, error) {
	name := strings.TrimSpace(configured)
	if name == "" {
		name = fallback
	}
	if path, err := exec.LookPath(name); err == nil {
		return path, nil
	}
	return "", safeMessageError{message: fmt.Sprintf("后端未安装 %s，无法使用 Logo 转 SVG 预设", fallback)}
}

func estimateCleanLogoVectorizeColors(ctx context.Context, magickPath string, sourcePath string) int {
	colors, err := cleanLogoReadHistogram(ctx, magickPath, sourcePath)
	if err != nil {
		return 3
	}
	total := 0
	for _, item := range colors {
		total += item.Count
	}
	if total == 0 {
		return 3
	}
	background, hasBackground := cleanLogoDominantHistogramColor(colors)
	borderBackground, hasBorderBackground := cleanLogoReadSourceCornerColor(ctx, magickPath, sourcePath)
	hasDarkBackground := hasBorderBackground && cleanLogoLuma(borderBackground.R, borderBackground.G, borderBackground.B) < 160
	if !hasBorderBackground {
		hasDarkBackground = hasBackground && cleanLogoLuma(background.R, background.G, background.B) < 160
	}
	var hueGroups []float64
	lightNeutralForegroundRatio := 0.0
	brightNeutralForegroundRatio := 0.0
	midNeutralForegroundRatio := 0.0
	darkNeutralForegroundRatio := 0.0
	chromaticForegroundRatio := 0.0
	chromaticMinLuma := math.MaxFloat64
	chromaticMaxLuma := 0.0
	lightChromaticAccentRatio := 0.0
	largeLightChromaticAccentRatio := 0.0
	paleTintedAccentRatio := 0.0
	chromaticLargeLumaBuckets := map[int]struct{}{}
	for _, item := range colors {
		ratio := float64(item.Count) / float64(total)
		if hasBackground && cleanLogoColorDistance(item.R, item.G, item.B, background.R, background.G, background.B) <= 16 {
			continue
		}
		hue, sat := cleanLogoHueAndSaturation(item.R, item.G, item.B)
		if sat < 0.18 {
			if ratio >= 0.003 && sat >= 0.08 && cleanLogoLuma(item.R, item.G, item.B) >= 180 && cleanLogoLuma(item.R, item.G, item.B) <= 240 && cleanLogoChannelSpread(item.R, item.G, item.B) > 30 {
				paleTintedAccentRatio += ratio
			}
			if ratio >= 0.001 && cleanLogoIsLightNeutralForeground(item.R, item.G, item.B) {
				lightNeutralForegroundRatio += ratio
				if cleanLogoLuma(item.R, item.G, item.B) >= 235 {
					brightNeutralForegroundRatio += ratio
				}
			}
			if ratio >= 0.001 && cleanLogoIsDarkNeutralForeground(item.R, item.G, item.B) {
				darkNeutralForegroundRatio += ratio
			}
			if ratio >= 0.001 && cleanLogoIsMidNeutralForeground(item.R, item.G, item.B) {
				midNeutralForegroundRatio += ratio
			}
			continue
		}
		luma := cleanLogoLuma(item.R, item.G, item.B)
		if ratio >= 0.003 && luma >= 135 {
			lightChromaticAccentRatio += ratio
		}
		if ratio >= 0.015 && luma >= 150 {
			largeLightChromaticAccentRatio += ratio
		}
		if ratio < 0.015 {
			continue
		}
		chromaticForegroundRatio += ratio
		chromaticMinLuma = math.Min(chromaticMinLuma, luma)
		chromaticMaxLuma = math.Max(chromaticMaxLuma, luma)
		if ratio >= 0.035 {
			chromaticLargeLumaBuckets[int(luma/32)] = struct{}{}
		}
		merged := false
		for index, groupHue := range hueGroups {
			if cleanLogoHueDistance(hue, groupHue) <= 18 {
				hueGroups[index] = (groupHue + hue) / 2
				merged = true
				break
			}
		}
		if !merged {
			hueGroups = append(hueGroups, hue)
		}
	}
	switch {
	case len(hueGroups) >= 3:
		return 12
	case hasDarkBackground && len(hueGroups) <= 2 && lightNeutralForegroundRatio > 0 && lightNeutralForegroundRatio <= 0.08:
		return 32
	case len(hueGroups) > 0 && (brightNeutralForegroundRatio >= 0.006 || lightNeutralForegroundRatio >= 0.03):
		return 8
	case len(hueGroups) > 0 && midNeutralForegroundRatio >= 0.004 && midNeutralForegroundRatio <= 0.018:
		return 12
	case len(hueGroups) > 0 && darkNeutralForegroundRatio >= 0.004 && darkNeutralForegroundRatio <= 0.012:
		return 12
	case len(hueGroups) > 0 && darkNeutralForegroundRatio >= 0.004:
		return 8
	case len(hueGroups) == 2:
		return 8
	case len(hueGroups) == 1 && paleTintedAccentRatio >= 0.025 && chromaticForegroundRatio >= 0.03:
		return 16
	case len(hueGroups) == 1 && chromaticForegroundRatio >= 0.12 && lightChromaticAccentRatio >= 0.03:
		return 16
	case len(hueGroups) == 1 && chromaticForegroundRatio >= 0.10 && largeLightChromaticAccentRatio >= 0.02 && chromaticMaxLuma-chromaticMinLuma >= 70:
		return 16
	case len(hueGroups) == 1 && len(chromaticLargeLumaBuckets) >= 2 && chromaticForegroundRatio >= 0.12 && chromaticMaxLuma-chromaticMinLuma >= 55:
		return 8
	default:
		return 3
	}
}

func cleanLogoDominantHistogramColor(colors []cleanLogoHistogramColor) (cleanLogoHistogramColor, bool) {
	if len(colors) == 0 {
		return cleanLogoHistogramColor{}, false
	}
	background := colors[0]
	for _, item := range colors[1:] {
		if item.Count > background.Count {
			background = item
		}
	}
	return background, true
}

func cleanLogoIsLightNeutralForeground(r uint8, g uint8, b uint8) bool {
	if cleanLogoLuma(r, g, b) < 180 || cleanLogoSaturation(r, g, b) > 0.12 {
		return false
	}
	return cleanLogoChannelSpread(r, g, b) <= 30
}

func cleanLogoIsDarkNeutralForeground(r uint8, g uint8, b uint8) bool {
	if cleanLogoLuma(r, g, b) > 95 || cleanLogoSaturation(r, g, b) > 0.12 {
		return false
	}
	return cleanLogoChannelSpread(r, g, b) <= 30
}

func cleanLogoIsMidNeutralForeground(r uint8, g uint8, b uint8) bool {
	value := cleanLogoLuma(r, g, b)
	if value <= 95 || value >= 180 || cleanLogoSaturation(r, g, b) > 0.12 {
		return false
	}
	return cleanLogoChannelSpread(r, g, b) <= 30
}

func cleanLogoReadSourceCornerColor(ctx context.Context, magickPath string, sourcePath string) (cleanLogoHistogramColor, bool) {
	cmd := exec.CommandContext(ctx, magickPath, sourcePath, "-background", "white", "-alpha", "remove", "-depth", "8", "-format", "%[hex:p{0,0}]", "info:-")
	output, err := cmd.Output()
	if err != nil {
		return cleanLogoHistogramColor{}, false
	}
	value := strings.TrimSpace(string(output))
	if len(value) < 6 {
		return cleanLogoHistogramColor{}, false
	}
	r, err := strconv.ParseUint(value[0:2], 16, 8)
	if err != nil {
		return cleanLogoHistogramColor{}, false
	}
	g, err := strconv.ParseUint(value[2:4], 16, 8)
	if err != nil {
		return cleanLogoHistogramColor{}, false
	}
	b, err := strconv.ParseUint(value[4:6], 16, 8)
	if err != nil {
		return cleanLogoHistogramColor{}, false
	}
	return cleanLogoHistogramColor{R: uint8(r), G: uint8(g), B: uint8(b)}, true
}

func cleanLogoChannelSpread(r uint8, g uint8, b uint8) uint8 {
	minValue := r
	if g < minValue {
		minValue = g
	}
	if b < minValue {
		minValue = b
	}
	maxValue := r
	if g > maxValue {
		maxValue = g
	}
	if b > maxValue {
		maxValue = b
	}
	return maxValue - minValue
}

func cleanLogoReadHistogram(ctx context.Context, magickPath string, sourcePath string) ([]cleanLogoHistogramColor, error) {
	cmd := exec.CommandContext(ctx, magickPath, sourcePath, "-background", "white", "-alpha", "remove", "-resize", "512x512>", "-depth", "8", "-colors", "32", "-format", "%c", "histogram:info:-")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(output), "\n")
	result := make([]cleanLogoHistogramColor, 0, len(lines))
	for _, line := range lines {
		match := cleanLogoHistogramPattern.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		count, err := strconv.Atoi(match[1])
		if err != nil {
			continue
		}
		r, ok := cleanLogoParseHistogramChannel(match[2])
		if !ok {
			continue
		}
		g, ok := cleanLogoParseHistogramChannel(match[3])
		if !ok {
			continue
		}
		b, ok := cleanLogoParseHistogramChannel(match[4])
		if !ok {
			continue
		}
		result = append(result, cleanLogoHistogramColor{Count: count, R: r, G: g, B: b})
	}
	return result, nil
}

func cleanLogoParseHistogramChannel(value string) (uint8, bool) {
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return 0, false
	}
	parsed = math.Round(parsed)
	if parsed < 0 {
		parsed = 0
	}
	if parsed > 255 {
		parsed = 255
	}
	return uint8(parsed), true
}

func cleanLogoNormalizePNG(ctx context.Context, magickPath string, inputPath string, outputPath string, longEdge int) error {
	if longEdge <= 0 {
		longEdge = cleanLogoLongEdge
	}
	args := []string{
		inputPath,
		"-background", "white",
		"-alpha", "remove",
		"-resize", fmt.Sprintf("%dx%d", longEdge, longEdge),
		"-colorspace", "sRGB",
		"-strip",
		outputPath,
	}
	return cleanLogoRunCommand(ctx, magickPath, args...)
}

func cleanLogoQuantizePNG(ctx context.Context, magickPath string, inputPath string, outputPath string, colors int) error {
	args := []string{
		inputPath,
		"-dither", "None",
		"-colors", fmt.Sprintf("%d", colors),
		"-type", "TrueColor",
		"-strip",
		outputPath,
	}
	return cleanLogoRunCommand(ctx, magickPath, args...)
}

func cleanLogoRestoreLightNeutralAccents(normalizedPath string, quantizedPath string) error {
	normalized, width, height, err := cleanLogoReadPNG(normalizedPath)
	if err != nil {
		return err
	}
	quantized, quantizedWidth, quantizedHeight, err := cleanLogoReadPNG(quantizedPath)
	if err != nil {
		return err
	}
	if width != quantizedWidth || height != quantizedHeight {
		return nil
	}
	background := cleanLogoRGBColorFromImage(normalized.At(normalized.Bounds().Min.X, normalized.Bounds().Min.Y))
	restoreLight := cleanLogoLuma(background.R, background.G, background.B) < 160 && cleanLogoLightNeutralCoverage(normalized) <= 0.08
	darkCoverage := cleanLogoDarkNeutralAccentCoverage(normalized)
	restoreDark := cleanLogoLuma(background.R, background.G, background.B) >= 160 && darkCoverage > 0 && darkCoverage <= 0.08
	midCoverage := cleanLogoMidNeutralAccentCoverage(normalized)
	restoreMid := cleanLogoLuma(background.R, background.G, background.B) >= 160 && midCoverage > 0 && midCoverage <= 0.08
	if !restoreLight && !restoreDark && !restoreMid {
		return nil
	}
	darkAccent := cleanLogoDarkestDarkNeutralAccent(normalized)
	midAccent := cleanLogoDarkestMidNeutralAccent(normalized)
	out := image.NewRGBA(image.Rect(0, 0, width, height))
	normalizedBounds := normalized.Bounds()
	quantizedBounds := quantized.Bounds()
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			q := quantized.At(quantizedBounds.Min.X+x, quantizedBounds.Min.Y+y)
			source := cleanLogoRGBColorFromImage(normalized.At(normalizedBounds.Min.X+x, normalizedBounds.Min.Y+y))
			if restoreLight && cleanLogoIsLightNeutralForeground(source.R, source.G, source.B) && cleanLogoLuma(source.R, source.G, source.B) >= 160 {
				out.Set(x, y, color.RGBA{R: source.R, G: source.G, B: source.B, A: 0xff})
				continue
			}
			if restoreDark && cleanLogoIsDarkNeutralAccent(source.R, source.G, source.B) {
				out.Set(x, y, color.RGBA{R: darkAccent.R, G: darkAccent.G, B: darkAccent.B, A: 0xff})
				continue
			}
			if restoreMid && cleanLogoIsMidNeutralForeground(source.R, source.G, source.B) {
				out.Set(x, y, color.RGBA{R: midAccent.R, G: midAccent.G, B: midAccent.B, A: 0xff})
				continue
			}
			out.Set(x, y, q)
		}
	}
	file, err := os.Create(quantizedPath)
	if err != nil {
		return err
	}
	defer file.Close()
	return png.Encode(file, out)
}

func cleanLogoLightNeutralCoverage(img image.Image) float64 {
	bounds := img.Bounds()
	total := bounds.Dx() * bounds.Dy()
	if total == 0 {
		return 0
	}
	count := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			source := cleanLogoRGBColorFromImage(img.At(x, y))
			if cleanLogoIsLightNeutralForeground(source.R, source.G, source.B) && cleanLogoLuma(source.R, source.G, source.B) >= 160 {
				count++
			}
		}
	}
	return float64(count) / float64(total)
}

func cleanLogoDarkNeutralAccentCoverage(img image.Image) float64 {
	bounds := img.Bounds()
	total := bounds.Dx() * bounds.Dy()
	if total == 0 {
		return 0
	}
	count := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			source := cleanLogoRGBColorFromImage(img.At(x, y))
			if cleanLogoIsDarkNeutralAccent(source.R, source.G, source.B) {
				count++
			}
		}
	}
	return float64(count) / float64(total)
}

func cleanLogoMidNeutralAccentCoverage(img image.Image) float64 {
	bounds := img.Bounds()
	total := bounds.Dx() * bounds.Dy()
	if total == 0 {
		return 0
	}
	count := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			source := cleanLogoRGBColorFromImage(img.At(x, y))
			if cleanLogoIsMidNeutralForeground(source.R, source.G, source.B) {
				count++
			}
		}
	}
	return float64(count) / float64(total)
}

func cleanLogoDarkestDarkNeutralAccent(img image.Image) cleanLogoHistogramColor {
	bounds := img.Bounds()
	best := cleanLogoHistogramColor{R: 20, G: 20, B: 20}
	bestLuma := math.MaxFloat64
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			source := cleanLogoRGBColorFromImage(img.At(x, y))
			if !cleanLogoIsDarkNeutralAccent(source.R, source.G, source.B) {
				continue
			}
			value := cleanLogoLuma(source.R, source.G, source.B)
			if value < bestLuma {
				best = source
				bestLuma = value
			}
		}
	}
	if cleanLogoLuma(best.R, best.G, best.B) < 16 {
		return cleanLogoHistogramColor{R: 20, G: 20, B: 20}
	}
	return best
}

func cleanLogoDarkestMidNeutralAccent(img image.Image) cleanLogoHistogramColor {
	bounds := img.Bounds()
	best := cleanLogoHistogramColor{R: 119, G: 119, B: 119}
	bestLuma := math.MaxFloat64
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			source := cleanLogoRGBColorFromImage(img.At(x, y))
			if !cleanLogoIsMidNeutralForeground(source.R, source.G, source.B) {
				continue
			}
			value := cleanLogoLuma(source.R, source.G, source.B)
			if value < bestLuma {
				best = source
				bestLuma = value
			}
		}
	}
	return best
}

func cleanLogoIsDarkNeutralAccent(r uint8, g uint8, b uint8) bool {
	if cleanLogoLuma(r, g, b) > 125 || cleanLogoSaturation(r, g, b) > 0.12 {
		return false
	}
	return cleanLogoChannelSpread(r, g, b) <= 30
}

func cleanLogoReadPNG(path string) (image.Image, int, int, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, 0, 0, err
	}
	defer file.Close()
	img, err := png.Decode(file)
	if err != nil {
		return nil, 0, 0, err
	}
	bounds := img.Bounds()
	return img, bounds.Dx(), bounds.Dy(), nil
}

func cleanLogoReadSourceBorderBackground(path string) (cleanLogoHistogramColor, bool) {
	file, err := os.Open(path)
	if err != nil {
		return cleanLogoHistogramColor{}, false
	}
	defer file.Close()
	img, _, err := image.Decode(file)
	if err != nil {
		return cleanLogoHistogramColor{}, false
	}
	bounds := img.Bounds()
	counts := make(map[uint32]int)
	add := func(x int, y int) {
		r, g, b := cleanLogoRGB8OnWhite(img.At(x, y))
		counts[cleanLogoRGBKey(r, g, b)]++
	}
	for x := bounds.Min.X; x < bounds.Max.X; x++ {
		add(x, bounds.Min.Y)
		add(x, bounds.Max.Y-1)
	}
	for y := bounds.Min.Y + 1; y < bounds.Max.Y-1; y++ {
		add(bounds.Min.X, y)
		add(bounds.Max.X-1, y)
	}
	bestKey := cleanLogoRGBKey(255, 255, 255)
	bestCount := -1
	for key, count := range counts {
		if count > bestCount {
			bestKey = key
			bestCount = count
		}
	}
	return cleanLogoHistogramColor{
		Count: bestCount,
		R:     uint8(bestKey >> 16),
		G:     uint8(bestKey >> 8),
		B:     uint8(bestKey),
	}, bestCount >= 0
}

func cleanLogoRGBColorFromImage(c color.Color) cleanLogoHistogramColor {
	r, g, b := cleanLogoRGB8(c)
	return cleanLogoHistogramColor{R: r, G: g, B: b}
}

func cleanLogoReadPalette(img image.Image) []cleanLogoPaletteColor {
	bounds := img.Bounds()
	counts := make(map[uint32]int)
	values := make(map[uint32]cleanLogoPaletteColor)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			key := cleanLogoColorKey(img.At(x, y))
			counts[key]++
			if _, ok := values[key]; !ok {
				r, g, b := cleanLogoRGB8(img.At(x, y))
				values[key] = cleanLogoPaletteColor{Hex: cleanLogoHex(r, g, b), R: r, G: g, B: b}
			}
		}
	}
	palette := make([]cleanLogoPaletteColor, 0, len(counts))
	for key, count := range counts {
		item := values[key]
		item.Count = count
		palette = append(palette, item)
	}
	sort.SliceStable(palette, func(i, j int) bool {
		iSat := cleanLogoSaturation(palette[i].R, palette[i].G, palette[i].B)
		jSat := cleanLogoSaturation(palette[j].R, palette[j].G, palette[j].B)
		if math.Abs(iSat-jSat) > 0.08 {
			return iSat > jSat
		}
		return palette[i].Count > palette[j].Count
	})
	return palette
}

func cleanLogoDetectBackground(img image.Image, palette []cleanLogoPaletteColor) cleanLogoTraceLayer {
	return cleanLogoDetectBackgroundWithOptions(img, palette, cleanLogoVectorizeOptionsPreset())
}

func cleanLogoDetectBackgroundWithOptions(img image.Image, palette []cleanLogoPaletteColor, options cleanLogoVectorizeOptions) cleanLogoTraceLayer {
	bounds := img.Bounds()
	borderCounts := make(map[uint32]int)
	countBorder := func(x int, y int) {
		borderCounts[cleanLogoColorKey(img.At(bounds.Min.X+x, bounds.Min.Y+y))]++
	}
	width := bounds.Dx()
	height := bounds.Dy()
	for x := 0; x < width; x++ {
		countBorder(x, 0)
		countBorder(x, height-1)
	}
	for y := 1; y < height-1; y++ {
		countBorder(0, y)
		countBorder(width-1, y)
	}
	items := make(map[uint32]cleanLogoPaletteColor, len(palette))
	for _, item := range palette {
		items[cleanLogoRGBKey(item.R, item.G, item.B)] = item
	}
	backgroundKey := uint32(0)
	backgroundBorderCount := -1
	for key, count := range borderCounts {
		if count > backgroundBorderCount {
			backgroundKey = key
			backgroundBorderCount = count
		}
	}
	background, ok := items[backgroundKey]
	if !ok {
		background = cleanLogoPaletteColor{Hex: "#FFFFFF", R: 255, G: 255, B: 255}
	}
	layer := cleanLogoTraceLayer{
		Hex:   background.Hex,
		R:     background.R,
		G:     background.G,
		B:     background.B,
		Count: background.Count,
		Keys:  map[uint32]struct{}{cleanLogoRGBKey(background.R, background.G, background.B): {}},
	}
	layer.Keys[backgroundKey] = struct{}{}
	for _, item := range palette {
		key := cleanLogoRGBKey(item.R, item.G, item.B)
		if key == backgroundKey {
			continue
		}
		if cleanLogoIsBackgroundCompanionWithOptions(item, layer, borderCounts[key], options) {
			layer.Keys[key] = struct{}{}
			layer.Count += item.Count
		}
	}
	return layer
}

func cleanLogoBuildTraceLayers(palette []cleanLogoPaletteColor, totalPixels int, backgroundKeys map[uint32]struct{}) []cleanLogoTraceLayer {
	return cleanLogoBuildTraceLayersWithOptions(palette, totalPixels, backgroundKeys, cleanLogoVectorizeOptionsPreset())
}

func cleanLogoBuildTraceLayersWithOptions(palette []cleanLogoPaletteColor, totalPixels int, backgroundKeys map[uint32]struct{}, options cleanLogoVectorizeOptions) []cleanLogoTraceLayer {
	minPixels := int(math.Max(16, float64(totalPixels)*0.00002))
	layers := make([]cleanLogoTraceLayer, 0, len(palette))
	for _, item := range palette {
		if item.Count < minPixels {
			continue
		}
		if _, ok := backgroundKeys[cleanLogoRGBKey(item.R, item.G, item.B)]; ok {
			continue
		}
		if !options.PreservePaletteLayers && cleanLogoIsNeutralNoise(item, totalPixels) {
			continue
		}
		if options.PreservePaletteLayers {
			layers = append(layers, cleanLogoTraceLayer{
				Hex:   item.Hex,
				R:     item.R,
				G:     item.G,
				B:     item.B,
				Count: item.Count,
				Keys:  map[uint32]struct{}{cleanLogoRGBKey(item.R, item.G, item.B): {}},
			})
			continue
		}
		merged := false
		for index := range layers {
			if cleanLogoIsLightNeutralForeground(item.R, item.G, item.B) && cleanLogoLuma(item.R, item.G, item.B)-cleanLogoLuma(layers[index].R, layers[index].G, layers[index].B) > 40 {
				continue
			}
			if cleanLogoIsMidNeutralForeground(item.R, item.G, item.B) != cleanLogoIsMidNeutralForeground(layers[index].R, layers[index].G, layers[index].B) {
				continue
			}
			if !cleanLogoCanMergeColorWithOptions(item, layers[index], options) {
				continue
			}
			layer := &layers[index]
			layer.Keys[cleanLogoRGBKey(item.R, item.G, item.B)] = struct{}{}
			layer.Count += item.Count
			if cleanLogoShouldUseMergedLayerColor(item.R, item.G, item.B, layer.R, layer.G, layer.B) || item.Count > layer.Count-item.Count {
				layer.Hex = item.Hex
				layer.R = item.R
				layer.G = item.G
				layer.B = item.B
			}
			merged = true
			break
		}
		if !merged {
			layers = append(layers, cleanLogoTraceLayer{
				Hex:   item.Hex,
				R:     item.R,
				G:     item.G,
				B:     item.B,
				Count: item.Count,
				Keys:  map[uint32]struct{}{cleanLogoRGBKey(item.R, item.G, item.B): {}},
			})
		}
	}
	if !options.PreservePaletteLayers {
		layers = cleanLogoMergeTintTraceLayersWithOptions(layers, options)
	}
	return layers
}

func cleanLogoShouldUseMergedLayerColor(ar uint8, ag uint8, ab uint8, br uint8, bg uint8, bb uint8) bool {
	return cleanLogoIsLightNeutralForeground(ar, ag, ab) &&
		cleanLogoIsLightNeutralForeground(br, bg, bb) &&
		cleanLogoLuma(ar, ag, ab) > cleanLogoLuma(br, bg, bb)
}

func cleanLogoMergeTintTraceLayers(layers []cleanLogoTraceLayer) []cleanLogoTraceLayer {
	return cleanLogoMergeTintTraceLayersWithOptions(layers, cleanLogoVectorizeOptionsPreset())
}

func cleanLogoMergeTintTraceLayersWithOptions(layers []cleanLogoTraceLayer, options cleanLogoVectorizeOptions) []cleanLogoTraceLayer {
	merged := make([]bool, len(layers))
	for sourceIndex := range layers {
		source := layers[sourceIndex]
		sourceSat := cleanLogoSaturation(source.R, source.G, source.B)
		if cleanLogoLuma(source.R, source.G, source.B) < 180 && sourceSat >= 0.12 {
			continue
		}
		targetIndex := -1
		targetScore := math.MaxFloat64
		for index := range layers {
			if index == sourceIndex || merged[index] {
				continue
			}
			target := layers[index]
			targetSat := cleanLogoSaturation(target.R, target.G, target.B)
			if targetSat <= sourceSat || cleanLogoLuma(target.R, target.G, target.B) > cleanLogoLuma(source.R, source.G, source.B) {
				continue
			}
			if cleanLogoIsLightNeutralForeground(source.R, source.G, source.B) && cleanLogoLuma(source.R, source.G, source.B)-cleanLogoLuma(target.R, target.G, target.B) > 40 {
				continue
			}
			if cleanLogoLuma(target.R, target.G, target.B) < 140 {
				continue
			}
			if !cleanLogoSameFunctionalHue(source.R, source.G, source.B, target.R, target.G, target.B, options.MergeHueDistance*2.5, math.Min(70, options.MergeLightness)) {
				continue
			}
			score := cleanLogoHueGap(source.R, source.G, source.B, target.R, target.G, target.B) + math.Abs(cleanLogoLuma(source.R, source.G, source.B)-cleanLogoLuma(target.R, target.G, target.B))*0.05
			if score < targetScore {
				targetScore = score
				targetIndex = index
			}
		}
		if targetIndex < 0 {
			continue
		}
		for key := range source.Keys {
			layers[targetIndex].Keys[key] = struct{}{}
		}
		layers[targetIndex].Count += source.Count
		merged[sourceIndex] = true
	}
	result := make([]cleanLogoTraceLayer, 0, len(layers))
	for index, layer := range layers {
		if !merged[index] {
			result = append(result, layer)
		}
	}
	return result
}

func cleanLogoWriteLayerMask(img image.Image, target cleanLogoTraceLayer, outputPath string) error {
	bounds := img.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	for y := 0; y < bounds.Dy(); y++ {
		for x := 0; x < bounds.Dx(); x++ {
			if _, ok := target.Keys[cleanLogoColorKey(img.At(bounds.Min.X+x, bounds.Min.Y+y))]; ok {
				out.Set(x, y, color.Black)
			} else {
				out.Set(x, y, color.White)
			}
		}
	}
	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()
	return png.Encode(file, out)
}

func cleanLogoMinComponentAreaForLayer(layer cleanLogoTraceLayer, totalPixels int, base int) int {
	return cleanLogoMinComponentAreaForLayerWithOptions(layer, totalPixels, base, cleanLogoVectorizeOptionsPreset())
}

func cleanLogoMinComponentAreaForLayerWithOptions(layer cleanLogoTraceLayer, totalPixels int, base int, options cleanLogoVectorizeOptions) int {
	if cleanLogoIsLightNeutralForeground(layer.R, layer.G, layer.B) {
		return base
	}
	if cleanLogoSaturation(layer.R, layer.G, layer.B) >= 0.12 {
		return base
	}
	if cleanLogoLuma(layer.R, layer.G, layer.B) < 180 {
		return base
	}
	layerArea := int(float64(totalPixels) * options.LightMinAreaRatio)
	if layerArea > base {
		return layerArea
	}
	return base
}

func cleanLogoCleanMask(path string, minComponentArea int, maxHoleArea int) (cleanLogoStats, error) {
	return cleanLogoCleanMaskWithOptions(path, true, true, minComponentArea, maxHoleArea)
}

func cleanLogoCleanMaskWithOptions(path string, removeSpeckles bool, fillSmallHoles bool, minComponentArea int, maxHoleArea int) (cleanLogoStats, error) {
	file, err := os.Open(path)
	if err != nil {
		return cleanLogoStats{}, err
	}
	img, err := png.Decode(file)
	_ = file.Close()
	if err != nil {
		return cleanLogoStats{}, err
	}
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	black := make([]bool, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r, g, b := cleanLogoRGB8(img.At(bounds.Min.X+x, bounds.Min.Y+y))
			black[y*width+x] = int(r)+int(g)+int(b) < 384
		}
	}
	stats := cleanLogoStats{}
	if removeSpeckles {
		stats = cleanLogoRemoveSmallComponents(black, width, height, minComponentArea)
	}
	if fillSmallHoles {
		holeStats := cleanLogoFillSmallHoles(black, width, height, maxHoleArea)
		stats.HolesFilled = holeStats.HolesFilled
		stats.HolePixelsFilled = holeStats.HolePixelsFilled
	}
	out := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if black[y*width+x] {
				out.Set(x, y, color.Black)
				stats.RemainingBlack++
			} else {
				out.Set(x, y, color.White)
			}
		}
	}
	output, err := os.Create(path)
	if err != nil {
		return cleanLogoStats{}, err
	}
	defer output.Close()
	return stats, png.Encode(output, out)
}

func cleanLogoRemoveSmallComponents(black []bool, width int, height int, minArea int) cleanLogoStats {
	visited := make([]bool, len(black))
	queue := make([]int, 0, 256)
	component := make([]int, 0, 256)
	stats := cleanLogoStats{}
	for index := range black {
		if !black[index] || visited[index] {
			continue
		}
		queue = append(queue[:0], index)
		component = append(component[:0], index)
		visited[index] = true
		for len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]
			x := current % width
			y := current / width
			for _, next := range cleanLogoNeighbors(x, y, width, height) {
				if visited[next] || !black[next] {
					continue
				}
				visited[next] = true
				queue = append(queue, next)
				component = append(component, next)
			}
		}
		if len(component) < minArea {
			stats.ComponentsRemoved++
			stats.PixelsRemoved += len(component)
			for _, pixel := range component {
				black[pixel] = false
			}
		}
	}
	return stats
}

func cleanLogoFillSmallHoles(black []bool, width int, height int, maxArea int) cleanLogoStats {
	visited := make([]bool, len(black))
	queue := make([]int, 0, 256)
	component := make([]int, 0, 256)
	stats := cleanLogoStats{}
	for index := range black {
		if black[index] || visited[index] {
			continue
		}
		queue = append(queue[:0], index)
		component = append(component[:0], index)
		visited[index] = true
		touchesBorder := false
		for len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]
			x := current % width
			y := current / width
			if x == 0 || y == 0 || x == width-1 || y == height-1 {
				touchesBorder = true
			}
			for _, next := range cleanLogoNeighbors(x, y, width, height) {
				if visited[next] || black[next] {
					continue
				}
				visited[next] = true
				queue = append(queue, next)
				component = append(component, next)
			}
		}
		if !touchesBorder && len(component) <= maxArea {
			stats.HolesFilled++
			stats.HolePixelsFilled += len(component)
			for _, pixel := range component {
				black[pixel] = true
			}
		}
	}
	return stats
}

func cleanLogoNeighbors(x int, y int, width int, height int) []int {
	result := make([]int, 0, 4)
	if x > 0 {
		result = append(result, y*width+x-1)
	}
	if x < width-1 {
		result = append(result, y*width+x+1)
	}
	if y > 0 {
		result = append(result, (y-1)*width+x)
	}
	if y < height-1 {
		result = append(result, (y+1)*width+x)
	}
	return result
}

func cleanLogoRunPotrace(ctx context.Context, magickPath string, potracePath string, maskPath string, outputPath string, fill string) error {
	pbmPath := strings.TrimSuffix(maskPath, filepath.Ext(maskPath)) + ".pbm"
	if err := cleanLogoRunCommand(ctx, magickPath, maskPath, "-threshold", "50%", pbmPath); err != nil {
		return err
	}
	return cleanLogoRunCommand(ctx, potracePath, pbmPath, "--svg", "--flat", "--color", fill, "--output", outputPath)
}

func cleanLogoExtractPotraceGroup(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	match := cleanLogoGroupPattern.Find(data)
	if len(match) == 0 {
		return "", errors.New("potrace svg group not found")
	}
	return string(match), nil
}

func cleanLogoRunCommand(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return vectorizeCommandError(filepath.Base(name), strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return nil
}

func vectorizeCommandError(tool string, stage string, err error, output string) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return safeMessageError{message: "转 SVG 超时，请降低图片尺寸或改用 Logo 模式后重试"}
	}
	message := strings.TrimSpace(output)
	errText := strings.ToLower(err.Error() + " " + message)
	if strings.Contains(errText, "signal: killed") || strings.Contains(errText, "killed") || strings.Contains(errText, "cannot allocate memory") {
		return safeMessageError{message: "转 SVG 失败：图片过大或服务器内存不足，请降低图片尺寸后重试"}
	}
	if message != "" {
		return fmt.Errorf("%s failed at %s: %w: %s", tool, stage, err, message)
	}
	return fmt.Errorf("%s failed at %s: %w", tool, stage, err)
}

func cleanLogoIsBackgroundCompanion(item cleanLogoPaletteColor, background cleanLogoTraceLayer, borderCount int) bool {
	return cleanLogoIsBackgroundCompanionWithOptions(item, background, borderCount, cleanLogoVectorizeOptionsPreset())
}

func cleanLogoIsBackgroundCompanionWithOptions(item cleanLogoPaletteColor, background cleanLogoTraceLayer, borderCount int, options cleanLogoVectorizeOptions) bool {
	itemIsWhite := cleanLogoIsBackground(item.R, item.G, item.B)
	backgroundIsWhite := cleanLogoIsBackground(background.R, background.G, background.B)
	if itemIsWhite {
		return backgroundIsWhite && borderCount > 0
	}
	if borderCount <= 0 && backgroundIsWhite {
		return false
	}
	if cleanLogoLuma(background.R, background.G, background.B) >= 180 &&
		cleanLogoLuma(item.R, item.G, item.B) < 140 &&
		cleanLogoLuma(background.R, background.G, background.B)-cleanLogoLuma(item.R, item.G, item.B) > 60 {
		return false
	}
	if background.Count > 0 &&
		float64(item.Count)/float64(background.Count) >= 0.015 &&
		cleanLogoColorDistance(item.R, item.G, item.B, background.R, background.G, background.B) > 24 {
		return false
	}
	if borderCount > 0 && cleanLogoCanMergeRaw(item.R, item.G, item.B, background.R, background.G, background.B, options.MergeDistance, options.MergeHueDistance, options.MergeLightness*1.4, options.MergeSaturation) {
		return true
	}
	if borderCount <= 0 {
		lumaGap := math.Abs(cleanLogoLuma(item.R, item.G, item.B) - cleanLogoLuma(background.R, background.G, background.B))
		if lumaGap <= 32 && cleanLogoColorDistance(item.R, item.G, item.B, background.R, background.G, background.B) <= options.MergeDistance*0.8 {
			return true
		}
		return false
	}
	if cleanLogoSaturation(background.R, background.G, background.B) < 0.12 {
		return false
	}
	if math.Abs(cleanLogoSaturation(item.R, item.G, item.B)-cleanLogoSaturation(background.R, background.G, background.B)) > options.MergeSaturation {
		return false
	}
	return cleanLogoSameFunctionalHue(item.R, item.G, item.B, background.R, background.G, background.B, options.MergeHueDistance*2, options.MergeLightness*1.4)
}

func cleanLogoCanMergeColor(item cleanLogoPaletteColor, layer cleanLogoTraceLayer) bool {
	return cleanLogoCanMergeColorWithOptions(item, layer, cleanLogoVectorizeOptionsPreset())
}

func cleanLogoCanMergeColorWithOptions(item cleanLogoPaletteColor, layer cleanLogoTraceLayer, options cleanLogoVectorizeOptions) bool {
	return cleanLogoCanMergeRaw(item.R, item.G, item.B, layer.R, layer.G, layer.B, options.MergeDistance, options.MergeHueDistance, options.MergeLightness, options.MergeSaturation)
}

func cleanLogoCanMergeRaw(ar uint8, ag uint8, ab uint8, br uint8, bg uint8, bb uint8, distanceLimit float64, hueLimit float64, lightLimit float64, satLimit float64) bool {
	aHue, aSat := cleanLogoHueAndSaturation(ar, ag, ab)
	bHue, bSat := cleanLogoHueAndSaturation(br, bg, bb)
	aLuma := cleanLogoLuma(ar, ag, ab)
	bLuma := cleanLogoLuma(br, bg, bb)
	if aSat < 0.12 || bSat < 0.12 {
		return cleanLogoColorDistance(ar, ag, ab, br, bg, bb) <= distanceLimit || cleanLogoCanMergeTintColor(ar, ag, ab, br, bg, bb, hueLimit, lightLimit)
	}
	if cleanLogoHueDistance(aHue, bHue) > hueLimit {
		return false
	}
	if bLuma < 120 && aLuma-bLuma > 35 {
		return false
	}
	if bLuma >= 140 && aLuma < 120 {
		return false
	}
	if math.Abs(aSat-bSat) > satLimit {
		return false
	}
	if math.Abs(aLuma-bLuma) > lightLimit {
		return false
	}
	return true
}

func cleanLogoCanMergeTintColor(ar uint8, ag uint8, ab uint8, br uint8, bg uint8, bb uint8, hueLimit float64, lightLimit float64) bool {
	aSat := cleanLogoSaturation(ar, ag, ab)
	bSat := cleanLogoSaturation(br, bg, bb)
	if aSat < 0.03 && bSat < 0.03 {
		return false
	}
	if cleanLogoSameFunctionalHue(ar, ag, ab, br, bg, bb, hueLimit*2, lightLimit*1.8) {
		return true
	}
	return cleanLogoSameChannelBias(ar, ag, ab, br, bg, bb) && math.Abs(cleanLogoLuma(ar, ag, ab)-cleanLogoLuma(br, bg, bb)) <= lightLimit*2.2
}

func cleanLogoSameFunctionalHue(ar uint8, ag uint8, ab uint8, br uint8, bg uint8, bb uint8, hueLimit float64, lightLimit float64) bool {
	return cleanLogoHueGap(ar, ag, ab, br, bg, bb) <= hueLimit && math.Abs(cleanLogoLuma(ar, ag, ab)-cleanLogoLuma(br, bg, bb)) <= lightLimit
}

func cleanLogoSameChannelBias(ar uint8, ag uint8, ab uint8, br uint8, bg uint8, bb uint8) bool {
	return cleanLogoChannelOrder(ar, ag, ab) == cleanLogoChannelOrder(br, bg, bb)
}

func cleanLogoChannelOrder(r uint8, g uint8, b uint8) string {
	type channel struct {
		name  string
		value uint8
	}
	values := []channel{{"r", r}, {"g", g}, {"b", b}}
	sort.Slice(values, func(i, j int) bool {
		return values[i].value > values[j].value
	})
	return values[0].name + values[1].name + values[2].name
}

func cleanLogoColorDistance(ar uint8, ag uint8, ab uint8, br uint8, bg uint8, bb uint8) float64 {
	return math.Sqrt(math.Pow(float64(ar)-float64(br), 2) + math.Pow(float64(ag)-float64(bg), 2) + math.Pow(float64(ab)-float64(bb), 2))
}

func cleanLogoIsNeutralNoise(item cleanLogoPaletteColor, totalPixels int) bool {
	if cleanLogoIsLightNeutralForeground(item.R, item.G, item.B) && cleanLogoLuma(item.R, item.G, item.B) >= 220 {
		return false
	}
	if cleanLogoSaturation(item.R, item.G, item.B) > 0.12 {
		return false
	}
	ratio := float64(item.Count) / float64(totalPixels)
	return ratio < 0.01 && cleanLogoLuma(item.R, item.G, item.B) > 200
}

func cleanLogoIsBackground(r uint8, g uint8, b uint8) bool {
	if cleanLogoLuma(r, g, b) > 245 && cleanLogoSaturation(r, g, b) < 0.12 {
		return true
	}
	if cleanLogoLuma(r, g, b) > 232 && cleanLogoSaturation(r, g, b) < 0.08 {
		return true
	}
	return false
}

func cleanLogoHex(r uint8, g uint8, b uint8) string {
	return fmt.Sprintf("#%02X%02X%02X", r, g, b)
}

func cleanLogoColorKey(c color.Color) uint32 {
	r, g, b := cleanLogoRGB8(c)
	return cleanLogoRGBKey(r, g, b)
}

func cleanLogoRGBKey(r uint8, g uint8, b uint8) uint32 {
	return uint32(r)<<16 | uint32(g)<<8 | uint32(b)
}

func cleanLogoRGB8(c color.Color) (uint8, uint8, uint8) {
	r, g, b, _ := c.RGBA()
	return uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)
}

func cleanLogoRGB8OnWhite(c color.Color) (uint8, uint8, uint8) {
	r, g, b, a := c.RGBA()
	alpha := int(a >> 8)
	if alpha >= 255 {
		return uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)
	}
	return uint8(((int(r>>8) * alpha) + 255*(255-alpha)) / 255),
		uint8(((int(g>>8) * alpha) + 255*(255-alpha)) / 255),
		uint8(((int(b>>8) * alpha) + 255*(255-alpha)) / 255)
}

func cleanLogoLuma(r uint8, g uint8, b uint8) float64 {
	return 0.2126*float64(r) + 0.7152*float64(g) + 0.0722*float64(b)
}

func cleanLogoHueAndSaturation(r uint8, g uint8, b uint8) (float64, float64) {
	rf := float64(r) / 255
	gf := float64(g) / 255
	bf := float64(b) / 255
	maxValue := math.Max(rf, math.Max(gf, bf))
	minValue := math.Min(rf, math.Min(gf, bf))
	delta := maxValue - minValue
	if delta == 0 {
		return 0, 0
	}
	saturation := delta / maxValue
	var hue float64
	switch maxValue {
	case rf:
		hue = math.Mod((gf-bf)/delta, 6)
	case gf:
		hue = (bf-rf)/delta + 2
	default:
		hue = (rf-gf)/delta + 4
	}
	hue *= 60
	if hue < 0 {
		hue += 360
	}
	return hue, saturation
}

func cleanLogoSaturation(r uint8, g uint8, b uint8) float64 {
	_, sat := cleanLogoHueAndSaturation(r, g, b)
	return sat
}

func cleanLogoHueDistance(a float64, b float64) float64 {
	diff := math.Abs(a - b)
	if diff > 180 {
		return 360 - diff
	}
	return diff
}

func cleanLogoHueGap(ar uint8, ag uint8, ab uint8, br uint8, bg uint8, bb uint8) float64 {
	ah, _ := cleanLogoHueAndSaturation(ar, ag, ab)
	bh, _ := cleanLogoHueAndSaturation(br, bg, bb)
	return cleanLogoHueDistance(ah, bh)
}
