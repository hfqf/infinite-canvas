package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
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
	defaultMinComponentRatio = 0.00005
	defaultMaxHoleRatio      = 0.00008
	defaultMergeDistance     = 64
	defaultMergeHueDistance  = 16
	defaultMergeLightness    = 80
	defaultMergeSaturation   = 0.55
	defaultLightMinAreaRatio = 0.004
	maxInputBytes            = 60 << 20
)

var (
	svgGroupPattern      = regexp.MustCompile(`(?s)<g\b.*</g>`)
	svgPathPattern       = regexp.MustCompile(`<path\b`)
	svgMovePattern       = regexp.MustCompile(`\bM`)
	histogramLinePattern = regexp.MustCompile(`^\s*(\d+):\s+\(([^,]+),([^,]+),([^)]+)\)`)
	rmseMetricPattern    = regexp.MustCompile(`\(([0-9.]+(?:e[-+]?\d+)?)\)`)
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

type batchItemResult struct {
	Source           string            `json:"source"`
	JobID            string            `json:"jobId,omitempty"`
	VisualComparison string            `json:"visualComparison,omitempty"`
	OutputSVG        string            `json:"outputSvg,omitempty"`
	Metrics          metricsResult     `json:"metrics,omitempty"`
	Quality          logoQualityResult `json:"quality,omitempty"`
	Error            string            `json:"error,omitempty"`
}

type batchRunResult struct {
	InputDir     string            `json:"inputDir"`
	OutputDir    string            `json:"outputDir"`
	ContactSheet string            `json:"contactSheet,omitempty"`
	Report       string            `json:"report,omitempty"`
	Passed       int               `json:"passed"`
	Warnings     int               `json:"warnings"`
	Failed       int               `json:"failed"`
	Errored      int               `json:"errored"`
	Items        []batchItemResult `json:"items"`
}

type logoQualityResult struct {
	Status   string             `json:"status"`
	Failed   int                `json:"failed"`
	Warnings int                `json:"warnings"`
	Checks   []logoQualityCheck `json:"checks"`
}

type logoQualityCheck struct {
	Name    string  `json:"name"`
	Label   string  `json:"label"`
	Status  string  `json:"status"`
	Value   float64 `json:"value"`
	WarnAt  float64 `json:"warnAt,omitempty"`
	FailAt  float64 `json:"failAt"`
	Message string  `json:"message"`
}

type metricsResult struct {
	SourceColors                    int             `json:"sourceColors"`
	QuantizedColors                 int             `json:"quantizedColors"`
	Background                      string          `json:"background"`
	Request                         vectorizeParams `json:"request"`
	Layers                          []layerMetrics  `json:"layers"`
	SVGBytes                        int64           `json:"svgBytes"`
	SVGPaths                        int             `json:"svgPaths"`
	SVGSubpaths                     int             `json:"svgSubpaths"`
	BackgroundDelta                 float64         `json:"backgroundDelta"`
	SourceForegroundColor           string          `json:"sourceForegroundColor,omitempty"`
	RenderedForegroundColor         string          `json:"renderedForegroundColor,omitempty"`
	ForegroundColorDelta            float64         `json:"foregroundColorDelta"`
	SourceForegroundCoverage        float64         `json:"sourceForegroundCoverage"`
	RenderedForegroundCoverage      float64         `json:"renderedForegroundCoverage"`
	ForegroundCoverageDelta         float64         `json:"foregroundCoverageDelta"`
	RenderedRMSE                    float64         `json:"renderedRMSE"`
	DarkLightTextTintBleedRatio     float64         `json:"darkLightTextTintBleedRatio"`
	HasDarkLightTextTintBleedMetric bool            `json:"hasDarkLightTextTintBleedMetric,omitempty"`
}

type vectorizeParams struct {
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

type rgbColor struct {
	R uint8
	G uint8
	B uint8
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

type logoHistogramColor struct {
	Count int
	R     uint8
	G     uint8
	B     uint8
}

func main() {
	batchDir := flag.String("batch", "", "run cleanLogo on every image under this directory and exit")
	batchOut := flag.String("batch-out", "", "output directory for --batch; defaults to demos/svg/outputs/batch-{timestamp}")
	batchStrict := flag.Bool("batch-strict", false, "exit with an error when --batch has failed or errored items")
	batchMinCount := flag.Int("batch-min-count", 0, "exit with an error when --batch reviews fewer than this many images")
	corpusOut := flag.String("corpus-out", "", "write built-in logo stress corpus to this directory and exit")
	flag.Parse()
	if err := config.Load(); err != nil {
		panic(err)
	}
	if strings.TrimSpace(*corpusOut) != "" {
		paths, err := generateBuiltInCorpus(context.Background(), *corpusOut)
		if err != nil {
			panic(err)
		}
		fmt.Printf("generated %d corpus image(s) under %s\n", len(paths), *corpusOut)
		return
	}
	if strings.TrimSpace(*batchDir) != "" {
		if err := runBatchCLI(context.Background(), *batchDir, *batchOut, *batchStrict, *batchMinCount); err != nil {
			panic(err)
		}
		return
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

func generateBuiltInCorpus(ctx context.Context, outputDir string) ([]string, error) {
	outputDir = strings.TrimSpace(outputDir)
	if outputDir == "" {
		return nil, errors.New("corpus output directory is required")
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, err
	}
	magickPath, err := requireCommand("magick")
	if err != nil {
		return nil, err
	}
	paths := []string{
		filepath.Join(outputDir, "01-bls-pale-ribbon.png"),
		filepath.Join(outputDir, "02-multicolor-block-text.png"),
		filepath.Join(outputDir, "03-micro-text.png"),
		filepath.Join(outputDir, "04-serif-logistic.png"),
		filepath.Join(outputDir, "05-cjk-slogan-banner.png"),
		filepath.Join(outputDir, "06-dark-white-text.png"),
		filepath.Join(outputDir, "07-white-text-brand-block.png"),
		filepath.Join(outputDir, "08-same-hue-gradient.png"),
		filepath.Join(outputDir, "09-narrow-gaps.png"),
		filepath.Join(outputDir, "10-small-marks.png"),
	}
	generators := []func(context.Context, string, string) error{
		generateCorpusBLSPaleRibbon,
		generateCorpusMulticolorBlockText,
		generateCorpusMicroText,
		generateCorpusSerifLogistic,
		generateCorpusCJKSloganBanner,
		generateCorpusDarkWhiteText,
		generateCorpusWhiteTextBrandBlock,
		generateCorpusSameHueGradient,
		generateCorpusNarrowGaps,
		generateCorpusSmallMarks,
	}
	for index, generator := range generators {
		if err := generator(ctx, magickPath, paths[index]); err != nil {
			return nil, err
		}
	}
	return paths, nil
}

func generateCorpusBLSPaleRibbon(ctx context.Context, magickPath string, outputPath string) error {
	font := corpusSerifFontPath()
	args := []string{
		"-size", "1536x1024",
		"xc:#fdfdfd",
		"-fill", "#c4d9e3",
		"-draw", "path 'M 270,475 C 470,590 650,610 860,535 C 1080,455 1285,450 1440,535 L 1440,650 C 1200,545 1015,540 820,610 C 590,690 405,655 270,600 Z'",
		"-fill", "#ecf3f6",
		"-draw", "path 'M 340,520 C 560,610 760,600 950,540 C 1135,485 1290,500 1440,565 L 1440,600 C 1255,535 1115,530 950,585 C 735,655 560,655 340,570 Z'",
		"-font", font,
		"-fill", "#00768e",
		"-pointsize", "300",
		"-gravity", "northwest",
		"-annotate", "+110+70", "30",
		"-pointsize", "72",
		"-annotate", "+300+246", "YEARS",
		"-pointsize", "300",
		"-annotate", "+600+320", "BLS",
		"-pointsize", "96",
		"-annotate", "+335+720", "Bremer Logistic Service",
		"-resize", "768x512!",
		"-colorspace", "sRGB",
		"-strip",
		outputPath,
	}
	return runCommand(ctx, magickPath, args...)
}

func generateCorpusMulticolorBlockText(_ context.Context, _ string, outputPath string) error {
	img := image.NewRGBA(image.Rect(0, 0, 720, 420))
	white := color.RGBA{R: 0xfd, G: 0xfd, B: 0xfd, A: 0xff}
	blue := color.RGBA{R: 0x24, G: 0x58, B: 0xe6, A: 0xff}
	red := color.RGBA{R: 0xe6, G: 0x32, B: 0x23, A: 0xff}
	green := color.RGBA{R: 0x18, G: 0xa0, B: 0x58, A: 0xff}
	corpusFillRect(img, image.Rect(0, 0, 720, 420), white)
	corpusFillRect(img, image.Rect(90, 90, 260, 290), blue)
	corpusFillRect(img, image.Rect(310, 90, 480, 290), red)
	corpusFillRect(img, image.Rect(530, 90, 650, 290), green)
	corpusFillRect(img, image.Rect(130, 150, 220, 205), white)
	corpusFillRect(img, image.Rect(350, 150, 438, 205), white)
	corpusFillRect(img, image.Rect(90, 330, 650, 355), blue)
	return writePNG(outputPath, img)
}

func generateCorpusMicroText(ctx context.Context, magickPath string, outputPath string) error {
	tmpPath := outputPath + ".large.png"
	img := image.NewRGBA(image.Rect(0, 0, 1440, 840))
	white := color.RGBA{R: 0xfd, G: 0xfd, B: 0xfd, A: 0xff}
	black := color.RGBA{R: 0x13, G: 0x13, B: 0x13, A: 0xff}
	blue := color.RGBA{R: 0x24, G: 0x58, B: 0xe6, A: 0xff}
	corpusFillRect(img, image.Rect(0, 0, 1440, 840), white)
	corpusFillRect(img, image.Rect(190, 170, 310, 570), blue)
	corpusFillRect(img, image.Rect(190, 170, 690, 270), blue)
	corpusFillRect(img, image.Rect(190, 470, 690, 570), blue)
	corpusFillRect(img, image.Rect(800, 170, 920, 570), black)
	corpusFillRect(img, image.Rect(800, 470, 1190, 570), black)
	for row := 0; row < 3; row++ {
		y := 650 + row*46
		for col := 0; col < 28; col++ {
			x := 170 + col*38
			corpusFillRect(img, image.Rect(x, y, x+8, y+22), black)
			corpusFillRect(img, image.Rect(x+14, y, x+22, y+22), black)
			corpusFillRect(img, image.Rect(x, y+28, x+28, y+36), black)
		}
	}
	if err := writePNG(tmpPath, img); err != nil {
		return err
	}
	defer os.Remove(tmpPath)
	return runCommand(ctx, magickPath, tmpPath, "-resize", "720x420!", "-colorspace", "sRGB", "-strip", outputPath)
}

func generateCorpusSerifLogistic(ctx context.Context, magickPath string, outputPath string) error {
	return generateCorpusBLSPaleRibbon(ctx, magickPath, outputPath)
}

func generateCorpusCJKSloganBanner(ctx context.Context, magickPath string, outputPath string) error {
	font := corpusCJKFontPath()
	args := []string{
		"-size", "2048x360",
		"xc:#d4e4f0",
		"-fill", "#dbe8f2",
		"-draw", "path 'M 0,72 C 160,112 330,50 500,76 C 720,112 940,48 1160,70 C 1390,95 1600,46 1820,72 C 1940,84 2020,58 2048,70 L 2048,0 L 0,0 Z'",
		"-fill", "#eef4f8",
		"-draw", "roundrectangle 70,92 1970,282 28,28",
		"-fill", "#a8cbe2",
		"-draw", "path 'M 20,245 C 210,188 365,205 500,252 C 690,310 880,276 1045,230 C 1235,178 1405,188 1580,238 C 1775,296 1945,262 2048,220 L 2048,360 L 0,360 Z'",
		"-fill", "#15547a",
		"-draw", "polygon 0,160 55,126 116,170 116,360 0,360",
		"-draw", "rectangle 20,175 110,360",
		"-fill", "#f1ddb1",
		"-draw", "path 'M 0,306 C 315,332 540,286 830,308 C 1125,330 1420,288 1680,302 C 1860,312 1975,300 2048,286 L 2048,320 C 1840,340 1645,322 1455,326 C 1110,332 865,336 635,322 C 410,308 205,340 0,328 Z'",
		"-fill", "#0b3c6f",
		"-font", font,
		"-pointsize", "104",
		"-gravity", "north",
		"-annotate", "+0+92", "人民有信仰  国家有力量  民族有希望",
		"-resize", "1024x180!",
		"-colorspace", "sRGB",
		"-strip",
		outputPath,
	}
	return runCommand(ctx, magickPath, args...)
}

func generateCorpusDarkWhiteText(ctx context.Context, magickPath string, outputPath string) error {
	font := corpusSansFontPath()
	args := []string{
		"-size", "1200x700",
		"xc:#0f172a",
		"-font", font,
		"-fill", "#38bdf8",
		"-draw", "roundrectangle 110,120 360,370 40,40",
		"-fill", "#0f172a",
		"-pointsize", "132",
		"-gravity", "northwest",
		"-annotate", "+150+170", "AI",
		"-fill", "#f4f4f4",
		"-pointsize", "104",
		"-annotate", "+410+140", "NOVA",
		"-pointsize", "40",
		"-annotate", "+418+286", "Light Text Vector Service",
		"-resize", "720x420!",
		"-colorspace", "sRGB",
		"-strip",
		outputPath,
	}
	return runCommand(ctx, magickPath, args...)
}

func generateCorpusWhiteTextBrandBlock(ctx context.Context, magickPath string, outputPath string) error {
	font := corpusSansFontPath()
	args := []string{
		"-size", "1200x700",
		"xc:#fdfdfd",
		"-font", font,
		"-fill", "#08758d",
		"-draw", "roundrectangle 130,120 1050,430 42,42",
		"-fill", "#fdfdfd",
		"-pointsize", "128",
		"-gravity", "northwest",
		"-annotate", "+190+208", "WHITE BRAND",
		"-fill", "#08758d",
		"-draw", "rectangle 180,520 1020,548",
		"-resize", "720x420!",
		"-colorspace", "sRGB",
		"-strip",
		outputPath,
	}
	return runCommand(ctx, magickPath, args...)
}

func generateCorpusSameHueGradient(_ context.Context, _ string, outputPath string) error {
	img := image.NewRGBA(image.Rect(0, 0, 720, 420))
	white := color.RGBA{R: 0xfd, G: 0xfd, B: 0xfd, A: 0xff}
	dark := color.RGBA{R: 0x00, G: 0x6d, B: 0x87, A: 0xff}
	mid := color.RGBA{R: 0x09, G: 0x82, B: 0x9d, A: 0xff}
	light := color.RGBA{R: 0x40, G: 0xa9, B: 0xba, A: 0xff}
	corpusFillRect(img, image.Rect(0, 0, 720, 420), white)
	for y := 90; y < 250; y++ {
		fill := dark
		if y < 145 {
			fill = light
		} else if y < 200 {
			fill = mid
		}
		corpusFillRect(img, image.Rect(110, y, 610, y+1), fill)
	}
	corpusFillRect(img, image.Rect(175, 145, 545, 195), white)
	corpusFillRect(img, image.Rect(110, 275, 610, 305), dark)
	return writePNG(outputPath, img)
}

func generateCorpusNarrowGaps(_ context.Context, _ string, outputPath string) error {
	img := image.NewRGBA(image.Rect(0, 0, 720, 420))
	white := color.RGBA{R: 0xfd, G: 0xfd, B: 0xfd, A: 0xff}
	blue := color.RGBA{R: 0x24, G: 0x58, B: 0xe6, A: 0xff}
	red := color.RGBA{R: 0xe6, G: 0x32, B: 0x23, A: 0xff}
	green := color.RGBA{R: 0x18, G: 0xa0, B: 0x58, A: 0xff}
	corpusFillRect(img, image.Rect(0, 0, 720, 420), white)
	corpusFillRect(img, image.Rect(105, 95, 300, 290), blue)
	corpusFillRect(img, image.Rect(316, 95, 510, 290), red)
	corpusFillRect(img, image.Rect(526, 95, 640, 290), green)
	corpusFillRect(img, image.Rect(145, 330, 610, 352), blue)
	return writePNG(outputPath, img)
}

func generateCorpusSmallMarks(ctx context.Context, magickPath string, outputPath string) error {
	args := []string{
		"-size", "1440x840",
		"xc:#fdfdfd",
		"-fill", "#131313",
		"-draw", "roundrectangle 210,190 930,520 42,42",
		"-fill", "#fdfdfd",
		"-draw", "roundrectangle 330,305 760,405 22,22",
		"-fill", "#131313",
		"-draw", "circle 1010,185 1010,154",
		"-fill", "#fdfdfd",
		"-draw", "circle 1010,185 1010,169",
		"-fill", "#131313",
		"-draw", "circle 1065,185 1065,172",
		"-draw", "circle 1110,185 1110,174",
		"-draw", "circle 1150,185 1150,176",
		"-draw", "rectangle 260,610 365,630",
		"-draw", "rectangle 420,610 548,630",
		"-resize", "720x420!",
		"-colorspace", "sRGB",
		"-strip",
		outputPath,
	}
	return runCommand(ctx, magickPath, args...)
}

func corpusFillRect(img *image.RGBA, rect image.Rectangle, fill color.Color) {
	draw.Draw(img, rect, &image.Uniform{C: fill}, image.Point{}, draw.Src)
}

func corpusSansFontPath() string {
	return firstExistingPath([]string{
		"/System/Library/Fonts/HelveticaNeue.ttc",
		"/System/Library/Fonts/SFNS.ttf",
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		"/usr/share/fonts/truetype/liberation2/LiberationSans-Regular.ttf",
	}, "Helvetica")
}

func corpusSerifFontPath() string {
	return firstExistingPath([]string{
		"/System/Library/Fonts/Times.ttc",
		"/System/Library/Fonts/Supplemental/Times New Roman.ttf",
		"/System/Library/Fonts/Supplemental/Georgia.ttf",
		"/usr/share/fonts/truetype/dejavu/DejaVuSerif.ttf",
		"/usr/share/fonts/truetype/liberation2/LiberationSerif-Regular.ttf",
	}, "Times")
}

func corpusCJKFontPath() string {
	return firstExistingPath([]string{
		"/System/Library/Fonts/STHeiti Medium.ttc",
		"/System/Library/Fonts/STHeiti Light.ttc",
		"/System/Library/Fonts/Supplemental/Songti.ttc",
		"/System/Library/Fonts/Supplemental/Arial Unicode.ttf",
		"/Library/Fonts/Arial Unicode.ttf",
		"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
	}, "Helvetica")
}

func firstExistingPath(candidates []string, fallback string) string {
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return fallback
}

func runBatchCLI(ctx context.Context, inputDir string, outputDir string, strict bool, minCount int) error {
	result, err := runBatch(ctx, inputDir, outputDir)
	if err != nil {
		return err
	}
	summaryPath := filepath.Join(result.OutputDir, "batch-summary.json")
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(summaryPath, data, 0o644); err != nil {
		return err
	}
	reportPath := filepath.Join(result.OutputDir, "batch-report.md")
	if err := os.WriteFile(reportPath, []byte(renderBatchMarkdownReport(result)), 0o644); err != nil {
		return err
	}
	fmt.Printf("processed %d image(s)\n%s\nsummary: %s\nreport: %s\n", len(result.Items), formatBatchQualitySummary(result), summaryPath, reportPath)
	if result.ContactSheet != "" {
		fmt.Printf("contact sheet: %s\n", result.ContactSheet)
	}
	if err := minimumBatchCountError(result, minCount); err != nil {
		return err
	}
	if strict {
		return strictBatchQualityError(result)
	}
	return nil
}

func minimumBatchCountError(result batchRunResult, minCount int) error {
	if minCount <= 0 || len(result.Items) >= minCount {
		return nil
	}
	return fmt.Errorf("batch reviewed %d image(s), below required minimum %d; report: %s", len(result.Items), minCount, result.Report)
}

func strictBatchQualityError(result batchRunResult) error {
	if result.Failed == 0 && result.Errored == 0 {
		return nil
	}
	return fmt.Errorf("batch quality failed: %d failed, %d errored; report: %s", result.Failed, result.Errored, result.Report)
}

func formatBatchQualitySummary(result batchRunResult) string {
	return fmt.Sprintf("quality: %d passed, %d warnings, %d failed, %d errored", result.Passed, result.Warnings, result.Failed, result.Errored)
}

func renderBatchMarkdownReport(result batchRunResult) string {
	var body strings.Builder
	body.WriteString("# SVG Logo Batch Report\n\n")
	body.WriteString(fmt.Sprintf("- Input: `%s`\n", result.InputDir))
	body.WriteString(fmt.Sprintf("- Output: `%s`\n", result.OutputDir))
	body.WriteString(fmt.Sprintf("- %s\n", formatBatchQualitySummary(result)))
	if result.ContactSheet != "" {
		body.WriteString(fmt.Sprintf("- Contact sheet: `%s`\n", result.ContactSheet))
	}
	body.WriteString("\n")
	body.WriteString("## Check Breakdown\n\n")
	body.WriteString(renderBatchCheckBreakdown(result))
	body.WriteString("\n")
	body.WriteString("## Items\n\n")
	body.WriteString("| Status | Source | Colors | Bg | Color | Coverage | RMSE | Paths | Subpaths | Failed / Warning Checks | Visual | SVG |\n")
	body.WriteString("| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- | --- | --- |\n")
	items := append([]batchItemResult(nil), result.Items...)
	sort.SliceStable(items, func(i int, j int) bool {
		left := batchReportStatus(items[i])
		right := batchReportStatus(items[j])
		if batchReportStatusRank(left) != batchReportStatusRank(right) {
			return batchReportStatusRank(left) < batchReportStatusRank(right)
		}
		return filepath.Base(items[i].Source) < filepath.Base(items[j].Source)
	})
	for _, item := range items {
		status := batchReportStatus(item)
		checks := batchReportChecks(item.Quality)
		if item.Error != "" {
			checks = item.Error
		}
		body.WriteString(fmt.Sprintf(
			"| %s | `%s` | %d | %.2f | %.2f | %.4f | %.6f | %d | %d | %s | %s | %s |\n",
			status,
			filepath.Base(item.Source),
			item.Metrics.Request.Colors,
			item.Metrics.BackgroundDelta,
			item.Metrics.ForegroundColorDelta,
			item.Metrics.ForegroundCoverageDelta,
			item.Metrics.RenderedRMSE,
			item.Metrics.SVGPaths,
			item.Metrics.SVGSubpaths,
			checks,
			batchReportLink("visual-comparison.png", item.VisualComparison),
			batchReportLink("output.svg", item.OutputSVG),
		))
	}
	body.WriteString("\n")
	body.WriteString("Review order: failed items first, then warnings. Open `visual-comparison.png` to inspect source, rendered SVG, and amplified diff side by side.\n")
	return body.String()
}

func renderBatchCheckBreakdown(result batchRunResult) string {
	counts := map[string]int{}
	for _, item := range result.Items {
		for _, check := range item.Quality.Checks {
			if check.Status != "fail" && check.Status != "warn" {
				continue
			}
			counts[check.Name+":"+check.Status]++
		}
	}
	if len(counts) == 0 {
		return "- none\n"
	}
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i int, j int) bool {
		if counts[keys[i]] != counts[keys[j]] {
			return counts[keys[i]] > counts[keys[j]]
		}
		return keys[i] < keys[j]
	})
	var body strings.Builder
	body.WriteString("| Check | Count |\n")
	body.WriteString("| --- | ---: |\n")
	for _, key := range keys {
		body.WriteString(fmt.Sprintf("| %s | %d |\n", key, counts[key]))
	}
	return body.String()
}

func batchReportStatus(item batchItemResult) string {
	if item.Error != "" {
		return "error"
	}
	if item.Quality.Status != "" {
		return item.Quality.Status
	}
	return "unknown"
}

func batchReportStatusRank(status string) int {
	switch status {
	case "fail":
		return 0
	case "warn":
		return 1
	case "error":
		return 2
	case "unknown":
		return 3
	case "pass":
		return 4
	default:
		return 3
	}
}

func batchReportChecks(quality logoQualityResult) string {
	if len(quality.Checks) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(quality.Checks))
	for _, check := range quality.Checks {
		if check.Status != "fail" && check.Status != "warn" {
			continue
		}
		parts = append(parts, batchReportCheckSummary(check))
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, "<br>")
}

func batchReportCheckSummary(check logoQualityCheck) string {
	limit := check.FailAt
	if check.Status == "warn" && check.WarnAt > 0 {
		limit = check.WarnAt
	}
	return fmt.Sprintf("%s:%s(%.4f/%.4f)", check.Name, check.Status, check.Value, limit)
}

func batchReportLink(label string, path string) string {
	if path == "" {
		return "-"
	}
	return fmt.Sprintf("[%s](%s)", label, path)
}

func runBatch(ctx context.Context, inputDir string, outputDir string) (batchRunResult, error) {
	inputDir = strings.TrimSpace(inputDir)
	if inputDir == "" {
		return batchRunResult{}, errors.New("batch input directory is required")
	}
	if outputDir == "" {
		outputDir = filepath.Join("demos", "svg", "outputs", "batch-"+time.Now().Format("20060102-150405"))
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return batchRunResult{}, err
	}
	paths, err := collectBatchImagePaths(inputDir)
	if err != nil {
		return batchRunResult{}, err
	}
	previousOutputRoot := os.Getenv("SVG_DEMO_OUTPUTS")
	if err := os.Setenv("SVG_DEMO_OUTPUTS", outputDir); err != nil {
		return batchRunResult{}, err
	}
	defer func() {
		if previousOutputRoot == "" {
			_ = os.Unsetenv("SVG_DEMO_OUTPUTS")
			return
		}
		_ = os.Setenv("SVG_DEMO_OUTPUTS", previousOutputRoot)
	}()
	result := batchRunResult{InputDir: inputDir, OutputDir: outputDir, Items: make([]batchItemResult, 0, len(paths))}
	visuals := make([]string, 0, len(paths))
	for _, path := range paths {
		item := batchItemResult{Source: path}
		job, err := runJob(ctx, vectorizeRequest{FilePath: path, Mode: "cleanLogo"})
		if err != nil {
			item.Error = err.Error()
			result.Errored++
			result.Items = append(result.Items, item)
			continue
		}
		item.JobID = job.ID
		item.Metrics = job.Metrics
		item.Quality = assessLogoQuality(job.Metrics)
		switch item.Quality.Status {
		case "pass":
			result.Passed++
		case "warn":
			result.Warnings++
		default:
			result.Failed++
		}
		item.VisualComparison = artifactPathByName(job.Artifacts, "visual-comparison.png")
		item.OutputSVG = artifactPathByName(job.Artifacts, "output.svg")
		if item.VisualComparison != "" {
			visuals = append(visuals, item.VisualComparison)
		}
		result.Items = append(result.Items, item)
	}
	if len(visuals) > 0 {
		contactSheetPath := filepath.Join(outputDir, "contact-sheet.png")
		if err := renderBatchContactSheet(ctx, visuals, contactSheetPath); err != nil {
			return result, err
		}
		result.ContactSheet = contactSheetPath
	}
	result.Report = filepath.Join(outputDir, "batch-report.md")
	return result, nil
}

func assessLogoQuality(metrics metricsResult) logoQualityResult {
	pathFailLimit, pathWarnLimit := logoPathLimits(metrics)
	subpathFailLimit, subpathWarnLimit := logoSubpathLimits(metrics)
	result := logoQualityResult{
		Status: "pass",
		Checks: []logoQualityCheck{
			logoQualityCheckValue("backgroundDelta", "主体背景不漂移", metrics.BackgroundDelta, 0, 0, "SVG 背景 fill 应与源图边框主色一致"),
			logoQualityCheckValue("foregroundColorDelta", "主体颜色不漂移", metrics.ForegroundColorDelta, 14, 24, "源图与 SVG 回渲染的主要前景色不应明显偏色"),
			logoQualityCheckValue("foregroundCoverageDelta", "轮廓/文字覆盖不漂移", metrics.ForegroundCoverageDelta, logoCoverageWarnLimit(metrics), 0.04, "前景覆盖变化过大会导致轮廓变粗、变细或文字丢失"),
			logoQualityCheckValue("renderedRMSE", "整体视觉相似度", metrics.RenderedRMSE, logoRMSEWarnLimit(metrics), logoRMSEFailLimit(metrics), "回渲染差异过大通常意味着颜色、文字或边缘发生明显变化"),
			logoQualityCheckValue("svgPaths", "SVG 路径数量", float64(metrics.SVGPaths), pathWarnLimit, pathFailLimit, "路径过多通常意味着输出碎块过多"),
			logoQualityCheckValue("svgSubpaths", "SVG 子路径数量", float64(metrics.SVGSubpaths), subpathWarnLimit, subpathFailLimit, "子路径过多通常意味着边缘毛刺或小碎块过多"),
		},
	}
	if metrics.HasDarkLightTextTintBleedMetric {
		result.Checks = append(result.Checks, logoQualityCheckValue("darkLightTextTintBleedRatio", "暗底浅字边缘不串色", metrics.DarkLightTextTintBleedRatio, 0.005, 0.01, "暗色背景上的白/灰文字边缘不应漂到蓝色或青色品牌层"))
	}
	for _, check := range result.Checks {
		switch check.Status {
		case "fail":
			result.Failed++
		case "warn":
			result.Warnings++
		}
	}
	if result.Failed > 0 {
		result.Status = "fail"
	} else if result.Warnings > 0 {
		result.Status = "warn"
	}
	return result
}

func logoQualityCheckValue(name string, label string, value float64, warnAt float64, failAt float64, message string) logoQualityCheck {
	status := "pass"
	if value > failAt {
		status = "fail"
	} else if warnAt > 0 && value > warnAt {
		status = "warn"
	}
	return logoQualityCheck{
		Name:    name,
		Label:   label,
		Status:  status,
		Value:   value,
		WarnAt:  warnAt,
		FailAt:  failAt,
		Message: message,
	}
}

func logoRMSEFailLimit(metrics metricsResult) float64 {
	if isDenseMicroTextQualityCase(metrics) {
		return 0.075
	}
	if metrics.Request.Colors >= 32 {
		return 0.055
	}
	return 0.05
}

func logoCoverageWarnLimit(metrics metricsResult) float64 {
	if isCleanComplexArtworkQualityCase(metrics) {
		return 0.035
	}
	return 0.03
}

func logoRMSEWarnLimit(metrics metricsResult) float64 {
	if isDenseMicroTextQualityCase(metrics) {
		return 0.07
	}
	if metrics.Request.Colors >= 32 {
		return 0.05
	}
	if metrics.Request.Colors >= 16 {
		return 0.045
	}
	return 0.04
}

func logoPathLimits(metrics metricsResult) (fail float64, warn float64) {
	if metrics.Request.Colors >= 32 {
		return 14, 10
	}
	return 12, 8
}

func logoSubpathLimits(metrics metricsResult) (fail float64, warn float64) {
	if isDenseMicroTextQualityCase(metrics) {
		return 320, 280
	}
	if isCleanComplexArtworkQualityCase(metrics) {
		return 220, 170
	}
	if metrics.Request.Colors >= 32 {
		return 260, 180
	}
	return 180, 120
}

func isDenseMicroTextQualityCase(metrics metricsResult) bool {
	return metrics.Request.Colors == 8 &&
		metrics.SVGPaths <= 8 &&
		metrics.ForegroundColorDelta <= 8 &&
		metrics.ForegroundCoverageDelta <= 0.025 &&
		metrics.SVGSubpaths >= 180
}

func isCleanComplexArtworkQualityCase(metrics metricsResult) bool {
	return metrics.BackgroundDelta == 0 &&
		metrics.ForegroundColorDelta <= 8 &&
		metrics.ForegroundCoverageDelta <= 0.035 &&
		metrics.RenderedRMSE <= 0.035 &&
		metrics.SVGPaths <= 6
}

func collectBatchImagePaths(root string) ([]string, error) {
	var paths []string
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if isBatchImagePath(path) {
			paths = append(paths, path)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}

func isBatchImagePath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp":
		return true
	default:
		return false
	}
}

func artifactPathByName(artifacts []artifact, name string) string {
	for _, item := range artifacts {
		if item.Name == name {
			return item.Path
		}
	}
	return ""
}

func renderBatchContactSheet(ctx context.Context, visualPaths []string, outputPath string) error {
	magickPath, err := requireCommand("magick")
	if err != nil {
		return err
	}
	args := append([]string{}, visualPaths...)
	args = append(args, "-resize", "1800x900>", "-append", outputPath)
	return runCommand(ctx, magickPath, args...)
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
	mode := strings.TrimSpace(input.Mode)
	if strings.EqualFold(mode, "cleanLogo") {
		input = applyCleanLogoDefaults(input)
	}
	if strings.EqualFold(mode, "illustration") {
		input = applyIllustrationDefaults(input)
	}
	if strings.EqualFold(mode, "colorMask") {
		return runColorMaskJob(ctx, input, data)
	}
	if strings.EqualFold(mode, "layeredRibbon") {
		return runLayeredRibbonJob(ctx, input, data)
	}
	if strings.EqualFold(mode, "backendLogo") {
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
	renderedMetricsPath := filepath.Join(jobDir, "rendered-metrics.png")
	visualDiffPath := filepath.Join(jobDir, "visual-diff.png")
	visualComparisonPath := filepath.Join(jobDir, "visual-comparison.png")
	if err := writeInputPNG(data, sourcePath); err != nil {
		return jobResult{}, err
	}

	longEdge := input.LongEdge
	if longEdge <= 0 {
		longEdge = defaultLongEdge
	}
	colors := resolveLogoColors(ctx, magickPath, sourcePath, input)
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
	if err := restoreLightNeutralAccents(normalizedPath, quantizedPath); err != nil {
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
	preservePaletteLayers := strings.EqualFold(strings.TrimSpace(input.Mode), "illustration")
	layers := buildTraceLayers(palette, width*height, background.Keys, mergeDistance, mergeHueDistance, mergeLightness, mergeSaturation, preservePaletteLayers)
	if len(layers) == 0 {
		return jobResult{}, errors.New("no traceable color layers")
	}
	sort.SliceStable(layers, func(i, j int) bool {
		return luma(layers[i].R, layers[i].G, layers[i].B) > luma(layers[j].R, layers[j].G, layers[j].B)
	})
	if sourceImage, _, _, err := readPNGImage(normalizedPath); err == nil {
		sourceBackground := borderBackgroundColor(sourceImage)
		background.Hex = hexColor(sourceBackground.R, sourceBackground.G, sourceBackground.B)
		background.R = sourceBackground.R
		background.G = sourceBackground.G
		background.B = sourceBackground.B
	}

	var body strings.Builder
	body.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	body.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`, width, height, width, height) + "\n")
	body.WriteString(fmt.Sprintf(`<rect width="100%%" height="100%%" fill="%s"/>`, background.Hex) + "\n")

	metrics := metricsResult{SourceColors: sourceColors, QuantizedColors: quantizedColors, Background: background.Hex, Request: resolvedVectorizeParams(input, colors, longEdge, minComponentRatio, maxHoleRatio, mergeDistance, mergeHueDistance, mergeLightness, mergeSaturation, lightMinAreaRatio)}
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
	if err := renderSVGForMetrics(ctx, magickPath, outputSVGPath, renderedMetricsPath, width, height); err != nil {
		return jobResult{}, err
	}
	if err := fillQualityMetrics(ctx, magickPath, normalizedPath, renderedMetricsPath, background.Hex, &metrics); err != nil {
		return jobResult{}, err
	}
	if err := renderVisualComparison(ctx, magickPath, normalizedPath, renderedMetricsPath, visualDiffPath, visualComparisonPath); err != nil {
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
			makeArtifact(jobID, "rendered-metrics.png", renderedMetricsPath),
			makeArtifact(jobID, "visual-diff.png", visualDiffPath),
			makeArtifact(jobID, "visual-comparison.png", visualComparisonPath),
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
	metrics := metricsResult{SVGBytes: int64(result.Bytes), Request: vectorizeParams{Mode: "backendLogo"}}
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

func applyCleanLogoDefaults(input vectorizeRequest) vectorizeRequest {
	input.Mode = "cleanLogo"
	if input.LongEdge <= 0 {
		input.LongEdge = 4096
	}
	if input.MinComponentRatio <= 0 {
		input.MinComponentRatio = 0.00005
	}
	if input.MaxHoleRatio <= 0 {
		input.MaxHoleRatio = 0.00008
	}
	if input.MergeDistance <= 0 {
		input.MergeDistance = 64
	}
	if input.MergeHueDistance <= 0 {
		input.MergeHueDistance = 16
	}
	if input.MergeLightness <= 0 {
		input.MergeLightness = 80
	}
	if input.MergeSaturation <= 0 {
		input.MergeSaturation = 0.55
	}
	if input.LightMinAreaRatio <= 0 {
		input.LightMinAreaRatio = 0.004
	}
	return input
}

func applyIllustrationDefaults(input vectorizeRequest) vectorizeRequest {
	input.Mode = "illustration"
	if input.Colors <= 0 {
		input.Colors = 96
	}
	if input.LongEdge <= 0 {
		input.LongEdge = 4096
	}
	if input.MinComponentRatio <= 0 {
		input.MinComponentRatio = 0.000005
	}
	if input.MaxHoleRatio <= 0 {
		input.MaxHoleRatio = 0.00001
	}
	if input.MergeDistance <= 0 {
		input.MergeDistance = 18
	}
	if input.MergeHueDistance <= 0 {
		input.MergeHueDistance = 8
	}
	if input.MergeLightness <= 0 {
		input.MergeLightness = 32
	}
	if input.MergeSaturation <= 0 {
		input.MergeSaturation = 0.3
	}
	if input.LightMinAreaRatio <= 0 {
		input.LightMinAreaRatio = 0.0005
	}
	if input.RemoveSpeckles == nil {
		value := false
		input.RemoveSpeckles = &value
	}
	if input.FillSmallHoles == nil {
		value := false
		input.FillSmallHoles = &value
	}
	return input
}

func resolveLogoColors(ctx context.Context, magickPath string, sourcePath string, input vectorizeRequest) int {
	colors := input.Colors
	if colors <= 0 {
		if strings.EqualFold(strings.TrimSpace(input.Mode), "cleanLogo") {
			colors = estimateCleanLogoColors(ctx, magickPath, sourcePath)
		} else {
			colors = defaultColors
		}
	}
	if colors < 2 {
		colors = 2
	}
	return colors
}

func estimateCleanLogoColors(ctx context.Context, magickPath string, sourcePath string) int {
	colors, err := readLogoHistogram(ctx, magickPath, sourcePath)
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

	background, hasBackground := dominantLogoHistogramColor(colors)
	borderBackground, hasBorderBackground := readSourceCornerColor(ctx, magickPath, sourcePath)
	hasDarkBackground := hasBorderBackground && luma(borderBackground.R, borderBackground.G, borderBackground.B) < 160
	if !hasBorderBackground {
		hasDarkBackground = hasBackground && luma(background.R, background.G, background.B) < 160
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
		if hasBackground && histogramColorDistance(item.R, item.G, item.B, background.R, background.G, background.B) <= 16 {
			continue
		}
		hue, sat := hueAndSaturation(item.R, item.G, item.B)
		if sat < 0.18 {
			if ratio >= 0.003 && sat >= 0.08 && luma(item.R, item.G, item.B) >= 180 && luma(item.R, item.G, item.B) <= 240 && max3(item.R, item.G, item.B)-min3(item.R, item.G, item.B) > 30 {
				paleTintedAccentRatio += ratio
			}
			if ratio >= 0.001 && isLightNeutralForeground(item.R, item.G, item.B) {
				lightNeutralForegroundRatio += ratio
				if luma(item.R, item.G, item.B) >= 235 {
					brightNeutralForegroundRatio += ratio
				}
			}
			if ratio >= 0.001 && isDarkNeutralForeground(item.R, item.G, item.B) {
				darkNeutralForegroundRatio += ratio
			}
			if ratio >= 0.001 && isMidNeutralForeground(item.R, item.G, item.B) {
				midNeutralForegroundRatio += ratio
			}
			continue
		}
		luma := luma(item.R, item.G, item.B)
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
			if hueDistance(hue, groupHue) <= 18 {
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

func dominantLogoHistogramColor(colors []logoHistogramColor) (logoHistogramColor, bool) {
	if len(colors) == 0 {
		return logoHistogramColor{}, false
	}
	background := colors[0]
	for _, item := range colors[1:] {
		if item.Count > background.Count {
			background = item
		}
	}
	return background, true
}

func isLightNeutralForeground(r uint8, g uint8, b uint8) bool {
	if luma(r, g, b) < 180 || saturation(r, g, b) > 0.12 {
		return false
	}
	return max3(r, g, b)-min3(r, g, b) <= 30
}

func isDarkNeutralForeground(r uint8, g uint8, b uint8) bool {
	if luma(r, g, b) > 95 || saturation(r, g, b) > 0.12 {
		return false
	}
	return max3(r, g, b)-min3(r, g, b) <= 30
}

func isMidNeutralForeground(r uint8, g uint8, b uint8) bool {
	value := luma(r, g, b)
	if value <= 95 || value >= 180 || saturation(r, g, b) > 0.12 {
		return false
	}
	return max3(r, g, b)-min3(r, g, b) <= 30
}

func readSourceCornerColor(ctx context.Context, magickPath string, sourcePath string) (logoHistogramColor, bool) {
	command := exec.CommandContext(ctx, magickPath, sourcePath, "-background", "white", "-alpha", "remove", "-depth", "8", "-format", "%[hex:p{0,0}]", "info:-")
	output, err := command.Output()
	if err != nil {
		return logoHistogramColor{}, false
	}
	value := strings.TrimSpace(string(output))
	if len(value) < 6 {
		return logoHistogramColor{}, false
	}
	r, err := strconv.ParseUint(value[0:2], 16, 8)
	if err != nil {
		return logoHistogramColor{}, false
	}
	g, err := strconv.ParseUint(value[2:4], 16, 8)
	if err != nil {
		return logoHistogramColor{}, false
	}
	b, err := strconv.ParseUint(value[4:6], 16, 8)
	if err != nil {
		return logoHistogramColor{}, false
	}
	return logoHistogramColor{R: uint8(r), G: uint8(g), B: uint8(b)}, true
}

func histogramColorDistance(ar uint8, ag uint8, ab uint8, br uint8, bg uint8, bb uint8) float64 {
	dr := float64(int(ar) - int(br))
	dg := float64(int(ag) - int(bg))
	db := float64(int(ab) - int(bb))
	return math.Sqrt(dr*dr + dg*dg + db*db)
}

func readLogoHistogram(ctx context.Context, magickPath string, sourcePath string) ([]logoHistogramColor, error) {
	command := exec.CommandContext(
		ctx,
		magickPath,
		sourcePath,
		"-background", "white",
		"-alpha", "remove",
		"-resize", "512x512>",
		"-depth", "8",
		"-colors", "32",
		"-format", "%c",
		"histogram:info:-",
	)
	output, err := command.Output()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(output), "\n")
	result := make([]logoHistogramColor, 0, len(lines))
	for _, line := range lines {
		match := histogramLinePattern.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		count, err := strconv.Atoi(match[1])
		if err != nil {
			continue
		}
		r, ok := parseHistogramChannel(match[2])
		if !ok {
			continue
		}
		g, ok := parseHistogramChannel(match[3])
		if !ok {
			continue
		}
		b, ok := parseHistogramChannel(match[4])
		if !ok {
			continue
		}
		result = append(result, logoHistogramColor{Count: count, R: r, G: g, B: b})
	}
	return result, nil
}

func parseHistogramChannel(value string) (uint8, bool) {
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

func isLogoBackgroundColor(r uint8, g uint8, b uint8) bool {
	if luma(r, g, b) > 245 && saturation(r, g, b) < 0.12 {
		return true
	}
	if luma(r, g, b) > 232 && saturation(r, g, b) < 0.08 {
		return true
	}
	return false
}

func resolvedVectorizeParams(
	input vectorizeRequest,
	colors int,
	longEdge int,
	minComponentRatio float64,
	maxHoleRatio float64,
	mergeDistance float64,
	mergeHueDistance float64,
	mergeLightness float64,
	mergeSaturation float64,
	lightMinAreaRatio float64,
) vectorizeParams {
	mode := strings.TrimSpace(input.Mode)
	if mode == "" {
		mode = "colorMask"
	}
	return vectorizeParams{
		Mode:              mode,
		Colors:            colors,
		LongEdge:          longEdge,
		MinComponentRatio: minComponentRatio,
		MaxHoleRatio:      maxHoleRatio,
		MergeDistance:     mergeDistance,
		MergeHueDistance:  mergeHueDistance,
		MergeLightness:    mergeLightness,
		MergeSaturation:   mergeSaturation,
		LightMinAreaRatio: lightMinAreaRatio,
		MaskCloseRadius:   input.MaskCloseRadius,
		LightDilateRadius: input.LightDilateRadius,
		DarkDilateRadius:  input.DarkDilateRadius,
	}
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
		"-background", "white",
		"-alpha", "remove",
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

func restoreLightNeutralAccents(normalizedPath string, quantizedPath string) error {
	normalized, width, height, err := readPNGImage(normalizedPath)
	if err != nil {
		return err
	}
	quantized, quantizedWidth, quantizedHeight, err := readPNGImage(quantizedPath)
	if err != nil {
		return err
	}
	if width != quantizedWidth || height != quantizedHeight {
		return nil
	}
	background := rgbColorFromImage(normalized.At(normalized.Bounds().Min.X, normalized.Bounds().Min.Y))
	restoreLight := luma(background.R, background.G, background.B) < 160 && lightNeutralCoverage(normalized) <= 0.08
	darkCoverage := darkNeutralAccentCoverage(normalized)
	restoreDark := luma(background.R, background.G, background.B) >= 160 && darkCoverage > 0 && darkCoverage <= 0.08
	midCoverage := midNeutralAccentCoverage(normalized)
	restoreMid := luma(background.R, background.G, background.B) >= 160 && midCoverage > 0 && midCoverage <= 0.08
	if !restoreLight && !restoreDark && !restoreMid {
		return nil
	}
	darkAccent := darkestDarkNeutralAccent(normalized)
	midAccent := darkestMidNeutralAccent(normalized)
	out := image.NewRGBA(image.Rect(0, 0, width, height))
	normalizedBounds := normalized.Bounds()
	quantizedBounds := quantized.Bounds()
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			source := rgbColorFromImage(normalized.At(normalizedBounds.Min.X+x, normalizedBounds.Min.Y+y))
			if restoreLight && isLightNeutralForeground(source.R, source.G, source.B) && luma(source.R, source.G, source.B) >= 160 {
				out.Set(x, y, color.RGBA{R: source.R, G: source.G, B: source.B, A: 0xff})
				continue
			}
			if restoreDark && isDarkNeutralAccent(source.R, source.G, source.B) {
				out.Set(x, y, color.RGBA{R: darkAccent.R, G: darkAccent.G, B: darkAccent.B, A: 0xff})
				continue
			}
			if restoreMid && isMidNeutralForeground(source.R, source.G, source.B) {
				out.Set(x, y, color.RGBA{R: midAccent.R, G: midAccent.G, B: midAccent.B, A: 0xff})
				continue
			}
			out.Set(x, y, quantized.At(quantizedBounds.Min.X+x, quantizedBounds.Min.Y+y))
		}
	}
	return writePNG(quantizedPath, out)
}

func lightNeutralCoverage(img image.Image) float64 {
	bounds := img.Bounds()
	total := bounds.Dx() * bounds.Dy()
	if total == 0 {
		return 0
	}
	count := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			source := rgbColorFromImage(img.At(x, y))
			if isLightNeutralForeground(source.R, source.G, source.B) && luma(source.R, source.G, source.B) >= 160 {
				count++
			}
		}
	}
	return float64(count) / float64(total)
}

func darkNeutralAccentCoverage(img image.Image) float64 {
	bounds := img.Bounds()
	total := bounds.Dx() * bounds.Dy()
	if total == 0 {
		return 0
	}
	count := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			source := rgbColorFromImage(img.At(x, y))
			if isDarkNeutralAccent(source.R, source.G, source.B) {
				count++
			}
		}
	}
	return float64(count) / float64(total)
}

func midNeutralAccentCoverage(img image.Image) float64 {
	bounds := img.Bounds()
	total := bounds.Dx() * bounds.Dy()
	if total == 0 {
		return 0
	}
	count := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			source := rgbColorFromImage(img.At(x, y))
			if isMidNeutralForeground(source.R, source.G, source.B) {
				count++
			}
		}
	}
	return float64(count) / float64(total)
}

func darkestDarkNeutralAccent(img image.Image) rgbColor {
	bounds := img.Bounds()
	best := rgbColor{R: 20, G: 20, B: 20}
	bestLuma := math.MaxFloat64
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			source := rgbColorFromImage(img.At(x, y))
			if !isDarkNeutralAccent(source.R, source.G, source.B) {
				continue
			}
			value := luma(source.R, source.G, source.B)
			if value < bestLuma {
				best = source
				bestLuma = value
			}
		}
	}
	if luma(best.R, best.G, best.B) < 16 {
		return rgbColor{R: 20, G: 20, B: 20}
	}
	return best
}

func darkestMidNeutralAccent(img image.Image) rgbColor {
	bounds := img.Bounds()
	best := rgbColor{R: 119, G: 119, B: 119}
	bestLuma := math.MaxFloat64
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			source := rgbColorFromImage(img.At(x, y))
			if !isMidNeutralForeground(source.R, source.G, source.B) {
				continue
			}
			value := luma(source.R, source.G, source.B)
			if value < bestLuma {
				best = source
				bestLuma = value
			}
		}
	}
	return best
}

func isDarkNeutralAccent(r uint8, g uint8, b uint8) bool {
	if luma(r, g, b) > 125 || saturation(r, g, b) > 0.12 {
		return false
	}
	return max3(r, g, b)-min3(r, g, b) <= 30
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
	layer.Keys[backgroundKey] = struct{}{}
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
		return backgroundIsWhite && borderCount > 0
	}
	if borderCount <= 0 && backgroundIsWhite {
		return false
	}
	if luma(background.R, background.G, background.B) >= 180 &&
		luma(item.R, item.G, item.B) < 140 &&
		luma(background.R, background.G, background.B)-luma(item.R, item.G, item.B) > 60 {
		return false
	}
	if background.Count > 0 &&
		float64(item.Count)/float64(background.Count) >= 0.015 &&
		colorDistance(item, background) > 24 {
		return false
	}
	if borderCount > 0 && canMergeColor(item, background, mergeDistance, mergeHueDistance, mergeLightness*1.4, mergeSaturation) {
		return true
	}
	if borderCount <= 0 {
		lumaGap := math.Abs(luma(item.R, item.G, item.B) - luma(background.R, background.G, background.B))
		if lumaGap <= 32 && rgbDistance(rgbColor{R: item.R, G: item.G, B: item.B}, rgbColor{R: background.R, G: background.G, B: background.B}) <= mergeDistance*0.8 {
			return true
		}
		return false
	}
	if saturation(background.R, background.G, background.B) < 0.12 {
		return false
	}
	if math.Abs(saturation(item.R, item.G, item.B)-saturation(background.R, background.G, background.B)) > mergeSaturation {
		return false
	}
	return sameFunctionalHue(item.R, item.G, item.B, background.R, background.G, background.B, mergeHueDistance*2, mergeLightness*1.4)
}

func buildTraceLayers(palette []paletteColor, totalPixels int, backgroundKeys map[uint32]struct{}, mergeDistance float64, mergeHueDistance float64, mergeLightness float64, mergeSaturation float64, preservePaletteLayers bool) []traceLayer {
	minPixels := int(math.Max(16, float64(totalPixels)*0.00002))
	layers := make([]traceLayer, 0, len(palette))
	for _, item := range palette {
		if item.Count < minPixels {
			continue
		}
		if _, ok := backgroundKeys[rgbKey(item.R, item.G, item.B)]; ok {
			continue
		}
		if !preservePaletteLayers && isNeutralNoise(item, totalPixels) {
			continue
		}
		if preservePaletteLayers {
			layers = append(layers, traceLayer{
				Hex:   item.Hex,
				R:     item.R,
				G:     item.G,
				B:     item.B,
				Count: item.Count,
				Keys:  map[uint32]struct{}{rgbKey(item.R, item.G, item.B): {}},
			})
			continue
		}
		merged := false
		for index := range layers {
			if isLightNeutralForeground(item.R, item.G, item.B) && luma(item.R, item.G, item.B)-luma(layers[index].R, layers[index].G, layers[index].B) > 40 {
				continue
			}
			if isMidNeutralForeground(item.R, item.G, item.B) != isMidNeutralForeground(layers[index].R, layers[index].G, layers[index].B) {
				continue
			}
			if !canMergeColor(item, layers[index], mergeDistance, mergeHueDistance, mergeLightness, mergeSaturation) {
				continue
			}
			layer := &layers[index]
			layer.Keys[rgbKey(item.R, item.G, item.B)] = struct{}{}
			layer.Count += item.Count
			if shouldUseMergedLayerColor(item.R, item.G, item.B, layer.R, layer.G, layer.B) || item.Count > layer.Count-item.Count {
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
	if !preservePaletteLayers {
		layers = mergeTintTraceLayers(layers, mergeHueDistance, mergeLightness)
	}
	return layers
}

func shouldUseMergedLayerColor(ar uint8, ag uint8, ab uint8, br uint8, bg uint8, bb uint8) bool {
	return isLightNeutralForeground(ar, ag, ab) &&
		isLightNeutralForeground(br, bg, bb) &&
		luma(ar, ag, ab) > luma(br, bg, bb)
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
			if isLightNeutralForeground(source.R, source.G, source.B) && luma(source.R, source.G, source.B)-luma(target.R, target.G, target.B) > 40 {
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
	if isLightNeutralForeground(layer.R, layer.G, layer.B) {
		return fallback
	}
	if isMidNeutralForeground(layer.R, layer.G, layer.B) {
		return fallback
	}
	if luma(layer.R, layer.G, layer.B) >= 120 && saturation(layer.R, layer.G, layer.B) < 0.12 {
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

func renderSVGForMetrics(ctx context.Context, magickPath string, svgPath string, outputPath string, width int, height int) error {
	return runCommand(ctx, magickPath, "-background", "white", svgPath, "-resize", fmt.Sprintf("%dx%d!", width, height), "-colorspace", "sRGB", "-strip", outputPath)
}

func renderVisualComparison(ctx context.Context, magickPath string, sourcePath string, renderedPath string, diffPath string, outputPath string) error {
	if err := runCommand(ctx, magickPath, sourcePath, renderedPath, "-compose", "difference", "-composite", "-auto-level", diffPath); err != nil {
		return err
	}
	return runCommand(ctx, magickPath, sourcePath, renderedPath, diffPath, "-resize", "1200x800>", "+append", outputPath)
}

func fillQualityMetrics(ctx context.Context, magickPath string, sourcePath string, renderedPath string, backgroundHex string, metrics *metricsResult) error {
	sourceImage, _, _, err := readPNGImage(sourcePath)
	if err != nil {
		return err
	}
	renderedImage, _, _, err := readPNGImage(renderedPath)
	if err != nil {
		return err
	}
	sourceBackground := borderBackgroundColor(sourceImage)
	svgBackground, ok := parseHexRGB(backgroundHex)
	if !ok {
		return fmt.Errorf("invalid background color: %s", backgroundHex)
	}
	metrics.BackgroundDelta = rgbDistance(sourceBackground, svgBackground)
	if sourceForeground, renderedForeground, delta, ok := foregroundColorDrift(sourceImage, renderedImage, sourceBackground); ok {
		metrics.SourceForegroundColor = hexColor(sourceForeground.R, sourceForeground.G, sourceForeground.B)
		metrics.RenderedForegroundColor = hexColor(renderedForeground.R, renderedForeground.G, renderedForeground.B)
		metrics.ForegroundColorDelta = delta
	}
	metrics.SourceForegroundCoverage = foregroundCoverage(sourceImage, sourceBackground)
	metrics.RenderedForegroundCoverage = foregroundCoverage(renderedImage, sourceBackground)
	metrics.ForegroundCoverageDelta = math.Abs(metrics.SourceForegroundCoverage - metrics.RenderedForegroundCoverage)
	if ratio, ok := darkLightTextTintBleedRatio(sourceImage, renderedImage, sourceBackground); ok {
		metrics.DarkLightTextTintBleedRatio = ratio
		metrics.HasDarkLightTextTintBleedMetric = true
	}
	rmse, err := renderedRMSE(ctx, magickPath, sourcePath, renderedPath)
	if err != nil {
		return err
	}
	metrics.RenderedRMSE = rmse
	return nil
}

func renderedRMSE(ctx context.Context, magickPath string, sourcePath string, renderedPath string) (float64, error) {
	commandCtx, cancel := context.WithTimeout(ctx, 180*time.Second)
	defer cancel()
	cmd := exec.CommandContext(commandCtx, magickPath, "compare", "-metric", "RMSE", sourcePath, renderedPath, "null:")
	output, err := cmd.CombinedOutput()
	if err != nil && len(output) == 0 {
		return 0, fmt.Errorf("compare failed: %w", err)
	}
	match := rmseMetricPattern.FindStringSubmatch(string(output))
	if match == nil {
		return 0, fmt.Errorf("compare did not return RMSE: %s", strings.TrimSpace(string(output)))
	}
	value, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return 0, err
	}
	return value, nil
}

func foregroundColorDelta(source image.Image, rendered image.Image, background rgbColor) (float64, bool) {
	_, _, delta, ok := foregroundColorDrift(source, rendered, background)
	return delta, ok
}

func foregroundColorDrift(source image.Image, rendered image.Image, background rgbColor) (rgbColor, rgbColor, float64, bool) {
	sourceForeground, ok := dominantForegroundColor(source, background)
	if !ok {
		return rgbColor{}, rgbColor{}, 0, false
	}
	renderedForeground, delta, ok := nearestForegroundColor(rendered, background, sourceForeground)
	if !ok {
		return rgbColor{}, rgbColor{}, 0, false
	}
	return sourceForeground, renderedForeground, delta, true
}

func dominantForegroundColor(img image.Image, background rgbColor) (rgbColor, bool) {
	bounds := img.Bounds()
	counts := make(map[rgbColor]int)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			rgb := rgbColorFromImage(img.At(x, y))
			if rgbDistance(rgb, background) <= 24 {
				continue
			}
			counts[quantizedQualityColor(rgb)]++
		}
	}
	best := rgbColor{}
	bestCount := 0
	for rgb, count := range counts {
		if count > bestCount {
			best = rgb
			bestCount = count
		}
	}
	return best, bestCount > 0
}

func nearestForegroundColor(img image.Image, background rgbColor, target rgbColor) (rgbColor, float64, bool) {
	bounds := img.Bounds()
	seen := make(map[rgbColor]struct{})
	best := rgbColor{}
	bestDistance := math.MaxFloat64
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			rgb := rgbColorFromImage(img.At(x, y))
			if rgbDistance(rgb, background) <= 24 {
				continue
			}
			quantized := quantizedQualityColor(rgb)
			if _, ok := seen[quantized]; ok {
				continue
			}
			seen[quantized] = struct{}{}
			distance := rgbDistance(target, quantized)
			if distance < bestDistance {
				best = quantized
				bestDistance = distance
			}
		}
	}
	return best, bestDistance, len(seen) > 0
}

func quantizedQualityColor(rgb rgbColor) rgbColor {
	return rgbColor{
		R: quantizedQualityChannel(rgb.R),
		G: quantizedQualityChannel(rgb.G),
		B: quantizedQualityChannel(rgb.B),
	}
}

func quantizedQualityChannel(value uint8) uint8 {
	rounded := (int(value) + 4) / 8 * 8
	if rounded > 255 {
		return 255
	}
	return uint8(rounded)
}

func borderBackgroundColor(img image.Image) rgbColor {
	bounds := img.Bounds()
	counts := make(map[rgbColor]int)
	add := func(x int, y int) {
		counts[rgbColorFromImage(img.At(x, y))]++
	}
	for x := bounds.Min.X; x < bounds.Max.X; x++ {
		add(x, bounds.Min.Y)
		add(x, bounds.Max.Y-1)
	}
	for y := bounds.Min.Y + 1; y < bounds.Max.Y-1; y++ {
		add(bounds.Min.X, y)
		add(bounds.Max.X-1, y)
	}
	best := rgbColor{R: 255, G: 255, B: 255}
	bestCount := -1
	for rgb, count := range counts {
		if count > bestCount {
			best = rgb
			bestCount = count
		}
	}
	return best
}

func foregroundCoverage(img image.Image, background rgbColor) float64 {
	bounds := img.Bounds()
	total := bounds.Dx() * bounds.Dy()
	if total == 0 {
		return 0
	}
	foreground := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if rgbDistance(rgbColorFromImage(img.At(x, y)), background) > 24 {
				foreground++
			}
		}
	}
	return float64(foreground) / float64(total)
}

func darkLightTextTintBleedRatio(source image.Image, rendered image.Image, background rgbColor) (float64, bool) {
	if luma(background.R, background.G, background.B) >= 160 {
		return 0, false
	}
	bounds := source.Bounds().Intersect(rendered.Bounds())
	lightTextPixels := 0
	tintedPixels := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			sourceRGB := rgbColorFromImage(source.At(x, y))
			if !isLightTextPixelOnDarkBackground(sourceRGB, background) {
				continue
			}
			lightTextPixels++
			renderedRGB := rgbColorFromImage(rendered.At(x, y))
			if isBlueCyanTintPixel(renderedRGB) {
				tintedPixels++
			}
		}
	}
	if lightTextPixels == 0 {
		return 0, false
	}
	return float64(tintedPixels) / float64(lightTextPixels), true
}

func isLightTextPixelOnDarkBackground(rgb rgbColor, background rgbColor) bool {
	return rgbDistance(rgb, background) > 24 &&
		luma(rgb.R, rgb.G, rgb.B) >= 150 &&
		saturation(rgb.R, rgb.G, rgb.B) <= 0.24 &&
		max3(rgb.R, rgb.G, rgb.B)-min3(rgb.R, rgb.G, rgb.B) <= 58
}

func isBlueCyanTintPixel(rgb rgbColor) bool {
	hue, sat := hueAndSaturation(rgb.R, rgb.G, rgb.B)
	value := luma(rgb.R, rgb.G, rgb.B)
	return sat >= 0.35 && hue >= 170 && hue <= 230 && value >= 90 && value <= 240
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

func rgbColorFromImage(c color.Color) rgbColor {
	r, g, b := rgb8(c)
	return rgbColor{R: r, G: g, B: b}
}

func parseHexRGB(value string) (rgbColor, bool) {
	value = strings.TrimPrefix(strings.TrimSpace(value), "#")
	if len(value) != 6 {
		return rgbColor{}, false
	}
	r, err := strconv.ParseUint(value[0:2], 16, 8)
	if err != nil {
		return rgbColor{}, false
	}
	g, err := strconv.ParseUint(value[2:4], 16, 8)
	if err != nil {
		return rgbColor{}, false
	}
	b, err := strconv.ParseUint(value[4:6], 16, 8)
	if err != nil {
		return rgbColor{}, false
	}
	return rgbColor{R: uint8(r), G: uint8(g), B: uint8(b)}, true
}

func rgbDistance(a rgbColor, b rgbColor) float64 {
	dr := float64(int(a.R) - int(b.R))
	dg := float64(int(a.G) - int(b.G))
	db := float64(int(a.B) - int(b.B))
	return math.Sqrt(dr*dr + dg*dg + db*db)
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
	if isLightNeutralForeground(item.R, item.G, item.B) && luma(item.R, item.G, item.B) >= 220 {
		return false
	}
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
    input[type="range"] { width: 220px; accent-color: #04758a; }
    select { min-width: 180px; padding: 7px 10px; border: 1px solid #d0d7de; border-radius: 8px; background: #fff; font: inherit; }
    button { margin-top: 16px; padding: 8px 14px; cursor: pointer; }
    .row { display: flex; gap: 24px; flex-wrap: wrap; align-items: end; }
    .control { min-width: 240px; }
    .control-head { display: flex; align-items: baseline; justify-content: space-between; gap: 12px; }
    .value { color: #04758a; font-variant-numeric: tabular-nums; font-weight: 700; }
    .hint { color: #667085; font-size: 12px; margin-top: 4px; }
    .preview { display: grid; grid-template-columns: repeat(auto-fit, minmax(320px, 1fr)); gap: 20px; margin-top: 24px; }
    .box { border: 1px solid #ddd; padding: 12px; background: #fff; }
    img { max-width: 100%; background: white; }
    pre { white-space: pre-wrap; background: #f6f8fa; padding: 12px; overflow: auto; }
    .quality { display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 10px; margin: 12px 0 16px; }
    .quality-item { border: 1px solid #d0d7de; border-radius: 8px; padding: 10px 12px; background: #fff; }
    .quality-title { font-weight: 700; margin-bottom: 4px; }
    .quality-value { font-variant-numeric: tabular-nums; color: #344054; }
    .quality-pass { border-color: #9ad0a5; background: #f2fbf4; }
    .quality-warn { border-color: #f2bd72; background: #fff8ed; }
    .quality-missing { border-style: dashed; color: #667085; }
    .artifacts { display: flex; flex-wrap: wrap; gap: 8px; margin: 10px 0 18px; }
    .artifact-link { display: inline-flex; align-items: center; border: 1px solid #d0d7de; border-radius: 999px; padding: 6px 10px; color: #04758a; text-decoration: none; background: #fff; font-size: 13px; }
    .artifact-link:hover { border-color: #04758a; background: #f0fbfd; }
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
      <label>Mode</label>
      <select id="mode">
        <option value="cleanLogo" selected>cleanLogo - Logo 稳定预设</option>
        <option value="illustration">illustration - 插画/素材保真</option>
        <option value="colorMask">colorMask - 分层颜色遮罩</option>
        <option value="backendLogo">backendLogo - 当前线上后端</option>
        <option value="layeredRibbon">layeredRibbon - 分层对照</option>
      </select>
    </div>
    <div class="control" data-slider="colors" data-label="Colors" data-value="0" data-min="0" data-max="128" data-step="1" data-hint="0 为自动色数；越高越保留多色细节但更容易碎。"></div>
    <div class="control" data-slider="longEdge" data-label="Long edge" data-value="4096" data-min="512" data-max="8192" data-step="128" data-hint="归一化图片最长边，越大细节越多、耗时越长。"></div>
    <div class="control" data-slider="minComponentRatio" data-label="Min component ratio" data-value="0.00005" data-min="0" data-max="0.001" data-step="0.000001" data-hint="过滤小碎片，值越大删除越多小组件。"></div>
    <div class="control" data-slider="maxHoleRatio" data-label="Max hole ratio" data-value="0.00008" data-min="0" data-max="0.001" data-step="0.000001" data-hint="填补小孔洞，值越大越容易把小洞抹平。"></div>
    <div class="control" data-slider="mergeDistance" data-label="Merge distance" data-value="64" data-min="0" data-max="220" data-step="1" data-hint="近似颜色合并距离，调大可减少碎色块。"></div>
    <div class="control" data-slider="mergeHueDistance" data-label="Merge hue" data-value="16" data-min="0" data-max="180" data-step="1" data-hint="色相合并容忍度。"></div>
    <div class="control" data-slider="mergeLightness" data-label="Merge light" data-value="80" data-min="0" data-max="255" data-step="1" data-hint="明度合并容忍度。"></div>
    <div class="control" data-slider="mergeSaturation" data-label="Merge sat" data-value="0.55" data-min="0" data-max="1" data-step="0.01" data-hint="饱和度合并容忍度。"></div>
    <div class="control" data-slider="lightMinAreaRatio" data-label="Light min area" data-value="0.004" data-min="0" data-max="0.05" data-step="0.0005" data-hint="浅色层最小面积比例，用于抑制浅色噪点。"></div>
    <div class="control" data-slider="maskCloseRadius" data-label="Mask close" data-value="0" data-min="0" data-max="12" data-step="1" data-hint="遮罩闭运算半径，调大可连接断裂区域。"></div>
    <div class="control" data-slider="lightDilateRadius" data-label="Light dilate" data-value="0" data-min="0" data-max="12" data-step="1" data-hint="浅色层膨胀半径。"></div>
    <div class="control" data-slider="darkDilateRadius" data-label="Dark dilate" data-value="0" data-min="0" data-max="8" data-step="1" data-hint="深色层膨胀半径。"></div>
  </div>
  <button id="run">Vectorize now</button>
  <p id="status"></p>
  <div class="preview">
    <div class="box"><h3>Source</h3><img id="source" /></div>
    <div class="box"><h3>Preview</h3><img id="preview" /></div>
    <div class="box"><h3>Visual comparison</h3><img id="visualComparison" /></div>
  </div>
  <h3>Quality Check</h3>
  <div class="quality" id="quality"></div>
  <h3>Artifacts</h3>
  <div class="artifacts" id="artifacts"></div>
  <h3>Metrics</h3>
  <pre id="metrics"></pre>
</main>
<script>
const fileInput = document.getElementById('file');
const source = document.getElementById('source');
const modeInput = document.getElementById('mode');
const runButton = document.getElementById('run');
const statusEl = document.getElementById('status');
const previewEl = document.getElementById('preview');
const visualComparisonEl = document.getElementById('visualComparison');
const metricsEl = document.getElementById('metrics');
const qualityEl = document.getElementById('quality');
const artifactsEl = document.getElementById('artifacts');
let autoRunTimer = 0;
let runSerial = 0;
let requestRevision = 0;
let isVectorizing = false;
let rerunAfterCurrent = false;
let currentDataUrl = '';
const sliderIds = [
  'colors',
  'longEdge',
  'minComponentRatio',
  'maxHoleRatio',
  'mergeDistance',
  'mergeHueDistance',
  'mergeLightness',
  'mergeSaturation',
  'lightMinAreaRatio',
  'maskCloseRadius',
  'lightDilateRadius',
  'darkDilateRadius'
];
const modePresets = {
  cleanLogo: {
    colors: 0,
    longEdge: 4096,
    minComponentRatio: 0.00005,
    maxHoleRatio: 0.00008,
    mergeDistance: 64,
    mergeHueDistance: 16,
    mergeLightness: 80,
    mergeSaturation: 0.55,
    lightMinAreaRatio: 0.004,
    maskCloseRadius: 0,
    lightDilateRadius: 0,
    darkDilateRadius: 0
  },
  illustration: {
    colors: 96,
    longEdge: 4096,
    minComponentRatio: 0.000005,
    maxHoleRatio: 0.00001,
    mergeDistance: 18,
    mergeHueDistance: 8,
    mergeLightness: 32,
    mergeSaturation: 0.3,
    lightMinAreaRatio: 0.0005,
    maskCloseRadius: 0,
    lightDilateRadius: 0,
    darkDilateRadius: 0
  },
  colorMask: {
    colors: 16,
    longEdge: 4096,
    minComponentRatio: 0.00001,
    maxHoleRatio: 0.00004,
    mergeDistance: 48,
    mergeHueDistance: 16,
    mergeLightness: 80,
    mergeSaturation: 0.55,
    lightMinAreaRatio: 0.003,
    maskCloseRadius: 0,
    lightDilateRadius: 0,
    darkDilateRadius: 0
  }
};
const qualityChecks = [
  { key: 'backgroundDelta', label: '背景色', limit: 0, format: (value) => value.toFixed(2), suffix: ' / 0' },
  { key: 'foregroundColorDelta', label: '主体颜色', limit: 24, format: (value) => value.toFixed(2), suffix: ' / 24' },
  { key: 'foregroundCoverageDelta', label: '轮廓/文字覆盖', limit: 0.04, format: (value) => value.toFixed(4), suffix: ' / 0.04' },
  { key: 'renderedRMSE', label: '整体相似度 RMSE', limit: (metrics) => qualityRMSELimit(metrics), format: (value) => value.toFixed(6), suffix: (metrics) => ' / ' + qualityRMSELimit(metrics).toFixed(3) },
  { key: 'darkLightTextTintBleedRatio', label: '暗底浅字串色', limit: 0.01, format: (value) => value.toFixed(4), suffix: ' / 0.0100', enabled: (metrics) => metrics.hasDarkLightTextTintBleedMetric === true },
  { key: 'svgPaths', label: 'SVG paths', limit: (metrics) => qualityPathLimit(metrics), format: (value) => String(value), suffix: (metrics) => ' / ' + qualityPathLimit(metrics) },
  { key: 'svgSubpaths', label: 'SVG subpaths', limit: (metrics) => qualitySubpathLimit(metrics), format: (value) => String(value), suffix: (metrics) => ' / ' + qualitySubpathLimit(metrics) }
];
const preferredArtifacts = [
  'visual-comparison.png',
  'output.svg',
  'preview.png',
  'metrics.json',
  'rendered-metrics.png',
  'visual-diff.png'
];

document.querySelectorAll('[data-slider]').forEach((control) => {
  const id = control.dataset.slider;
  const value = control.dataset.value;
  control.innerHTML = [
    '<div class="control-head">',
      '<label for="' + id + '">' + control.dataset.label + '</label>',
      '<span class="value" id="' + id + 'Value">' + value + '</span>',
    '</div>',
    '<input id="' + id + '" type="range" min="' + control.dataset.min + '" max="' + control.dataset.max + '" step="' + control.dataset.step + '" value="' + value + '" />',
    '<div class="hint">' + (control.dataset.hint || '') + '</div>'
  ].join('');
  const input = document.getElementById(id);
  const output = document.getElementById(id + 'Value');
  input.addEventListener('input', () => {
    output.textContent = input.value;
    scheduleVectorize(0);
  });
});

fileInput.addEventListener('change', () => {
  const file = fileInput.files[0];
  if (!file) return;
  source.src = URL.createObjectURL(file);
  currentDataUrl = '';
  scheduleVectorize(0);
});
modeInput.addEventListener('change', () => {
  applyModePreset(modeInput.value);
  scheduleVectorize(0);
});
runButton.addEventListener('click', () => {
  requestRevision++;
  void runVectorize(requestRevision, true);
});

function applyModePreset(mode) {
  const preset = modePresets[mode];
  if (!preset) return;
  Object.entries(preset).forEach(([id, value]) => {
    const input = document.getElementById(id);
    const output = document.getElementById(id + 'Value');
    if (!input || !output) return;
    input.value = String(value);
    output.textContent = input.value;
  });
}

function scheduleVectorize(delay = 0) {
  requestRevision++;
  statusEl.textContent = 'Parameters changed, refreshing SVG...';
  window.clearTimeout(autoRunTimer);
  autoRunTimer = window.setTimeout(() => {
    void runVectorize(requestRevision, false);
  }, delay);
}

function qualityPathLimit(metrics) {
  return Number(metrics?.request?.colors) >= 32 ? 12 : 8;
}

function qualitySubpathLimit(metrics) {
  return Number(metrics?.request?.colors) >= 32 ? 120 : 80;
}

function qualityRMSELimit(metrics) {
  const colors = Number(metrics?.request?.colors);
  if (colors >= 32) return 0.05;
  if (colors >= 16) return 0.045;
  return 0.04;
}

function renderQuality(metrics) {
  if (!metrics) {
    qualityEl.innerHTML = '<div class="quality-item quality-missing">暂无质量指标</div>';
    return;
  }
  qualityEl.innerHTML = qualityChecks.map((check) => {
    if (typeof check.enabled === 'function' && !check.enabled(metrics)) {
      return '<div class="quality-item quality-missing"><div class="quality-title">' + check.label + '</div><div class="quality-value">暂无</div></div>';
    }
    const rawValue = metrics[check.key];
    const value = Number(rawValue);
    if (!Number.isFinite(value)) {
      return '<div class="quality-item quality-missing"><div class="quality-title">' + check.label + '</div><div class="quality-value">暂无</div></div>';
    }
    const limit = typeof check.limit === 'function' ? check.limit(metrics) : check.limit;
    const suffix = typeof check.suffix === 'function' ? check.suffix(metrics) : check.suffix;
    const pass = value <= limit;
    return [
      '<div class="quality-item ' + (pass ? 'quality-pass' : 'quality-warn') + '">',
        '<div class="quality-title">' + (pass ? '通过 ' : '警告 ') + check.label + '</div>',
        '<div class="quality-value">' + check.format(value) + suffix + '</div>',
      '</div>'
    ].join('');
  }).join('');
}

function renderArtifacts(artifacts) {
  if (!Array.isArray(artifacts) || artifacts.length === 0) {
    artifactsEl.innerHTML = '<div class="quality-item quality-missing">暂无输出文件</div>';
    return;
  }
  const ranked = [...artifacts].sort((a, b) => {
    const ai = preferredArtifacts.indexOf(a.name);
    const bi = preferredArtifacts.indexOf(b.name);
    return (ai === -1 ? 999 : ai) - (bi === -1 ? 999 : bi) || String(a.name).localeCompare(String(b.name));
  });
  artifactsEl.innerHTML = ranked.map((item) => {
    const name = String(item.name || 'artifact');
    const url = String(item.url || '#');
    return '<a class="artifact-link" href="' + url + '" target="_blank" rel="noreferrer">' + name + '</a>';
  }).join('');
}

async function readCurrentDataUrl(file) {
  if (currentDataUrl) return currentDataUrl;
  currentDataUrl = await new Promise((resolve) => {
    const reader = new FileReader();
    reader.onload = () => resolve(reader.result);
    reader.readAsDataURL(file);
  });
  return currentDataUrl;
}

async function runVectorize(revision = requestRevision, showMissingFileAlert = true) {
  window.clearTimeout(autoRunTimer);
  const file = fileInput.files[0];
  if (!file) {
    if (showMissingFileAlert) alert('Choose an image first');
    statusEl.textContent = '';
    return;
  }
  if (isVectorizing) {
    rerunAfterCurrent = true;
    statusEl.textContent = 'Parameters changed, refreshing after current run...';
    return;
  }
  isVectorizing = true;
  const serial = ++runSerial;
  statusEl.textContent = 'Processing... #' + serial;
  renderQuality(null);
  renderArtifacts([]);
  visualComparisonEl.removeAttribute('src');
  runButton.disabled = true;
  const dataUrl = await readCurrentDataUrl(file);
  const body = { dataUrl, mode: modeInput.value };
  sliderIds.forEach((id) => {
    body[id] = Number(document.getElementById(id).value);
  });
  try {
    const res = await fetch('/api/vectorize', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body)
    });
    const payload = await res.json();
    if (revision !== requestRevision) {
      rerunAfterCurrent = true;
      return;
    }
    if (serial !== runSerial) return;
    if (payload.code !== 0) {
      statusEl.textContent = payload.msg || 'Failed';
      return;
    }
    previewEl.src = payload.data.previewUrl + '?t=' + Date.now();
    metricsEl.textContent = JSON.stringify(payload.data.metrics, null, 2);
    renderQuality(payload.data.metrics);
    renderArtifacts(payload.data.artifacts);
    const visualArtifact = (payload.data.artifacts || []).find((item) => item.name === 'visual-comparison.png');
    if (visualArtifact) {
      visualComparisonEl.src = visualArtifact.url + '?t=' + Date.now();
    }
    const visualLink = visualArtifact ? ' · <a href="' + visualArtifact.url + '" target="_blank">visual-comparison.png</a>' : '';
    statusEl.innerHTML = 'Done #' + serial + ': <a href="' + payload.data.svgUrl + '" target="_blank">output.svg</a>' + visualLink;
  } catch (error) {
    if (serial !== runSerial) return;
    statusEl.textContent = error instanceof Error ? error.message : 'Failed';
  } finally {
    if (serial === runSerial) runButton.disabled = false;
    isVectorizing = false;
    if (rerunAfterCurrent) {
      rerunAfterCurrent = false;
      window.setTimeout(() => {
        void runVectorize(requestRevision, false);
      }, 0);
    }
  }
}
</script>
</body>
</html>
`
