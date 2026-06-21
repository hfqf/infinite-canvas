package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/basketikun/infinite-canvas/config"
	"github.com/basketikun/infinite-canvas/service"

	_ "image/gif"
	_ "image/jpeg"
)

const (
	defaultPort              = "8091"
	defaultLongEdge          = 4096
	defaultColors            = 16
	defaultMinComponentRatio = 0.00001
	defaultMaxHoleRatio      = 0.00004
	defaultMergeDistance     = 48
	defaultMergeHueDistance  = 16
	defaultMergeLightness    = 80
	defaultMergeSaturation   = 0.55
	defaultLightMinAreaRatio = 0.003
	maxInputBytes            = 60 << 20
)

var (
	svgGroupPattern = regexp.MustCompile(`(?s)<g\b.*</g>`)
	svgPathPattern  = regexp.MustCompile(`<path\b`)
	svgMovePattern  = regexp.MustCompile(`\bM`)
)

type vectorizeRequest struct {
	DataURL           string  `json:"dataUrl"`
	ImageURL          string  `json:"imageUrl"`
	FilePath          string  `json:"filePath"`
	Mode              string  `json:"mode"`
	Colors            int     `json:"colors"`
	LongEdge          int     `json:"longEdge"`
	MinComponentRatio float64 `json:"minComponentRatio"`
	MaxHoleRatio      float64 `json:"maxHoleRatio"`
	MergeDistance     float64 `json:"mergeDistance"`
	MergeHueDistance  float64 `json:"mergeHueDistance"`
	MergeLightness    float64 `json:"mergeLightness"`
	MergeSaturation   float64 `json:"mergeSaturation"`
	LightMinAreaRatio float64 `json:"lightMinAreaRatio"`
	MaskCloseRadius   int     `json:"maskCloseRadius"`
	LightDilateRadius int     `json:"lightDilateRadius"`
	DarkDilateRadius  int     `json:"darkDilateRadius"`
	RemoveSpeckles    *bool   `json:"removeSpeckles"`
	FillSmallHoles    *bool   `json:"fillSmallHoles"`
}

type artifact struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Path string `json:"path"`
}

type jobResult struct {
	ID        string        `json:"id"`
	Width     int           `json:"width"`
	Height    int           `json:"height"`
	SVGURL    string        `json:"svgUrl"`
	Preview   string        `json:"previewUrl"`
	Artifacts []artifact    `json:"artifacts"`
	Metrics   metricsResult `json:"metrics"`
}

type metricsResult struct {
	SourceColors    int            `json:"sourceColors"`
	QuantizedColors int            `json:"quantizedColors"`
	Background      string         `json:"background"`
	Layers          []layerMetrics `json:"layers"`
	SVGBytes        int64          `json:"svgBytes"`
	SVGPaths        int            `json:"svgPaths"`
	SVGSubpaths     int            `json:"svgSubpaths"`
}

type layerMetrics struct {
	Hex                string `json:"hex"`
	Pixels             int    `json:"pixels"`
	MaskURL            string `json:"maskUrl"`
	SVGURL             string `json:"svgUrl"`
	ComponentsRemoved  int    `json:"componentsRemoved"`
	PixelsRemoved      int    `json:"pixelsRemoved"`
	HolesFilled        int    `json:"holesFilled"`
	HolePixelsFilled   int    `json:"holePixelsFilled"`
	RemainingBlackArea int    `json:"remainingBlackArea"`
}

type paletteColor struct {
	Hex   string
	R     uint8
	G     uint8
	B     uint8
	Count int
}

type traceLayer struct {
	Hex   string
	R     uint8
	G     uint8
	B     uint8
	Count int
	Keys  map[uint32]struct{}
}

type cleanStats struct {
	ComponentsRemoved int
	PixelsRemoved     int
	HolesFilled       int
	HolePixelsFilled  int
	RemainingBlack    int
}

func main() {
	if err := config.Load(); err != nil {
		panic(err)
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}
	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/api/health", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	http.HandleFunc("/api/vectorize", handleVectorize)
	http.Handle("/outputs/", http.StripPrefix("/outputs/", http.FileServer(http.Dir(outputRoot()))))
	fmt.Printf("svg demo server listening on http://127.0.0.1:%s\n", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		panic(err)
	}
}

func handleIndex(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(indexHTML))
}

func handleVectorize(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		writeCORS(w)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeCORS(w)
	var input vectorizeRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxInputBytes+1024)).Decode(&input); err != nil {
		writeJSON(w, map[string]any{"code": 1, "data": nil, "msg": "invalid request"})
		return
	}
	result, err := runJob(r.Context(), input)
	if err != nil {
		writeJSON(w, map[string]any{"code": 1, "data": nil, "msg": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"code": 0, "data": result, "msg": "ok"})
}

func runJob(ctx context.Context, input vectorizeRequest) (jobResult, error) {
	magickPath, err := requireCommand("magick")
	if err != nil {
		return jobResult{}, err
	}
	potracePath, err := requireCommand("potrace")
	if err != nil {
		return jobResult{}, err
	}
	data, err := readInput(ctx, input)
	if err != nil {
		return jobResult{}, err
	}
	if strings.EqualFold(strings.TrimSpace(input.Mode), "layeredRibbon") {
		return runLayeredRibbonJob(ctx, input, data)
	}
	if strings.EqualFold(strings.TrimSpace(input.Mode), "colorMask") {
		return runColorMaskJob(ctx, input, data)
	}
	if strings.EqualFold(strings.TrimSpace(input.Mode), "backendLogo") {
		return runBackendLogoJob(ctx, input, data)
	}
	jobID := newID()
	jobDir := filepath.Join(outputRoot(), jobID)
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		return jobResult{}, err
	}

	sourcePath := filepath.Join(jobDir, "source.png")
	normalizedPath := filepath.Join(jobDir, "normalized.png")
	quantizedPath := filepath.Join(jobDir, "quantized.png")
	outputSVGPath := filepath.Join(jobDir, "output.svg")
	previewPath := filepath.Join(jobDir, "preview.png")
	if err := writeInputPNG(data, sourcePath); err != nil {
		return jobResult{}, err
	}

	longEdge := input.LongEdge
	if longEdge <= 0 {
		longEdge = defaultLongEdge
	}
	colors := input.Colors
	if colors <= 0 {
		colors = defaultColors
	}
	if colors < 2 {
		colors = 2
	}
	removeSpeckles := boolDefault(input.RemoveSpeckles, true)
	fillSmallHoles := boolDefault(input.FillSmallHoles, true)
	minComponentRatio := input.MinComponentRatio
	if minComponentRatio <= 0 {
		minComponentRatio = defaultMinComponentRatio
	}
	maxHoleRatio := input.MaxHoleRatio
	if maxHoleRatio <= 0 {
		maxHoleRatio = defaultMaxHoleRatio
	}
	mergeDistance := input.MergeDistance
	if mergeDistance <= 0 {
		mergeDistance = defaultMergeDistance
	}
	mergeHueDistance := input.MergeHueDistance
	if mergeHueDistance <= 0 {
		mergeHueDistance = defaultMergeHueDistance
	}
	mergeLightness := input.MergeLightness
	if mergeLightness <= 0 {
		mergeLightness = defaultMergeLightness
	}
	mergeSaturation := input.MergeSaturation
	if mergeSaturation <= 0 {
		mergeSaturation = defaultMergeSaturation
	}
	lightMinAreaRatio := input.LightMinAreaRatio
	if lightMinAreaRatio <= 0 {
		lightMinAreaRatio = defaultLightMinAreaRatio
	}

	if err := normalizePNG(ctx, magickPath, sourcePath, normalizedPath, longEdge); err != nil {
		return jobResult{}, err
	}
	if err := quantizePNG(ctx, magickPath, normalizedPath, quantizedPath, colors); err != nil {
		return jobResult{}, err
	}

	sourceColors, _ := countUniqueColors(sourcePath)
	quantizedColors, _ := countUniqueColors(quantizedPath)
	quantizedImage, width, height, err := readPNGImage(quantizedPath)
	if err != nil {
		return jobResult{}, err
	}
	palette := readPalette(quantizedImage)
	background := detectBackgroundLayer(quantizedImage, palette, mergeDistance, mergeHueDistance, mergeLightness, mergeSaturation)
	layers := buildTraceLayers(palette, width*height, background.Keys, mergeDistance, mergeHueDistance, mergeLightness, mergeSaturation)
	if len(layers) == 0 {
		return jobResult{}, errors.New("no traceable color layers")
	}
	sort.SliceStable(layers, func(i, j int) bool {
		return luma(layers[i].R, layers[i].G, layers[i].B) > luma(layers[j].R, layers[j].G, layers[j].B)
	})

	var body strings.Builder
	body.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	body.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`, width, height, width, height) + "\n")
	body.WriteString(fmt.Sprintf(`<rect width="100%%" height="100%%" fill="%s"/>`, background.Hex) + "\n")

	metrics := metricsResult{SourceColors: sourceColors, QuantizedColors: quantizedColors, Background: background.Hex}
	minComponentArea := int(math.Max(4, float64(width*height)*minComponentRatio))
	maxHoleArea := int(math.Max(4, float64(width*height)*maxHoleRatio))
	for index, layer := range layers {
		maskPath := filepath.Join(jobDir, fmt.Sprintf("layer-%02d-mask.png", index))
		layerSVGPath := filepath.Join(jobDir, fmt.Sprintf("layer-%02d.svg", index))
		if err := writeLayerMask(quantizedImage, layer, maskPath); err != nil {
			return jobResult{}, err
		}
		dilateRadius := layerDilateRadius(input.LightDilateRadius, input.DarkDilateRadius, layer)
		if err := smoothMask(ctx, magickPath, maskPath, input.MaskCloseRadius, dilateRadius); err != nil {
			return jobResult{}, err
		}
		layerMinComponentArea := minComponentAreaForLayer(layer, width*height, minComponentArea, lightMinAreaRatio)
		stats, err := cleanMask(maskPath, removeSpeckles, fillSmallHoles, layerMinComponentArea, maxHoleArea)
		if err != nil {
			return jobResult{}, err
		}
		if stats.RemainingBlack == 0 {
			continue
		}
		if err := runPotrace(ctx, magickPath, potracePath, maskPath, layerSVGPath, layer.Hex); err != nil {
			return jobResult{}, err
		}
		group, err := extractPotraceGroup(layerSVGPath)
		if err != nil {
			return jobResult{}, err
		}
		body.WriteString(group)
		body.WriteString("\n")
		metrics.Layers = append(metrics.Layers, layerMetrics{
			Hex:                layer.Hex,
			Pixels:             layer.Count,
			MaskURL:            artifactURL(jobID, filepath.Base(maskPath)),
			SVGURL:             artifactURL(jobID, filepath.Base(layerSVGPath)),
			ComponentsRemoved:  stats.ComponentsRemoved,
			PixelsRemoved:      stats.PixelsRemoved,
			HolesFilled:        stats.HolesFilled,
			HolePixelsFilled:   stats.HolePixelsFilled,
			RemainingBlackArea: stats.RemainingBlack,
		})
	}
	body.WriteString("</svg>\n")
	if err := os.WriteFile(outputSVGPath, []byte(body.String()), 0o644); err != nil {
		return jobResult{}, err
	}
	if err := renderPreview(ctx, magickPath, outputSVGPath, previewPath); err != nil {
		return jobResult{}, err
	}
	if info, err := os.Stat(outputSVGPath); err == nil {
		metrics.SVGBytes = info.Size()
	}
	svgText := body.String()
	metrics.SVGPaths = len(svgPathPattern.FindAllStringIndex(svgText, -1))
	metrics.SVGSubpaths = len(svgMovePattern.FindAllStringIndex(svgText, -1))
	writeMetrics(jobDir, metrics)

	return jobResult{
		ID:      jobID,
		Width:   width,
		Height:  height,
		SVGURL:  artifactURL(jobID, "output.svg"),
		Preview: artifactURL(jobID, "preview.png"),
		Artifacts: []artifact{
			makeArtifact(jobID, "source", sourcePath),
			makeArtifact(jobID, "normalized", normalizedPath),
			makeArtifact(jobID, "quantized", quantizedPath),
			makeArtifact(jobID, "output.svg", outputSVGPath),
			makeArtifact(jobID, "preview.png", previewPath),
			makeArtifact(jobID, "metrics.json", filepath.Join(jobDir, "metrics.json")),
		},
		Metrics: metrics,
	}, nil
}

func runBackendLogoJob(ctx context.Context, input vectorizeRequest, data []byte) (jobResult, error) {
	magickPath, err := requireCommand("magick")
	if err != nil {
		return jobResult{}, err
	}
	jobID := newID()
	jobDir := filepath.Join(outputRoot(), jobID)
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		return jobResult{}, err
	}
	sourcePath := filepath.Join(jobDir, "source.png")
	outputSVGPath := filepath.Join(jobDir, "output.svg")
	previewPath := filepath.Join(jobDir, "preview.png")
	if err := writeInputPNG(data, sourcePath); err != nil {
		return jobResult{}, err
	}

	backendInput := service.VectorizeInput{
		DataURL: "data:image/png;base64," + base64.StdEncoding.EncodeToString(data),
		Mode:    "logo",
	}
	result, err := service.VectorizeImage(backendInput)
	if err != nil {
		return jobResult{}, err
	}
	if err := os.WriteFile(outputSVGPath, []byte(result.Content), 0o644); err != nil {
		return jobResult{}, err
	}
	if err := renderPreview(ctx, magickPath, outputSVGPath, previewPath); err != nil {
		return jobResult{}, err
	}
	metrics := metricsResult{SVGBytes: int64(result.Bytes)}
	svgText := result.Content
	metrics.SVGPaths = len(svgPathPattern.FindAllStringIndex(svgText, -1))
	metrics.SVGSubpaths = len(svgMovePattern.FindAllStringIndex(svgText, -1))
	writeMetrics(jobDir, metrics)

	return jobResult{
		ID:      jobID,
		Width:   result.Width,
		Height:  result.Height,
		SVGURL:  artifactURL(jobID, "output.svg"),
		Preview: artifactURL(jobID, "preview.png"),
		Artifacts: []artifact{
			makeArtifact(jobID, "source", sourcePath),
			makeArtifact(jobID, "output.svg", outputSVGPath),
			makeArtifact(jobID, "preview.png", previewPath),
			makeArtifact(jobID, "metrics.json", filepath.Join(jobDir, "metrics.json")),
		},
		Metrics: metrics,
	}, nil
}

func outputRoot() string {
	root := os.Getenv("SVG_DEMO_OUTPUTS")
	if root != "" {
		return root
	}
	return filepath.Join("demos", "svg", "outputs")
}

func readInput(ctx context.Context, input vectorizeRequest) ([]byte, error) {
	if input.DataURL != "" {
		return decodeDataURL(input.DataURL)
	}
	if input.FilePath != "" {
		return os.ReadFile(input.FilePath)
	}
	if input.ImageURL != "" {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, input.ImageURL, nil)
		if err != nil {
			return nil, err
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("image download failed: %s", resp.Status)
		}
		return io.ReadAll(io.LimitReader(resp.Body, maxInputBytes))
	}
	return nil, errors.New("missing dataUrl, filePath or imageUrl")
}

func decodeDataURL(value string) ([]byte, error) {
	index := strings.Index(value, ",")
	if index >= 0 {
		value = value[index+1:]
	}
	return base64.StdEncoding.DecodeString(value)
}

func writeInputPNG(data []byte, outputPath string) error {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return err
	}
	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()
	return png.Encode(file, img)
}

func normalizePNG(ctx context.Context, magickPath string, inputPath string, outputPath string, longEdge int) error {
	args := []string{
		inputPath,
		"-alpha", "remove",
		"-background", "white",
		"-resize", fmt.Sprintf("%dx%d", longEdge, longEdge),
		"-colorspace", "sRGB",
		"-strip",
		outputPath,
	}
	return runCommand(ctx, magickPath, args...)
}

func quantizePNG(ctx context.Context, magickPath string, inputPath string, outputPath string, colors int) error {
	args := []string{
		inputPath,
		"-dither", "None",
		"-colors", fmt.Sprintf("%d", colors),
		"-type", "TrueColor",
		"-strip",
		outputPath,
	}
	return runCommand(ctx, magickPath, args...)
}

func readPNGImage(path string) (image.Image, int, int, error) {
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

func countUniqueColors(path string) (int, error) {
	img, _, _, err := readPNGImage(path)
	if err != nil {
		return 0, err
	}
	bounds := img.Bounds()
	colors := make(map[uint32]struct{}, 1024)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			colors[colorKey(img.At(x, y))] = struct{}{}
		}
	}
	return len(colors), nil
}

func readPalette(img image.Image) []paletteColor {
	bounds := img.Bounds()
	counts := make(map[uint32]int)
	values := make(map[uint32]paletteColor)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			key := colorKey(img.At(x, y))
			counts[key]++
			if _, ok := values[key]; !ok {
				r, g, b := rgb8(img.At(x, y))
				values[key] = paletteColor{Hex: hexColor(r, g, b), R: r, G: g, B: b}
			}
		}
	}
	palette := make([]paletteColor, 0, len(counts))
	for key, count := range counts {
		item := values[key]
		item.Count = count
		palette = append(palette, item)
	}
	sort.SliceStable(palette, func(i, j int) bool {
		return palette[i].Count > palette[j].Count
	})
	sortPaletteForFunctionalColors(palette)
	return palette
}

func sortPaletteForFunctionalColors(palette []paletteColor) {
	sort.SliceStable(palette, func(i, j int) bool {
		iSat := saturation(palette[i].R, palette[i].G, palette[i].B)
		jSat := saturation(palette[j].R, palette[j].G, palette[j].B)
		if math.Abs(iSat-jSat) > 0.08 {
			return iSat > jSat
		}
		return palette[i].Count > palette[j].Count
	})
}

func detectBackgroundLayer(img image.Image, palette []paletteColor, mergeDistance float64, mergeHueDistance float64, mergeLightness float64, mergeSaturation float64) traceLayer {
	bounds := img.Bounds()
	borderCounts := make(map[uint32]int)
	countBorder := func(x int, y int) {
		borderCounts[colorKey(img.At(bounds.Min.X+x, bounds.Min.Y+y))]++
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

	items := make(map[uint32]paletteColor, len(palette))
	for _, item := range palette {
		items[rgbKey(item.R, item.G, item.B)] = item
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
		background = paletteColor{Hex: "#FFFFFF", R: 255, G: 255, B: 255}
	}

	layer := traceLayer{
		Hex:   background.Hex,
		R:     background.R,
		G:     background.G,
		B:     background.B,
		Count: background.Count,
		Keys:  map[uint32]struct{}{rgbKey(background.R, background.G, background.B): {}},
	}
	for _, item := range palette {
		key := rgbKey(item.R, item.G, item.B)
		if key == backgroundKey {
			continue
		}
		if isBackgroundCompanion(item, layer, borderCounts[key], mergeDistance, mergeHueDistance, mergeLightness, mergeSaturation) {
			layer.Keys[key] = struct{}{}
			layer.Count += item.Count
		}
	}
	return layer
}

func isBackgroundCompanion(item paletteColor, background traceLayer, borderCount int, mergeDistance float64, mergeHueDistance float64, mergeLightness float64, mergeSaturation float64) bool {
	itemIsWhite := isBackgroundWhite(item)
	backgroundIsWhite := isBackgroundWhite(paletteColor{R: background.R, G: background.G, B: background.B})
	if itemIsWhite {
		return backgroundIsWhite
	}
	if borderCount <= 0 && backgroundIsWhite {
		return false
	}
	if borderCount > 0 && canMergeColor(item, background, mergeDistance, mergeHueDistance, mergeLightness*1.4, mergeSaturation) {
		return true
	}
	if saturation(background.R, background.G, background.B) < 0.12 {
		return false
	}
	if math.Abs(saturation(item.R, item.G, item.B)-saturation(background.R, background.G, background.B)) > mergeSaturation {
		return false
	}
	return sameFunctionalHue(item.R, item.G, item.B, background.R, background.G, background.B, mergeHueDistance*2, mergeLightness*1.4)
}

func buildTraceLayers(palette []paletteColor, totalPixels int, backgroundKeys map[uint32]struct{}, mergeDistance float64, mergeHueDistance float64, mergeLightness float64, mergeSaturation float64) []traceLayer {
	minPixels := int(math.Max(16, float64(totalPixels)*0.00002))
	layers := make([]traceLayer, 0, len(palette))
	for _, item := range palette {
		if item.Count < minPixels {
			continue
		}
		if _, ok := backgroundKeys[rgbKey(item.R, item.G, item.B)]; ok {
			continue
		}
		if isNeutralNoise(item, totalPixels) {
			continue
		}
		merged := false
		for index := range layers {
			if !canMergeColor(item, layers[index], mergeDistance, mergeHueDistance, mergeLightness, mergeSaturation) {
				continue
			}
			layer := &layers[index]
			layer.Keys[rgbKey(item.R, item.G, item.B)] = struct{}{}
			layer.Count += item.Count
			if item.Count > layer.Count-item.Count {
				layer.Hex = item.Hex
				layer.R = item.R
				layer.G = item.G
				layer.B = item.B
			}
			merged = true
			break
		}
		if !merged {
			layers = append(layers, traceLayer{
				Hex:   item.Hex,
				R:     item.R,
				G:     item.G,
				B:     item.B,
				Count: item.Count,
				Keys:  map[uint32]struct{}{rgbKey(item.R, item.G, item.B): {}},
			})
		}
	}
	layers = mergeTintTraceLayers(layers, mergeHueDistance, mergeLightness)
	return layers
}

func mergeTintTraceLayers(layers []traceLayer, mergeHueDistance float64, mergeLightness float64) []traceLayer {
	merged := make([]bool, len(layers))
	for sourceIndex := range layers {
		source := layers[sourceIndex]
		sourceSat := saturation(source.R, source.G, source.B)
		if luma(source.R, source.G, source.B) < 180 && sourceSat >= 0.12 {
			continue
		}
		targetIndex := -1
		targetScore := math.MaxFloat64
		for index := range layers {
			if index == sourceIndex || merged[index] {
				continue
			}
			target := layers[index]
			targetSat := saturation(target.R, target.G, target.B)
			if targetSat <= sourceSat || luma(target.R, target.G, target.B) > luma(source.R, source.G, source.B) {
				continue
			}
			if luma(target.R, target.G, target.B) < 140 {
				continue
			}
			if !sameFunctionalHue(source.R, source.G, source.B, target.R, target.G, target.B, mergeHueDistance*2.5, math.Min(70, mergeLightness)) {
				continue
			}
			score := hueGap(source.R, source.G, source.B, target.R, target.G, target.B) + math.Abs(luma(source.R, source.G, source.B)-luma(target.R, target.G, target.B))*0.05
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
	result := make([]traceLayer, 0, len(layers))
	for index, layer := range layers {
		if !merged[index] {
			result = append(result, layer)
		}
	}
	return result
}

func writeLayerMask(img image.Image, target traceLayer, outputPath string) error {
	bounds := img.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	black := color.RGBA{A: 255}
	for y := 0; y < bounds.Dy(); y++ {
		for x := 0; x < bounds.Dx(); x++ {
			if _, ok := target.Keys[colorKey(img.At(bounds.Min.X+x, bounds.Min.Y+y))]; ok {
				out.Set(x, y, black)
			} else {
				out.Set(x, y, white)
			}
		}
	}
	return writePNG(outputPath, out)
}

func smoothMask(ctx context.Context, magickPath string, maskPath string, closeRadius int, dilateRadius int) error {
	if closeRadius <= 0 && dilateRadius <= 0 {
		return nil
	}
	args := []string{maskPath}
	if closeRadius > 0 {
		args = append(args, "-morphology", "Close", fmt.Sprintf("Disk:%d", closeRadius))
	}
	if dilateRadius > 0 {
		args = append(args, "-morphology", "Dilate", fmt.Sprintf("Disk:%d", dilateRadius))
	}
	args = append(args, "-type", "bilevel", maskPath)
	return runCommand(ctx, magickPath, args...)
}

func layerDilateRadius(lightRadius int, darkRadius int, layer traceLayer) int {
	if luma(layer.R, layer.G, layer.B) >= 120 {
		return max(0, lightRadius)
	}
	return max(0, darkRadius)
}

func minComponentAreaForLayer(layer traceLayer, totalPixels int, fallback int, lightMinAreaRatio float64) int {
	if luma(layer.R, layer.G, layer.B) >= 120 && saturation(layer.R, layer.G, layer.B) < 0.35 {
		return max(fallback, int(float64(totalPixels)*lightMinAreaRatio))
	}
	return fallback
}

func cleanMask(path string, removeSpeckles bool, fillSmallHoles bool, minComponentArea int, maxHoleArea int) (cleanStats, error) {
	img, width, height, err := readMask(path)
	if err != nil {
		return cleanStats{}, err
	}
	stats := cleanStats{}
	if removeSpeckles {
		removeSmallBlackComponents(img, width, height, minComponentArea, &stats)
	}
	if fillSmallHoles {
		fillWhiteHoles(img, width, height, maxHoleArea, &stats)
	}
	stats.RemainingBlack = countBlackPixels(img)
	return stats, writePNG(path, img)
}

func readMask(path string) (*image.RGBA, int, int, error) {
	src, width, height, err := readPNGImage(path)
	if err != nil {
		return nil, 0, 0, err
	}
	out := image.NewRGBA(image.Rect(0, 0, width, height))
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	black := color.RGBA{A: 255}
	bounds := src.Bounds()
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if isBlack(src.At(bounds.Min.X+x, bounds.Min.Y+y)) {
				out.Set(x, y, black)
			} else {
				out.Set(x, y, white)
			}
		}
	}
	return out, width, height, nil
}

func removeSmallBlackComponents(img *image.RGBA, width int, height int, minArea int, stats *cleanStats) {
	visited := make([]bool, width*height)
	walkComponents(width, height, visited, func(x int, y int) bool {
		return isBlack(img.At(x, y))
	}, func(points []image.Point, touchesBorder bool) {
		if len(points) >= minArea {
			return
		}
		stats.ComponentsRemoved++
		stats.PixelsRemoved += len(points)
		for _, point := range points {
			img.Set(point.X, point.Y, color.RGBA{R: 255, G: 255, B: 255, A: 255})
		}
	})
}

func fillWhiteHoles(img *image.RGBA, width int, height int, maxArea int, stats *cleanStats) {
	visited := make([]bool, width*height)
	walkComponents(width, height, visited, func(x int, y int) bool {
		return !isBlack(img.At(x, y))
	}, func(points []image.Point, touchesBorder bool) {
		if touchesBorder || len(points) > maxArea {
			return
		}
		stats.HolesFilled++
		stats.HolePixelsFilled += len(points)
		for _, point := range points {
			img.Set(point.X, point.Y, color.RGBA{A: 255})
		}
	})
}

func walkComponents(width int, height int, visited []bool, match func(int, int) bool, handle func([]image.Point, bool)) {
	directions := [...]image.Point{{X: 1}, {X: -1}, {Y: 1}, {Y: -1}}
	queue := make([]image.Point, 0, 1024)
	points := make([]image.Point, 0, 1024)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			index := y*width + x
			if visited[index] || !match(x, y) {
				continue
			}
			visited[index] = true
			touchesBorder := x == 0 || y == 0 || x == width-1 || y == height-1
			queue = append(queue[:0], image.Point{X: x, Y: y})
			points = append(points[:0], image.Point{X: x, Y: y})
			for len(queue) > 0 {
				point := queue[len(queue)-1]
				queue = queue[:len(queue)-1]
				for _, direction := range directions {
					next := image.Point{X: point.X + direction.X, Y: point.Y + direction.Y}
					if next.X < 0 || next.Y < 0 || next.X >= width || next.Y >= height {
						continue
					}
					nextIndex := next.Y*width + next.X
					if visited[nextIndex] || !match(next.X, next.Y) {
						continue
					}
					visited[nextIndex] = true
					if next.X == 0 || next.Y == 0 || next.X == width-1 || next.Y == height-1 {
						touchesBorder = true
					}
					queue = append(queue, next)
					points = append(points, next)
				}
			}
			handle(points, touchesBorder)
		}
	}
}

func countBlackPixels(img *image.RGBA) int {
	bounds := img.Bounds()
	count := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if isBlack(img.At(x, y)) {
				count++
			}
		}
	}
	return count
}

func runPotrace(ctx context.Context, magickPath string, potracePath string, maskPath string, outputPath string, fill string) error {
	bitmapPath := strings.TrimSuffix(outputPath, filepath.Ext(outputPath)) + ".pbm"
	if err := runCommand(ctx, magickPath, maskPath, "-type", "bilevel", "pbm:"+bitmapPath); err != nil {
		return err
	}
	args := []string{bitmapPath, "-s", "--group", "--flat", "-t", "8", "-a", "0.75", "-O", "0.90", "-u", "10", "-C", fill, "-o", outputPath}
	return runCommand(ctx, potracePath, args...)
}

func extractPotraceGroup(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	group := svgGroupPattern.FindString(string(data))
	if group == "" {
		return "", fmt.Errorf("potrace did not produce an svg group: %s", path)
	}
	return group, nil
}

func renderPreview(ctx context.Context, magickPath string, svgPath string, outputPath string) error {
	return runCommand(ctx, magickPath, "-background", "white", svgPath, "-resize", "1600x", outputPath)
}

func writeMetrics(jobDir string, metrics metricsResult) {
	data, _ := json.MarshalIndent(metrics, "", "  ")
	_ = os.WriteFile(filepath.Join(jobDir, "metrics.json"), data, 0o644)
}

func writePNG(path string, img image.Image) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return png.Encode(file, img)
}

func runCommand(ctx context.Context, name string, args ...string) error {
	commandCtx, cancel := context.WithTimeout(ctx, 180*time.Second)
	defer cancel()
	cmd := exec.CommandContext(commandCtx, name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s failed: %w: %s", filepath.Base(name), err, strings.TrimSpace(string(output)))
	}
	return nil
}

func requireCommand(name string) (string, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("missing command: %s", name)
	}
	return path, nil
}

func writeCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(value)
}

func makeArtifact(jobID string, name string, path string) artifact {
	return artifact{Name: name, URL: artifactURL(jobID, filepath.Base(path)), Path: path}
}

func artifactURL(jobID string, name string) string {
	return "/outputs/" + jobID + "/" + name
}

func newID() string {
	var data [8]byte
	if _, err := rand.Read(data[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("%x", data[:])
}

func boolDefault(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func colorKey(c color.Color) uint32 {
	r, g, b := rgb8(c)
	return rgbKey(r, g, b)
}

func rgbKey(r uint8, g uint8, b uint8) uint32 {
	return uint32(r)<<16 | uint32(g)<<8 | uint32(b)
}

func rgb8(c color.Color) (uint8, uint8, uint8) {
	r, g, b, _ := c.RGBA()
	return uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)
}

func hexColor(r uint8, g uint8, b uint8) string {
	return fmt.Sprintf("#%02X%02X%02X", r, g, b)
}

func isBlack(c color.Color) bool {
	r, g, b, _ := c.RGBA()
	return int(r>>8)+int(g>>8)+int(b>>8) < 384
}

func isBackgroundWhite(item paletteColor) bool {
	minChannel := min3(item.R, item.G, item.B)
	maxChannel := max3(item.R, item.G, item.B)
	return minChannel > 235 && maxChannel-minChannel < 24
}

func isNeutralNoise(item paletteColor, totalPixels int) bool {
	if saturation(item.R, item.G, item.B) > 0.04 {
		return false
	}
	return float64(item.Count)/float64(totalPixels) < 0.002
}

func luma(r uint8, g uint8, b uint8) float64 {
	return 0.2126*float64(r) + 0.7152*float64(g) + 0.0722*float64(b)
}

func colorDistance(a paletteColor, b traceLayer) float64 {
	dr := float64(int(a.R) - int(b.R))
	dg := float64(int(a.G) - int(b.G))
	db := float64(int(a.B) - int(b.B))
	return math.Sqrt(dr*dr + dg*dg + db*db)
}

func canMergeColor(a paletteColor, b traceLayer, mergeDistance float64, mergeHueDistance float64, mergeLightness float64, mergeSaturation float64) bool {
	aHue, aSat := hueAndSaturation(a.R, a.G, a.B)
	bHue, bSat := hueAndSaturation(b.R, b.G, b.B)
	aLuma := luma(a.R, a.G, a.B)
	bLuma := luma(b.R, b.G, b.B)
	if aSat < 0.12 || bSat < 0.12 {
		return colorDistance(a, b) <= mergeDistance || canMergeTintColor(a, b, mergeHueDistance, mergeLightness)
	}
	if hueDistance(aHue, bHue) > mergeHueDistance {
		return false
	}
	if bLuma < 120 && aLuma-bLuma > 35 {
		return false
	}
	if bLuma >= 140 && aLuma < 120 {
		return false
	}
	if math.Abs(aSat-bSat) > mergeSaturation {
		return false
	}
	if math.Abs(aLuma-bLuma) > mergeLightness {
		return false
	}
	return true
}

func canMergeTintColor(a paletteColor, b traceLayer, mergeHueDistance float64, mergeLightness float64) bool {
	aSat := saturation(a.R, a.G, a.B)
	bSat := saturation(b.R, b.G, b.B)
	if aSat < 0.03 && bSat < 0.03 {
		return false
	}
	if sameFunctionalHue(a.R, a.G, a.B, b.R, b.G, b.B, mergeHueDistance*2, mergeLightness*1.8) {
		return true
	}
	return sameChannelBias(a.R, a.G, a.B, b.R, b.G, b.B) && math.Abs(luma(a.R, a.G, a.B)-luma(b.R, b.G, b.B)) <= mergeLightness*2.2
}

func sameFunctionalHue(ar uint8, ag uint8, ab uint8, br uint8, bg uint8, bb uint8, maxHueDistance float64, maxLightnessDistance float64) bool {
	return hueGap(ar, ag, ab, br, bg, bb) <= maxHueDistance && math.Abs(luma(ar, ag, ab)-luma(br, bg, bb)) <= maxLightnessDistance
}

func hueGap(ar uint8, ag uint8, ab uint8, br uint8, bg uint8, bb uint8) float64 {
	aHue, _ := hueAndSaturation(ar, ag, ab)
	bHue, _ := hueAndSaturation(br, bg, bb)
	return hueDistance(aHue, bHue)
}

func sameChannelBias(ar uint8, ag uint8, ab uint8, br uint8, bg uint8, bb uint8) bool {
	return channelOrder(ar, ag, ab) == channelOrder(br, bg, bb)
}

func channelOrder(r uint8, g uint8, b uint8) string {
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

func hueAndSaturation(r uint8, g uint8, b uint8) (float64, float64) {
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

func saturation(r uint8, g uint8, b uint8) float64 {
	_, sat := hueAndSaturation(r, g, b)
	return sat
}

func hueDistance(a float64, b float64) float64 {
	diff := math.Abs(a - b)
	if diff > 180 {
		return 360 - diff
	}
	return diff
}

func min3(a uint8, b uint8, c uint8) uint8 {
	if b < a {
		a = b
	}
	if c < a {
		a = c
	}
	return a
}

func max3(a uint8, b uint8, c uint8) uint8 {
	if b > a {
		a = b
	}
	if c > a {
		a = c
	}
	return a
}

const indexHTML = `<!doctype html>
<html>
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>PNG to SVG Lab</title>
  <style>
    body { font-family: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; margin: 24px; color: #202124; }
    main { max-width: 1180px; margin: 0 auto; }
    label { display: block; margin: 12px 0 4px; font-weight: 600; }
    input, button { font: inherit; }
    input[type="number"] { width: 120px; }
    button { margin-top: 16px; padding: 8px 14px; cursor: pointer; }
    .row { display: flex; gap: 24px; flex-wrap: wrap; align-items: end; }
    .preview { display: grid; grid-template-columns: repeat(auto-fit, minmax(320px, 1fr)); gap: 20px; margin-top: 24px; }
    .box { border: 1px solid #ddd; padding: 12px; background: #fff; }
    img { max-width: 100%; background: white; }
    pre { white-space: pre-wrap; background: #f6f8fa; padding: 12px; overflow: auto; }
  </style>
</head>
<body>
<main>
  <h1>PNG to SVG Lab</h1>
  <div class="row">
    <div>
      <label>PNG/JPG</label>
      <input id="file" type="file" accept="image/*" />
    </div>
    <div>
      <label>Colors</label>
      <input id="colors" type="number" value="16" min="2" max="48" />
    </div>
    <div>
      <label>Long edge</label>
      <input id="longEdge" type="number" value="4096" min="512" max="8192" />
    </div>
    <div>
      <label>Min component ratio</label>
      <input id="minComponentRatio" type="number" value="0.00001" step="0.000001" />
    </div>
    <div>
      <label>Max hole ratio</label>
      <input id="maxHoleRatio" type="number" value="0.00004" step="0.000001" />
    </div>
    <div>
      <label>Merge distance</label>
      <input id="mergeDistance" type="number" value="48" min="0" max="220" />
    </div>
    <div>
      <label>Merge hue</label>
      <input id="mergeHueDistance" type="number" value="16" min="0" max="180" />
    </div>
    <div>
      <label>Merge light</label>
      <input id="mergeLightness" type="number" value="80" min="0" max="255" />
    </div>
    <div>
      <label>Merge sat</label>
      <input id="mergeSaturation" type="number" value="0.55" min="0" max="1" step="0.01" />
    </div>
    <div>
      <label>Light min area</label>
      <input id="lightMinAreaRatio" type="number" value="0.003" min="0" max="0.05" step="0.0005" />
    </div>
    <div>
      <label>Mask close</label>
      <input id="maskCloseRadius" type="number" value="0" min="0" max="12" />
    </div>
    <div>
      <label>Light dilate</label>
      <input id="lightDilateRadius" type="number" value="0" min="0" max="12" />
    </div>
    <div>
      <label>Dark dilate</label>
      <input id="darkDilateRadius" type="number" value="0" min="0" max="8" />
    </div>
  </div>
  <button id="run">Vectorize</button>
  <p id="status"></p>
  <div class="preview">
    <div class="box"><h3>Source</h3><img id="source" /></div>
    <div class="box"><h3>Preview</h3><img id="preview" /></div>
  </div>
  <h3>Metrics</h3>
  <pre id="metrics"></pre>
</main>
<script>
const fileInput = document.getElementById('file');
const source = document.getElementById('source');
fileInput.addEventListener('change', () => {
  const file = fileInput.files[0];
  if (file) source.src = URL.createObjectURL(file);
});
document.getElementById('run').addEventListener('click', async () => {
  const file = fileInput.files[0];
  if (!file) return alert('Choose an image first');
  document.getElementById('status').textContent = 'Processing...';
  const dataUrl = await new Promise((resolve) => {
    const reader = new FileReader();
    reader.onload = () => resolve(reader.result);
    reader.readAsDataURL(file);
  });
  const body = {
    dataUrl,
    colors: Number(document.getElementById('colors').value),
    longEdge: Number(document.getElementById('longEdge').value),
    minComponentRatio: Number(document.getElementById('minComponentRatio').value),
    maxHoleRatio: Number(document.getElementById('maxHoleRatio').value),
    mergeDistance: Number(document.getElementById('mergeDistance').value),
    mergeHueDistance: Number(document.getElementById('mergeHueDistance').value),
    mergeLightness: Number(document.getElementById('mergeLightness').value),
    mergeSaturation: Number(document.getElementById('mergeSaturation').value),
    lightMinAreaRatio: Number(document.getElementById('lightMinAreaRatio').value),
    maskCloseRadius: Number(document.getElementById('maskCloseRadius').value),
    lightDilateRadius: Number(document.getElementById('lightDilateRadius').value),
    darkDilateRadius: Number(document.getElementById('darkDilateRadius').value)
  };
  const res = await fetch('/api/vectorize', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body)
  });
  const payload = await res.json();
  if (payload.code !== 0) {
    document.getElementById('status').textContent = payload.msg || 'Failed';
    return;
  }
  document.getElementById('preview').src = payload.data.previewUrl + '?t=' + Date.now();
  document.getElementById('metrics').textContent = JSON.stringify(payload.data.metrics, null, 2);
  document.getElementById('status').innerHTML = 'Done: <a href="' + payload.data.svgUrl + '" target="_blank">output.svg</a>';
});
</script>
</body>
</html>
`
