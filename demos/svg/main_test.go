package main

import (
	"context"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestParseHistogramChannel(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  uint8
		ok    bool
	}{
		{name: "integer", input: "125", want: 125, ok: true},
		{name: "decimal rounds", input: "125.6", want: 126, ok: true},
		{name: "clamps low", input: "-4", want: 0, ok: true},
		{name: "clamps high", input: "260", want: 255, ok: true},
		{name: "rejects text", input: "nope", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseHistogramChannel(tt.input)
			if ok != tt.ok {
				t.Fatalf("ok = %v, want %v", ok, tt.ok)
			}
			if got != tt.want {
				t.Fatalf("value = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestCleanLogoLabDefaultsMatchRecommendedPreset(t *testing.T) {
	if defaultLongEdge != 4096 {
		t.Fatalf("defaultLongEdge = %d, want 4096", defaultLongEdge)
	}
	if defaultMinComponentRatio != 0.00005 {
		t.Fatalf("defaultMinComponentRatio = %.8f, want 0.00005", defaultMinComponentRatio)
	}
	if defaultMaxHoleRatio != 0.00008 {
		t.Fatalf("defaultMaxHoleRatio = %.8f, want 0.00008", defaultMaxHoleRatio)
	}
	if defaultMergeDistance != 64 {
		t.Fatalf("defaultMergeDistance = %d, want 64", defaultMergeDistance)
	}
	if defaultMergeHueDistance != 16 {
		t.Fatalf("defaultMergeHueDistance = %d, want 16", defaultMergeHueDistance)
	}
	if defaultMergeLightness != 80 {
		t.Fatalf("defaultMergeLightness = %d, want 80", defaultMergeLightness)
	}
	if defaultMergeSaturation != 0.55 {
		t.Fatalf("defaultMergeSaturation = %.2f, want 0.55", defaultMergeSaturation)
	}
	if defaultLightMinAreaRatio != 0.004 {
		t.Fatalf("defaultLightMinAreaRatio = %.6f, want 0.004", defaultLightMinAreaRatio)
	}
}

func TestIndexHTMLLinksVisualComparisonArtifact(t *testing.T) {
	for _, want := range []string{"id=\"artifacts\"", "id=\"visualComparison\"", "visualComparisonEl.src", "visual-comparison.png"} {
		if !strings.Contains(indexHTML, want) {
			t.Fatalf("indexHTML does not contain %q", want)
		}
	}
}

func TestIndexHTMLIncludesIllustrationMode(t *testing.T) {
	for _, want := range []string{`option value="illustration"`, `illustration - 插画/素材保真`, `illustration: {`} {
		if !strings.Contains(indexHTML, want) {
			t.Fatalf("indexHTML does not contain %q", want)
		}
	}
}

func TestApplyIllustrationDefaultsPreservesComplexArtwork(t *testing.T) {
	got := applyIllustrationDefaults(vectorizeRequest{})

	if got.Mode != "illustration" {
		t.Fatalf("mode = %q, want illustration", got.Mode)
	}
	if got.Colors != 96 {
		t.Fatalf("colors = %d, want 96", got.Colors)
	}
	if got.MinComponentRatio != 0.000005 {
		t.Fatalf("minComponentRatio = %.8f, want 0.000005", got.MinComponentRatio)
	}
	if got.MaxHoleRatio != 0.00001 {
		t.Fatalf("maxHoleRatio = %.8f, want 0.00001", got.MaxHoleRatio)
	}
	if got.MergeDistance != 18 || got.MergeHueDistance != 8 || got.MergeLightness != 32 || got.MergeSaturation != 0.3 {
		t.Fatalf("merge params = %.0f/%.0f/%.0f/%.2f, want 18/8/32/0.30", got.MergeDistance, got.MergeHueDistance, got.MergeLightness, got.MergeSaturation)
	}
	if got.RemoveSpeckles == nil || *got.RemoveSpeckles {
		t.Fatalf("removeSpeckles = %v, want false", got.RemoveSpeckles)
	}
	if got.FillSmallHoles == nil || *got.FillSmallHoles {
		t.Fatalf("fillSmallHoles = %v, want false", got.FillSmallHoles)
	}
}

func TestIllustrationTraceLayersKeepSimilarPaletteColorsSeparate(t *testing.T) {
	img, total := paletteImageFromColors(1, []paletteColor{
		{Hex: "#FFFFFF", R: 0xff, G: 0xff, B: 0xff, Count: 100},
		{Hex: "#FACA71", R: 0xfa, G: 0xca, B: 0x71, Count: 40},
		{Hex: "#FBC976", R: 0xfb, G: 0xc9, B: 0x76, Count: 40},
	})
	palette := readPalette(img)
	background := detectBackgroundLayer(img, palette, defaultMergeDistance, defaultMergeHueDistance, defaultMergeLightness, defaultMergeSaturation)

	layers := buildTraceLayers(palette, total, background.Keys, defaultMergeDistance, defaultMergeHueDistance, defaultMergeLightness, defaultMergeSaturation, true)

	if len(layers) != 2 {
		t.Fatalf("illustration layers = %d, want 2; layers=%+v", len(layers), layers)
	}
}

func TestCollectBatchImagePaths(t *testing.T) {
	dir := t.TempDir()
	files := []string{
		"b.JPG",
		"a.png",
		filepath.Join("nested", "c.jpeg"),
		filepath.Join("nested", "d.gif"),
		"ignore.svg",
		"notes.txt",
	}
	for _, name := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create fixture dir: %v", err)
		}
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatalf("write fixture file: %v", err)
		}
	}

	got, err := collectBatchImagePaths(dir)
	if err != nil {
		t.Fatalf("collectBatchImagePaths() error = %v", err)
	}
	want := []string{
		filepath.Join(dir, "a.png"),
		filepath.Join(dir, "b.JPG"),
		filepath.Join(dir, "nested", "c.jpeg"),
		filepath.Join(dir, "nested", "d.gif"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("paths = %v, want %v", got, want)
	}
}

func TestGenerateBuiltInCorpusCreatesBatchableStressImages(t *testing.T) {
	if _, err := requireCommand("magick"); err != nil {
		t.Skipf("magick is not available: %v", err)
	}
	dir := t.TempDir()
	paths, err := generateBuiltInCorpus(context.Background(), dir)
	if err != nil {
		t.Fatalf("generateBuiltInCorpus() error = %v", err)
	}
	if len(paths) < 10 {
		t.Fatalf("generated %d corpus images, want at least 10", len(paths))
	}
	for _, want := range []string{
		"01-bls-pale-ribbon.png",
		"05-cjk-slogan-banner.png",
		"07-white-text-brand-block.png",
	} {
		path := filepath.Join(dir, want)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected corpus image %s: %v", want, err)
		}
	}
	batchPaths, err := collectBatchImagePaths(dir)
	if err != nil {
		t.Fatalf("collectBatchImagePaths() error = %v", err)
	}
	if len(batchPaths) != len(paths) {
		t.Fatalf("collectBatchImagePaths() found %d images, want %d", len(batchPaths), len(paths))
	}
}

func TestAssessLogoQualityFlagsFailedRequirements(t *testing.T) {
	metrics := metricsResult{
		BackgroundDelta:         7.5,
		ForegroundColorDelta:    32,
		ForegroundCoverageDelta: 0.063,
		RenderedRMSE:            0.072,
		SVGPaths:                24,
		SVGSubpaths:             340,
		Request:                 vectorizeParams{Mode: "cleanLogo", Colors: 12},
	}

	got := assessLogoQuality(metrics)

	if got.Status != "fail" {
		t.Fatalf("status = %q, want fail", got.Status)
	}
	if got.Failed != 6 {
		t.Fatalf("failed = %d, want 6", got.Failed)
	}
	wantNames := []string{"backgroundDelta", "foregroundColorDelta", "foregroundCoverageDelta", "renderedRMSE", "svgPaths", "svgSubpaths"}
	for _, name := range wantNames {
		if !qualityHasCheck(got, name, "fail") {
			t.Fatalf("quality result does not contain failed check %q: %+v", name, got.Checks)
		}
	}
}

func TestAssessLogoQualityPassesCleanMetrics(t *testing.T) {
	metrics := metricsResult{
		BackgroundDelta:         0,
		ForegroundColorDelta:    4,
		ForegroundCoverageDelta: 0.018,
		RenderedRMSE:            0.031,
		SVGPaths:                4,
		SVGSubpaths:             48,
		Request:                 vectorizeParams{Mode: "cleanLogo", Colors: 12},
	}

	got := assessLogoQuality(metrics)

	if got.Status != "pass" {
		t.Fatalf("status = %q, want pass; checks=%+v", got.Status, got.Checks)
	}
	if got.Failed != 0 || got.Warnings != 0 {
		t.Fatalf("failed/warnings = %d/%d, want 0/0", got.Failed, got.Warnings)
	}
}

func TestAssessLogoQualityFailsDarkLightTextTintBleed(t *testing.T) {
	metrics := metricsResult{
		BackgroundDelta:                 0,
		ForegroundColorDelta:            0,
		ForegroundCoverageDelta:         0.018,
		RenderedRMSE:                    0.031,
		SVGPaths:                        4,
		SVGSubpaths:                     48,
		DarkLightTextTintBleedRatio:     0.023,
		HasDarkLightTextTintBleedMetric: true,
		Request:                         vectorizeParams{Mode: "cleanLogo", Colors: 32},
	}

	got := assessLogoQuality(metrics)

	if got.Status != "fail" {
		t.Fatalf("status = %q, want fail; checks=%+v", got.Status, got.Checks)
	}
	if !qualityHasCheck(got, "darkLightTextTintBleedRatio", "fail") {
		t.Fatalf("quality result does not contain failed dark text tint check: %+v", got.Checks)
	}
}

func TestAssessLogoQualitySkipsDarkLightTextTintBleedWhenMetricIsUnavailable(t *testing.T) {
	metrics := metricsResult{
		BackgroundDelta:             0,
		ForegroundColorDelta:        0,
		ForegroundCoverageDelta:     0.018,
		RenderedRMSE:                0.031,
		SVGPaths:                    4,
		SVGSubpaths:                 48,
		DarkLightTextTintBleedRatio: 0.023,
		Request:                     vectorizeParams{Mode: "cleanLogo", Colors: 12},
	}

	got := assessLogoQuality(metrics)

	if got.Status != "pass" {
		t.Fatalf("status = %q, want pass; checks=%+v", got.Status, got.Checks)
	}
	if qualityHasCheck(got, "darkLightTextTintBleedRatio", "fail") || qualityHasCheck(got, "darkLightTextTintBleedRatio", "warn") {
		t.Fatalf("dark text tint check should be skipped when metric is unavailable: %+v", got.Checks)
	}
}

func TestDarkLightTextTintBleedRatioMeasuresTintedLightTextEdges(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 8, 8))
	rendered := image.NewRGBA(image.Rect(0, 0, 8, 8))
	dark := color.RGBA{R: 15, G: 23, B: 42, A: 255}
	light := color.RGBA{R: 230, G: 230, B: 232, A: 255}
	cyan := color.RGBA{R: 56, G: 189, B: 248, A: 255}
	draw.Draw(source, source.Bounds(), &image.Uniform{C: dark}, image.Point{}, draw.Src)
	draw.Draw(rendered, rendered.Bounds(), &image.Uniform{C: dark}, image.Point{}, draw.Src)
	for y := 2; y < 6; y++ {
		for x := 2; x < 6; x++ {
			source.Set(x, y, light)
			rendered.Set(x, y, cyan)
		}
	}

	got, ok := darkLightTextTintBleedRatio(source, rendered, rgbColor{R: 15, G: 23, B: 42})

	if !ok {
		t.Fatalf("darkLightTextTintBleedRatio() ok = false, want true")
	}
	if got < 0.99 {
		t.Fatalf("darkLightTextTintBleedRatio() = %.4f, want near 1", got)
	}
}

func TestDarkLightTextTintBleedRatioSkipsWhenNoLightText(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 8, 8))
	rendered := image.NewRGBA(image.Rect(0, 0, 8, 8))
	dark := color.RGBA{R: 15, G: 23, B: 42, A: 255}
	draw.Draw(source, source.Bounds(), &image.Uniform{C: dark}, image.Point{}, draw.Src)
	draw.Draw(rendered, rendered.Bounds(), &image.Uniform{C: dark}, image.Point{}, draw.Src)

	_, ok := darkLightTextTintBleedRatio(source, rendered, rgbColor{R: 15, G: 23, B: 42})

	if ok {
		t.Fatalf("darkLightTextTintBleedRatio() ok = true, want false")
	}
}

func TestAssessLogoQualityAllowsSameHueLightAccentRMSE(t *testing.T) {
	metrics := metricsResult{
		BackgroundDelta:         0,
		ForegroundColorDelta:    0,
		ForegroundCoverageDelta: 0.026,
		RenderedRMSE:            0.0426,
		SVGPaths:                2,
		SVGSubpaths:             36,
		Request:                 vectorizeParams{Mode: "cleanLogo", Colors: 16},
	}

	got := assessLogoQuality(metrics)

	if got.Status != "pass" {
		t.Fatalf("status = %q, want pass; checks=%+v", got.Status, got.Checks)
	}
	if qualityHasCheck(got, "renderedRMSE", "warn") {
		t.Fatalf("renderedRMSE should not warn for 16-color light-accent logos: %+v", got.Checks)
	}
}

func TestAssessLogoQualityAllowsDenseMicroTextWhenCoverageAndColorAreStable(t *testing.T) {
	metrics := metricsResult{
		BackgroundDelta:         0,
		ForegroundColorDelta:    0,
		ForegroundCoverageDelta: 0.0245,
		RenderedRMSE:            0.0649,
		SVGPaths:                6,
		SVGSubpaths:             260,
		Request:                 vectorizeParams{Mode: "cleanLogo", Colors: 8},
	}

	got := assessLogoQuality(metrics)

	if got.Status != "pass" {
		t.Fatalf("status = %q, want pass; checks=%+v", got.Status, got.Checks)
	}
	if qualityHasCheck(got, "renderedRMSE", "fail") || qualityHasCheck(got, "svgSubpaths", "fail") {
		t.Fatalf("dense micro text should not fail RMSE/subpath checks when color and coverage are stable: %+v", got.Checks)
	}
}

func TestAssessLogoQualityAllowsCleanPaleRibbonCoverage(t *testing.T) {
	metrics := metricsResult{
		BackgroundDelta:         0,
		ForegroundColorDelta:    0,
		ForegroundCoverageDelta: 0.0348,
		RenderedRMSE:            0.0322,
		SVGPaths:                2,
		SVGSubpaths:             73,
		Request:                 vectorizeParams{Mode: "cleanLogo", Colors: 16},
	}

	got := assessLogoQuality(metrics)

	if got.Status != "pass" {
		t.Fatalf("status = %q, want pass; checks=%+v", got.Status, got.Checks)
	}
}

func TestAssessLogoQualityAllowsCleanFlatBannerSubpaths(t *testing.T) {
	metrics := metricsResult{
		BackgroundDelta:         0,
		ForegroundColorDelta:    0,
		ForegroundCoverageDelta: 0.0331,
		RenderedRMSE:            0.0282,
		SVGPaths:                6,
		SVGSubpaths:             165,
		Request:                 vectorizeParams{Mode: "cleanLogo", Colors: 8},
	}

	got := assessLogoQuality(metrics)

	if got.Status != "pass" {
		t.Fatalf("status = %q, want pass; checks=%+v", got.Status, got.Checks)
	}
}

func TestStrictBatchQualityErrorFailsOnFailedOrErroredItems(t *testing.T) {
	result := batchRunResult{Failed: 1, Warnings: 0, Errored: 0}
	if err := strictBatchQualityError(result); err == nil {
		t.Fatal("strictBatchQualityError() = nil, want error for failed items")
	}
	result = batchRunResult{Failed: 0, Warnings: 0, Errored: 1}
	if err := strictBatchQualityError(result); err == nil {
		t.Fatal("strictBatchQualityError() = nil, want error for errored items")
	}
}

func TestStrictBatchQualityErrorAllowsPassesAndWarnings(t *testing.T) {
	result := batchRunResult{Passed: 8, Warnings: 2, Failed: 0, Errored: 0}
	if err := strictBatchQualityError(result); err != nil {
		t.Fatalf("strictBatchQualityError() = %v, want nil", err)
	}
}

func TestMinimumBatchCountErrorFailsWhenTooFewItemsWereReviewed(t *testing.T) {
	result := batchRunResult{Items: make([]batchItemResult, 3)}
	if err := minimumBatchCountError(result, 10); err == nil {
		t.Fatal("minimumBatchCountError() = nil, want error for too few reviewed items")
	}
}

func TestMinimumBatchCountErrorAllowsEnoughItemsAndDisabledLimit(t *testing.T) {
	result := batchRunResult{Items: make([]batchItemResult, 10)}
	if err := minimumBatchCountError(result, 10); err != nil {
		t.Fatalf("minimumBatchCountError() = %v, want nil", err)
	}
	if err := minimumBatchCountError(batchRunResult{Items: make([]batchItemResult, 1)}, 0); err != nil {
		t.Fatalf("minimumBatchCountError(disabled) = %v, want nil", err)
	}
}

func TestDominantForegroundColorDelta(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 10, 10))
	rendered := image.NewRGBA(image.Rect(0, 0, 10, 10))
	white := color.RGBA{R: 0xfd, G: 0xfd, B: 0xfd, A: 0xff}
	teal := color.RGBA{R: 0x08, G: 0x75, B: 0x8d, A: 0xff}
	shiftedTeal := color.RGBA{R: 0x12, G: 0x80, B: 0x98, A: 0xff}
	fillRect(source, source.Bounds(), white)
	fillRect(rendered, rendered.Bounds(), white)
	fillRect(source, image.Rect(2, 2, 8, 8), teal)
	fillRect(rendered, image.Rect(2, 2, 8, 8), shiftedTeal)

	got, ok := foregroundColorDelta(source, rendered, rgbColor{R: 0xfd, G: 0xfd, B: 0xfd})

	if !ok {
		t.Fatal("foregroundColorDelta did not find foreground colors")
	}
	want := rgbDistance(
		quantizedQualityColor(rgbColor{R: 0x08, G: 0x75, B: 0x8d}),
		quantizedQualityColor(rgbColor{R: 0x12, G: 0x80, B: 0x98}),
	)
	if math.Abs(got-want) > 0.0001 {
		t.Fatalf("foreground color delta = %.4f, want %.4f", got, want)
	}
}

func TestQuantizedQualityColorClampsWhite(t *testing.T) {
	got := quantizedQualityColor(rgbColor{R: 252, G: 252, B: 252})
	want := rgbColor{R: 255, G: 255, B: 255}
	if got != want {
		t.Fatalf("quantized white = %+v, want %+v", got, want)
	}
}

func TestFormatBatchQualitySummary(t *testing.T) {
	result := batchRunResult{
		Passed:   7,
		Warnings: 2,
		Failed:   1,
		Errored:  3,
	}

	got := formatBatchQualitySummary(result)
	want := "quality: 7 passed, 2 warnings, 1 failed, 3 errored"
	if got != want {
		t.Fatalf("summary = %q, want %q", got, want)
	}
}

func TestRenderBatchMarkdownReport(t *testing.T) {
	result := batchRunResult{
		InputDir:     "/tmp/source-logos",
		OutputDir:    "/tmp/out",
		ContactSheet: "/tmp/out/contact-sheet.png",
		Passed:       1,
		Failed:       1,
		Items: []batchItemResult{
			{
				Source:           "/tmp/source-logos/pass.png",
				VisualComparison: "/tmp/out/pass/visual-comparison.png",
				OutputSVG:        "/tmp/out/pass/output.svg",
				Metrics: metricsResult{
					Request:                 vectorizeParams{Colors: 3},
					BackgroundDelta:         0,
					ForegroundColorDelta:    4,
					ForegroundCoverageDelta: 0.01,
					RenderedRMSE:            0.02,
					SVGPaths:                2,
					SVGSubpaths:             20,
				},
				Quality: logoQualityResult{Status: "pass"},
			},
			{
				Source:           "/tmp/source-logos/fail.png",
				VisualComparison: "/tmp/out/fail/visual-comparison.png",
				OutputSVG:        "/tmp/out/fail/output.svg",
				Metrics: metricsResult{
					Request:                 vectorizeParams{Colors: 12},
					BackgroundDelta:         0,
					ForegroundColorDelta:    30,
					ForegroundCoverageDelta: 0.06,
					RenderedRMSE:            0.07,
					SVGPaths:                24,
					SVGSubpaths:             300,
				},
				Quality: logoQualityResult{
					Status: "fail",
					Failed: 2,
					Checks: []logoQualityCheck{
						{Name: "foregroundColorDelta", Label: "主体颜色不漂移", Status: "fail", Value: 30, FailAt: 24},
						{Name: "foregroundCoverageDelta", Label: "轮廓/文字覆盖不漂移", Status: "fail", Value: 0.06, FailAt: 0.04},
					},
				},
			},
		},
	}

	got := renderBatchMarkdownReport(result)
	for _, want := range []string{
		"# SVG Logo Batch Report",
		"quality: 1 passed, 0 warnings, 1 failed, 0 errored",
		"contact-sheet.png",
		"fail.png",
		"foregroundColorDelta:fail(30.0000/24.0000)",
		"foregroundCoverageDelta:fail(0.0600/0.0400)",
		"foregroundColorDelta:fail | 1",
		"foregroundCoverageDelta:fail | 1",
		"visual-comparison.png",
		"output.svg",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("report does not contain %q:\n%s", want, got)
		}
	}
	passIndex := strings.Index(got, "`pass.png`")
	failIndex := strings.Index(got, "`fail.png`")
	if passIndex == -1 || failIndex == -1 {
		t.Fatalf("report does not contain both pass and fail rows:\n%s", got)
	}
	if failIndex > passIndex {
		t.Fatalf("failed item should be listed before passed item:\n%s", got)
	}
}

func qualityHasCheck(result logoQualityResult, name string, status string) bool {
	for _, check := range result.Checks {
		if check.Name == name && check.Status == status {
			return true
		}
	}
	return false
}

func TestEstimateCleanLogoColors(t *testing.T) {
	magickPath, err := requireCommand("magick")
	if err != nil {
		t.Skipf("magick is not available: %v", err)
	}
	generatedDir := t.TempDir()

	tests := []struct {
		name string
		path string
		want int
	}{
		{
			name: "same hue BLS logo with light neutral accent keeps color detail",
			path: sampleSourcePath("bls-logo.png", "d0454657e509e31a"),
			want: 16,
		},
		{
			name: "red and white mark uses compact palette",
			path: sampleSourcePath("red-mark.png", "c2a368d3868d96b8"),
			want: 3,
		},
		{
			name: "multi color GJ logo keeps brand colors",
			path: sampleSourcePath("gj-logo.png", "5bf827797c9f90ea"),
			want: 12,
		},
		{
			name: "dark background light strokes get expanded palette",
			path: generatedDarkThinStrokeLogo(t, generatedDir),
			want: 32,
		},
		{
			name: "generated multi color block logo keeps multi color palette",
			path: generatedMultiColorBlockLogo(t, generatedDir),
			want: 12,
		},
		{
			name: "monochrome thin line icon stays compact",
			path: generatedMonochromeThinLineIcon(t, generatedDir),
			want: 3,
		},
		{
			name: "transparent background logo uses white canvas",
			path: generatedTransparentLogo(t, generatedDir),
			want: 8,
		},
		{
			name: "speckled logo ignores isolated noise",
			path: generatedSpeckledLogo(t, generatedDir),
			want: 3,
		},
		{
			name: "micro text strokes stay visible",
			path: generatedMicroTextLogo(t, generatedDir),
			want: 8,
		},
		{
			name: "real font small text keeps neutral palette",
			path: generatedRealFontTextLogo(t, generatedDir),
			want: 12,
		},
		{
			name: "long real font tagline keeps neutral palette",
			path: generatedLongTaglineTextLogo(t, generatedDir),
			want: 12,
		},
		{
			name: "serif logistic logo keeps ribbon and service text",
			path: generatedSerifLogisticLogo(t, generatedDir),
			want: 16,
		},
		{
			name: "low contrast real font tagline keeps neutral palette",
			path: generatedLowContrastTaglineLogo(t, generatedDir),
			want: 12,
		},
		{
			name: "cjk real font text keeps neutral palette",
			path: generatedCJKRealFontTextLogo(t, generatedDir),
			want: 12,
		},
		{
			name: "flat cjk slogan banner keeps illustration and text",
			path: generatedFlatCJKSloganBanner(t, generatedDir),
			want: 8,
		},
		{
			name: "dark real font white text gets expanded palette",
			path: generatedDarkRealFontTextLogo(t, generatedDir),
			want: 32,
		},
		{
			name: "ring holes and diagonal edges stay clean",
			path: generatedRingDiagonalLogo(t, generatedDir),
			want: 8,
		},
		{
			name: "soft neutral shadow stays controlled",
			path: generatedSoftShadowLogo(t, generatedDir),
			want: 8,
		},
		{
			name: "jpeg compression artifacts stay compact",
			path: generatedJPEGArtifactLogo(t, generatedDir),
			want: 8,
		},
		{
			name: "thin colored outline logo keeps strokes",
			path: generatedThinOutlineLogo(t, generatedDir),
			want: 3,
		},
		{
			name: "tinted background logo preserves canvas",
			path: generatedTintedBackgroundLogo(t, generatedDir),
			want: 8,
		},
		{
			name: "knockout micro holes stay open",
			path: generatedKnockoutMicroHolesLogo(t, generatedDir),
			want: 8,
		},
		{
			name: "white text on brand block keeps knockout letters",
			path: generatedWhiteTextOnBrandBlockLogo(t, generatedDir),
			want: 3,
		},
		{
			name: "semi transparent shadow logo composites cleanly",
			path: generatedSemiTransparentShadowLogo(t, generatedDir),
			want: 8,
		},
		{
			name: "narrow negative gaps stay open",
			path: generatedNarrowGapLogo(t, generatedDir),
			want: 12,
		},
		{
			name: "small legitimate marks stay visible",
			path: generatedSmallMarksLogo(t, generatedDir),
			want: 3,
		},
		{
			name: "edge aligned logo keeps border strokes",
			path: generatedEdgeAlignedLogo(t, generatedDir),
			want: 8,
		},
		{
			name: "same hue gradient logo keeps tonal steps",
			path: generatedSameHueGradientLogo(t, generatedDir),
			want: 16,
		},
		{
			name: "white knockout text on off white background stays white",
			path: generatedWhiteTextOffWhiteLogo(t, generatedDir),
			want: 8,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := os.Stat(tt.path); err != nil {
				t.Skipf("sample image is not available: %v", err)
			}
			if got := estimateCleanLogoColors(ctx, magickPath, tt.path); got != tt.want {
				t.Fatalf("estimateCleanLogoColors() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestCleanLogoVectorizationMetrics(t *testing.T) {
	if _, err := requireCommand("magick"); err != nil {
		t.Skipf("magick is not available: %v", err)
	}
	if _, err := requireCommand("potrace"); err != nil {
		t.Skipf("potrace is not available: %v", err)
	}
	generatedDir := t.TempDir()

	tests := []struct {
		name            string
		source          string
		wantColors      int
		maxPaths        int
		maxSubpaths     int
		maxBgDelta      float64
		maxFgColorDelta float64
		maxFgDelta      float64
		maxRMSE         float64
		maxColorDelta   float64
		requiredFills   []string
		anyFills        []string
	}{
		{
			name:            "BLS logo keeps light accent and exact teal colors",
			source:          sampleSourcePath("bls-logo.png", "d0454657e509e31a"),
			wantColors:      16,
			maxPaths:        3,
			maxSubpaths:     80,
			maxBgDelta:      0,
			maxFgDelta:      0.04,
			maxRMSE:         0.05,
			maxFgColorDelta: 1,
			maxColorDelta:   12,
			anyFills:        []string{"#02738b", "#05748c", "#08758d"},
		},
		{
			name:        "red mark stays compact",
			source:      sampleSourcePath("red-mark.png", "c2a368d3868d96b8"),
			wantColors:  3,
			maxPaths:    2,
			maxSubpaths: 12,
			maxBgDelta:  0,
			maxFgDelta:  0.04,
			maxRMSE:     0.04,
			anyFills:    []string{"#CA1719", "#CA1819"},
		},
		{
			name:          "multi color GJ logo keeps separate brand colors",
			source:        sampleSourcePath("gj-logo.png", "5bf827797c9f90ea"),
			wantColors:    12,
			maxPaths:      6,
			maxSubpaths:   48,
			maxBgDelta:    0,
			maxFgDelta:    0.04,
			maxRMSE:       0.04,
			requiredFills: []string{"#241668", "#dc1a17", "#e96d10"},
		},
		{
			name:          "dark background thin stroke logo keeps contrast",
			source:        generatedDarkThinStrokeLogo(t, generatedDir),
			wantColors:    32,
			maxPaths:      12,
			maxSubpaths:   120,
			maxBgDelta:    0,
			maxFgDelta:    0.05,
			maxRMSE:       0.045,
			requiredFills: []string{"#fee715"},
		},
		{
			name:          "multi color block text logo keeps separate colors",
			source:        generatedMultiColorBlockLogo(t, generatedDir),
			wantColors:    12,
			maxPaths:      10,
			maxSubpaths:   96,
			maxBgDelta:    0,
			maxFgDelta:    0.05,
			maxRMSE:       0.04,
			requiredFills: []string{"#2458e6", "#e63223", "#18a058"},
		},
		{
			name:          "monochrome thin line icon keeps strokes",
			source:        generatedMonochromeThinLineIcon(t, generatedDir),
			wantColors:    3,
			maxPaths:      8,
			maxSubpaths:   80,
			maxBgDelta:    0,
			maxFgDelta:    0.04,
			maxRMSE:       0.04,
			requiredFills: []string{"#131313"},
		},
		{
			name:          "transparent background logo uses clean white canvas",
			source:        generatedTransparentLogo(t, generatedDir),
			wantColors:    8,
			maxPaths:      8,
			maxSubpaths:   80,
			maxBgDelta:    0,
			maxFgDelta:    0.04,
			maxRMSE:       0.04,
			requiredFills: []string{"#2458e6", "#18a058"},
		},
		{
			name:          "speckled logo ignores isolated noise",
			source:        generatedSpeckledLogo(t, generatedDir),
			wantColors:    3,
			maxPaths:      8,
			maxSubpaths:   80,
			maxBgDelta:    0,
			maxFgDelta:    0.05,
			maxRMSE:       0.045,
			requiredFills: []string{"#141414"},
		},
		{
			name:          "micro text strokes stay visible",
			source:        generatedMicroTextLogo(t, generatedDir),
			wantColors:    8,
			maxPaths:      8,
			maxSubpaths:   320,
			maxBgDelta:    0,
			maxFgDelta:    0.025,
			maxRMSE:       0.07,
			requiredFills: []string{"#141414"},
		},
		{
			name:          "real font small text keeps dark letters",
			source:        generatedRealFontTextLogo(t, generatedDir),
			wantColors:    12,
			maxPaths:      10,
			maxSubpaths:   220,
			maxBgDelta:    0,
			maxFgDelta:    0.035,
			maxRMSE:       0.055,
			maxColorDelta: 8,
			requiredFills: []string{"#141414", "#08758d"},
		},
		{
			name:          "long real font tagline keeps dark letters",
			source:        generatedLongTaglineTextLogo(t, generatedDir),
			wantColors:    12,
			maxPaths:      10,
			maxSubpaths:   260,
			maxBgDelta:    0,
			maxFgDelta:    0.04,
			maxRMSE:       0.06,
			maxColorDelta: 8,
			requiredFills: []string{"#121212", "#08758d"},
		},
		{
			name:          "serif logistic logo keeps ribbon and service text",
			source:        generatedSerifLogisticLogo(t, generatedDir),
			wantColors:    16,
			maxPaths:      12,
			maxSubpaths:   280,
			maxBgDelta:    0,
			maxFgDelta:    0.04,
			maxRMSE:       0.055,
			maxColorDelta: 16,
			anyFills:      []string{"#00768e", "#08758d", "#0b778f"},
		},
		{
			name:          "low contrast real font tagline keeps gray letters",
			source:        generatedLowContrastTaglineLogo(t, generatedDir),
			wantColors:    12,
			maxPaths:      10,
			maxSubpaths:   260,
			maxBgDelta:    0,
			maxFgDelta:    0.04,
			maxRMSE:       0.055,
			maxColorDelta: 10,
			requiredFills: []string{"#787878", "#08758d"},
		},
		{
			name:          "cjk real font text stays readable",
			source:        generatedCJKRealFontTextLogo(t, generatedDir),
			wantColors:    12,
			maxPaths:      10,
			maxSubpaths:   240,
			maxBgDelta:    0,
			maxFgDelta:    0.035,
			maxRMSE:       0.04,
			maxColorDelta: 10,
			requiredFills: []string{"#141414", "#08758d"},
		},
		{
			name:          "flat cjk slogan banner keeps illustration and text",
			source:        generatedFlatCJKSloganBanner(t, generatedDir),
			wantColors:    8,
			maxPaths:      18,
			maxSubpaths:   520,
			maxBgDelta:    0,
			maxFgDelta:    0.04,
			maxRMSE:       0.055,
			maxColorDelta: 16,
			requiredFills: []string{"#114875", "#4e7498"},
			anyFills:      []string{"#eef4f8", "#a8cbe2", "#e1d5b5"},
		},
		{
			name:          "dark real font white text stays readable",
			source:        generatedDarkRealFontTextLogo(t, generatedDir),
			wantColors:    32,
			maxPaths:      14,
			maxSubpaths:   260,
			maxBgDelta:    0,
			maxFgDelta:    0.04,
			maxRMSE:       0.055,
			maxColorDelta: 18,
			requiredFills: []string{"#0F172A", "#38bdf8"},
			anyFills:      []string{"#f4f4f4", "#f9f9f9"},
		},
		{
			name:          "ring holes and diagonal edges stay clean",
			source:        generatedRingDiagonalLogo(t, generatedDir),
			wantColors:    8,
			maxPaths:      12,
			maxSubpaths:   80,
			maxBgDelta:    0,
			maxFgDelta:    0.04,
			maxRMSE:       0.04,
			requiredFills: []string{"#08758d", "#e63223"},
		},
		{
			name:          "soft neutral shadow stays controlled",
			source:        generatedSoftShadowLogo(t, generatedDir),
			wantColors:    8,
			maxPaths:      12,
			maxSubpaths:   120,
			maxBgDelta:    0,
			maxFgDelta:    0.04,
			maxRMSE:       0.045,
			requiredFills: []string{"#2458e6", "#cacaca"},
		},
		{
			name:          "jpeg compression artifacts stay compact",
			source:        generatedJPEGArtifactLogo(t, generatedDir),
			wantColors:    8,
			maxPaths:      12,
			maxSubpaths:   160,
			maxBgDelta:    0,
			maxFgDelta:    0.06,
			maxRMSE:       0.055,
			requiredFills: []string{"#2658e5", "#e53224"},
		},
		{
			name:          "thin colored outline logo keeps strokes",
			source:        generatedThinOutlineLogo(t, generatedDir),
			wantColors:    3,
			maxPaths:      8,
			maxSubpaths:   140,
			maxBgDelta:    0,
			maxFgDelta:    0.035,
			maxRMSE:       0.05,
			requiredFills: []string{"#0e7990"},
		},
		{
			name:          "tinted background logo preserves canvas",
			source:        generatedTintedBackgroundLogo(t, generatedDir),
			wantColors:    8,
			maxPaths:      10,
			maxSubpaths:   120,
			maxBgDelta:    0,
			maxFgDelta:    0.04,
			maxRMSE:       0.04,
			requiredFills: []string{"#0f766e", "#f97316", "#212b39"},
		},
		{
			name:          "knockout micro holes stay open",
			source:        generatedKnockoutMicroHolesLogo(t, generatedDir),
			wantColors:    8,
			maxPaths:      10,
			maxSubpaths:   180,
			maxBgDelta:    0,
			maxFgDelta:    0.035,
			maxRMSE:       0.045,
			requiredFills: []string{"#2558e6", "#f97316"},
		},
		{
			name:          "white text on brand block keeps knockout letters",
			source:        generatedWhiteTextOnBrandBlockLogo(t, generatedDir),
			wantColors:    3,
			maxPaths:      6,
			maxSubpaths:   140,
			maxBgDelta:    0,
			maxFgDelta:    0.04,
			maxRMSE:       0.045,
			maxColorDelta: 10,
			requiredFills: []string{"#0b778f"},
		},
		{
			name:          "semi transparent shadow logo composites cleanly",
			source:        generatedSemiTransparentShadowLogo(t, generatedDir),
			wantColors:    8,
			maxPaths:      12,
			maxSubpaths:   120,
			maxBgDelta:    0,
			maxFgDelta:    0.04,
			maxRMSE:       0.045,
			requiredFills: []string{"#2458e6", "#f97316", "#dfdfdf"},
		},
		{
			name:          "narrow negative gaps stay open",
			source:        generatedNarrowGapLogo(t, generatedDir),
			wantColors:    12,
			maxPaths:      12,
			maxSubpaths:   120,
			maxBgDelta:    0,
			maxFgDelta:    0.035,
			maxRMSE:       0.04,
			requiredFills: []string{"#2458e6", "#e63223", "#18a058"},
		},
		{
			name:          "small legitimate marks stay visible",
			source:        generatedSmallMarksLogo(t, generatedDir),
			wantColors:    3,
			maxPaths:      10,
			maxSubpaths:   140,
			maxBgDelta:    0,
			maxFgDelta:    0.035,
			maxRMSE:       0.045,
			requiredFills: []string{"#141414"},
		},
		{
			name:          "edge aligned logo keeps border strokes",
			source:        generatedEdgeAlignedLogo(t, generatedDir),
			wantColors:    8,
			maxPaths:      10,
			maxSubpaths:   120,
			maxBgDelta:    0,
			maxFgDelta:    0.035,
			maxRMSE:       0.04,
			requiredFills: []string{"#2458e6", "#f97316"},
		},
		{
			name:          "same hue gradient logo keeps tonal steps",
			source:        generatedSameHueGradientLogo(t, generatedDir),
			wantColors:    16,
			maxPaths:      10,
			maxSubpaths:   140,
			maxBgDelta:    0,
			maxFgDelta:    0.035,
			maxRMSE:       0.04,
			requiredFills: []string{"#006d87", "#40a9ba"},
		},
		{
			name:          "white knockout text on off white background stays white",
			source:        generatedWhiteTextOffWhiteLogo(t, generatedDir),
			wantColors:    8,
			maxPaths:      8,
			maxSubpaths:   100,
			maxBgDelta:    0,
			maxFgDelta:    0.035,
			maxRMSE:       0.04,
			maxColorDelta: 18,
			requiredFills: []string{"#123a5a"},
			anyFills:      []string{"#ffffff", "#f5f5f7"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := os.Stat(tt.source); err != nil {
				t.Skipf("sample image is not available: %v", err)
			}
			t.Setenv("SVG_DEMO_OUTPUTS", t.TempDir())
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			defer cancel()

			result, err := runJob(ctx, vectorizeRequest{FilePath: tt.source, Mode: "cleanLogo"})
			if err != nil {
				t.Fatalf("runJob() error = %v", err)
			}
			if result.Metrics.Request.Colors != tt.wantColors {
				t.Fatalf("colors = %d, want %d", result.Metrics.Request.Colors, tt.wantColors)
			}
			if result.Metrics.SVGPaths > tt.maxPaths {
				t.Fatalf("svg paths = %d, want <= %d", result.Metrics.SVGPaths, tt.maxPaths)
			}
			if result.Metrics.SVGSubpaths > tt.maxSubpaths {
				t.Fatalf("svg subpaths = %d, want <= %d", result.Metrics.SVGSubpaths, tt.maxSubpaths)
			}
			if result.Metrics.BackgroundDelta > tt.maxBgDelta {
				t.Fatalf("background delta = %.4f, want <= %.4f", result.Metrics.BackgroundDelta, tt.maxBgDelta)
			}
			maxFgColorDelta := tt.maxFgColorDelta
			if maxFgColorDelta <= 0 {
				maxFgColorDelta = 24
			}
			if result.Metrics.ForegroundColorDelta > maxFgColorDelta {
				t.Fatalf(
					"foreground color delta = %.4f, want <= %.4f (source=%s rendered=%s)",
					result.Metrics.ForegroundColorDelta,
					maxFgColorDelta,
					result.Metrics.SourceForegroundColor,
					result.Metrics.RenderedForegroundColor,
				)
			}
			if result.Metrics.ForegroundCoverageDelta > tt.maxFgDelta {
				t.Fatalf("foreground coverage delta = %.4f, want <= %.4f", result.Metrics.ForegroundCoverageDelta, tt.maxFgDelta)
			}
			if result.Metrics.RenderedRMSE > tt.maxRMSE {
				t.Fatalf("rendered RMSE = %.6f, want <= %.6f", result.Metrics.RenderedRMSE, tt.maxRMSE)
			}
			if tt.name == "dark real font white text stays readable" {
				if !result.Metrics.HasDarkLightTextTintBleedMetric {
					t.Fatal("dark light text tint bleed metric was not emitted")
				}
				if result.Metrics.DarkLightTextTintBleedRatio > 0.01 {
					t.Fatalf("dark light text tint bleed ratio = %.4f, want <= 0.0100", result.Metrics.DarkLightTextTintBleedRatio)
				}
			}
			outputSVG := artifactPath(result.Artifacts, "output.svg")
			if outputSVG == "" {
				t.Fatal("output.svg artifact was not returned")
			}
			visualComparison := artifactPath(result.Artifacts, "visual-comparison.png")
			if visualComparison == "" {
				t.Fatal("visual-comparison.png artifact was not returned")
			}
			if _, err := os.Stat(visualComparison); err != nil {
				t.Fatalf("visual-comparison.png artifact is not readable: %v", err)
			}
			svgBytes, err := os.ReadFile(outputSVG)
			if err != nil {
				t.Fatalf("read output.svg: %v", err)
			}
			svgText := string(svgBytes)
			for _, fill := range tt.requiredFills {
				if !strings.Contains(svgText, `fill="`+fill+`"`) {
					t.Fatalf("output.svg does not contain required fill %s; fills=%v", fill, svgFillValues(svgText))
				}
			}
			if len(tt.anyFills) > 0 && !containsAnyFill(svgText, tt.anyFills) {
				t.Fatalf("output.svg does not contain any acceptable fill from %v", tt.anyFills)
			}
			maxColorDelta := tt.maxColorDelta
			if maxColorDelta <= 0 {
				maxColorDelta = 12
			}
			assertNearestFillColorDelta(t, svgText, append(tt.requiredFills, tt.anyFills...), maxColorDelta)
		})
	}
}

func TestFlatBannerDarkTextIsNotBackgroundCompanion(t *testing.T) {
	img, total := paletteImageFromColors(1, []paletteColor{
		{Hex: "#DBE8F2", R: 0xdb, G: 0xe8, B: 0xf2, Count: 50},
		{Hex: "#A8CBE2", R: 0xa8, G: 0xcb, B: 0xe2, Count: 20},
		{Hex: "#114875", R: 0x11, G: 0x48, B: 0x75, Count: 20},
		{Hex: "#EEF4F8", R: 0xee, G: 0xf4, B: 0xf8, Count: 10},
	})
	palette := readPalette(img)
	background := detectBackgroundLayer(img, palette, defaultMergeDistance, defaultMergeHueDistance, defaultMergeLightness, defaultMergeSaturation)
	if _, ok := background.Keys[rgbKey(0x11, 0x48, 0x75)]; ok {
		t.Fatalf("dark banner text color was merged into background keys: %#v", background)
	}
	if _, ok := background.Keys[rgbKey(0xa8, 0xcb, 0xe2)]; ok {
		t.Fatalf("large pale illustration color was merged into background keys: %#v", background)
	}
	layers := buildTraceLayers(palette, total, background.Keys, defaultMergeDistance, defaultMergeHueDistance, defaultMergeLightness, defaultMergeSaturation, false)
	if !traceLayersContain(layers, rgbKey(0x11, 0x48, 0x75)) {
		t.Fatalf("dark banner text color was not preserved in trace layers: %#v", layers)
	}
	if !traceLayersContain(layers, rgbKey(0xa8, 0xcb, 0xe2)) {
		t.Fatalf("large pale illustration color was not preserved in trace layers: %#v", layers)
	}
}

func paletteImageFromColors(width int, colors []paletteColor) (image.Image, int) {
	total := 0
	for _, item := range colors {
		total += item.Count
	}
	height := int(math.Ceil(float64(total) / float64(width)))
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	index := 0
	for _, item := range colors {
		fill := color.RGBA{R: item.R, G: item.G, B: item.B, A: 0xff}
		for i := 0; i < item.Count; i++ {
			x := index % width
			y := index / width
			img.Set(x, y, fill)
			index++
		}
	}
	return img, total
}

func traceLayersContain(layers []traceLayer, key uint32) bool {
	for _, layer := range layers {
		if _, ok := layer.Keys[key]; ok {
			return true
		}
	}
	return false
}

func artifactPath(artifacts []artifact, name string) string {
	for _, item := range artifacts {
		if item.Name == name {
			return item.Path
		}
	}
	return ""
}

func containsAnyFill(svgText string, fills []string) bool {
	for _, fill := range fills {
		if strings.Contains(svgText, `fill="`+fill+`"`) {
			return true
		}
	}
	return false
}

func svgFillValues(svgText string) []string {
	matches := regexp.MustCompile(`fill="#[0-9A-Fa-f]{6}"`).FindAllString(svgText, -1)
	seen := map[string]struct{}{}
	result := make([]string, 0, len(matches))
	for _, match := range matches {
		if _, ok := seen[match]; ok {
			continue
		}
		seen[match] = struct{}{}
		result = append(result, match)
	}
	return result
}

func assertNearestFillColorDelta(t *testing.T, svgText string, expectedFills []string, maxDelta float64) {
	t.Helper()
	actualFills := svgFillValues(svgText)
	if len(actualFills) == 0 {
		t.Fatal("output.svg does not contain fill colors")
	}
	for _, expected := range expectedFills {
		expectedColor, ok := parseTestHexColor(expected)
		if !ok {
			t.Fatalf("invalid expected fill %q", expected)
		}
		best := float64(1 << 30)
		bestFill := ""
		for _, actual := range actualFills {
			actualColor, ok := parseTestHexColor(strings.TrimPrefix(actual, `fill="`))
			if !ok {
				continue
			}
			delta := testRGBDistance(expectedColor, actualColor)
			if delta < best {
				best = delta
				bestFill = actual
			}
		}
		if best > maxDelta {
			t.Fatalf("nearest fill for %s is %s with color delta %.2f, want <= %.2f; fills=%v", expected, bestFill, best, maxDelta, actualFills)
		}
	}
}

func parseTestHexColor(value string) ([3]int, bool) {
	value = strings.Trim(strings.TrimSpace(value), `"`)
	value = strings.TrimPrefix(value, "#")
	if len(value) != 6 {
		return [3]int{}, false
	}
	r, err := strconv.ParseInt(value[0:2], 16, 0)
	if err != nil {
		return [3]int{}, false
	}
	g, err := strconv.ParseInt(value[2:4], 16, 0)
	if err != nil {
		return [3]int{}, false
	}
	b, err := strconv.ParseInt(value[4:6], 16, 0)
	if err != nil {
		return [3]int{}, false
	}
	return [3]int{int(r), int(g), int(b)}, true
}

func testRGBDistance(a [3]int, b [3]int) float64 {
	dr := float64(a[0] - b[0])
	dg := float64(a[1] - b[1])
	db := float64(a[2] - b[2])
	return math.Sqrt(dr*dr + dg*dg + db*db)
}

func generatedDarkThinStrokeLogo(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "dark-thin-stroke-logo.png")
	img := image.NewRGBA(image.Rect(0, 0, 960, 420))
	fillRect(img, image.Rect(0, 0, 960, 420), color.RGBA{R: 0x10, G: 0x18, B: 0x20, A: 0xff})
	yellow := color.RGBA{R: 0xfe, G: 0xe7, B: 0x15, A: 0xff}
	white := color.RGBA{R: 0xee, G: 0xef, B: 0xef, A: 0xff}

	fillRect(img, image.Rect(120, 96, 172, 294), yellow)
	fillRect(img, image.Rect(260, 96, 312, 294), yellow)
	fillRect(img, image.Rect(120, 174, 312, 218), yellow)
	fillRect(img, image.Rect(370, 96, 594, 144), yellow)
	fillRect(img, image.Rect(456, 96, 508, 294), yellow)
	fillRect(img, image.Rect(660, 96, 712, 294), yellow)
	fillRect(img, image.Rect(660, 246, 836, 294), yellow)
	fillRect(img, image.Rect(102, 326, 858, 342), white)
	for i := 0; i < 8; i++ {
		x := 130 + i*88
		fillRect(img, image.Rect(x, 354, x+52, 366), white)
		fillRect(img, image.Rect(x, 376, x+72, 388), white)
	}
	writeTestPNG(t, path, img)
	return path
}

func generatedMultiColorBlockLogo(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "multi-color-block-logo.png")
	img := image.NewRGBA(image.Rect(0, 0, 960, 420))
	white := color.RGBA{R: 0xfd, G: 0xfd, B: 0xfd, A: 0xff}
	blue := color.RGBA{R: 0x24, G: 0x58, B: 0xe6, A: 0xff}
	red := color.RGBA{R: 0xe6, G: 0x32, B: 0x23, A: 0xff}
	green := color.RGBA{R: 0x18, G: 0xa0, B: 0x58, A: 0xff}
	fillRect(img, image.Rect(0, 0, 960, 420), white)

	fillRect(img, image.Rect(105, 100, 255, 250), blue)
	fillRect(img, image.Rect(150, 145, 210, 205), white)
	fillRect(img, image.Rect(305, 100, 455, 250), red)
	fillRect(img, image.Rect(305, 100, 455, 142), green)
	fillRect(img, image.Rect(505, 100, 655, 250), green)
	fillRect(img, image.Rect(548, 142, 612, 208), white)
	fillRect(img, image.Rect(710, 100, 858, 142), blue)
	fillRect(img, image.Rect(762, 100, 806, 250), blue)
	for i := 0; i < 7; i++ {
		x := 120 + i*100
		fillRect(img, image.Rect(x, 310, x+58, 320), blue)
		fillRect(img, image.Rect(x, 336, x+72, 346), red)
		fillRect(img, image.Rect(x, 362, x+46, 372), green)
	}
	writeTestPNG(t, path, img)
	return path
}

func generatedMonochromeThinLineIcon(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "monochrome-thin-line-icon.png")
	img := image.NewRGBA(image.Rect(0, 0, 720, 420))
	white := color.RGBA{R: 0xfd, G: 0xfd, B: 0xfd, A: 0xff}
	black := color.RGBA{R: 0x11, G: 0x11, B: 0x11, A: 0xff}
	fillRect(img, image.Rect(0, 0, 720, 420), white)

	fillRect(img, image.Rect(150, 90, 570, 110), black)
	fillRect(img, image.Rect(150, 310, 570, 330), black)
	fillRect(img, image.Rect(150, 90, 170, 330), black)
	fillRect(img, image.Rect(550, 90, 570, 330), black)
	fillRect(img, image.Rect(245, 160, 475, 180), black)
	fillRect(img, image.Rect(245, 240, 475, 260), black)
	fillRect(img, image.Rect(245, 160, 265, 260), black)
	fillRect(img, image.Rect(455, 160, 475, 260), black)
	fillRect(img, image.Rect(335, 120, 385, 300), black)
	fillRect(img, image.Rect(300, 340, 420, 356), black)
	writeTestPNG(t, path, img)
	return path
}

func generatedTransparentLogo(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "transparent-logo.png")
	img := image.NewRGBA(image.Rect(0, 0, 720, 420))
	blue := color.RGBA{R: 0x24, G: 0x58, B: 0xe6, A: 0xff}
	green := color.RGBA{R: 0x18, G: 0xa0, B: 0x58, A: 0xff}

	fillRect(img, image.Rect(120, 120, 280, 280), blue)
	fillRect(img, image.Rect(200, 200, 360, 320), green)
	fillRect(img, image.Rect(430, 120, 470, 320), blue)
	fillRect(img, image.Rect(430, 280, 600, 320), blue)
	fillRect(img, image.Rect(500, 120, 600, 160), green)
	writeTestPNG(t, path, img)
	return path
}

func generatedSpeckledLogo(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "speckled-logo.png")
	img := image.NewRGBA(image.Rect(0, 0, 720, 420))
	white := color.RGBA{R: 0xfd, G: 0xfd, B: 0xfd, A: 0xff}
	black := color.RGBA{R: 0x13, G: 0x13, B: 0x13, A: 0xff}
	gray := color.RGBA{R: 0x6a, G: 0x6a, B: 0x6a, A: 0xff}
	fillRect(img, image.Rect(0, 0, 720, 420), white)

	fillRect(img, image.Rect(120, 120, 180, 300), black)
	fillRect(img, image.Rect(120, 120, 335, 170), black)
	fillRect(img, image.Rect(120, 250, 335, 300), black)
	fillRect(img, image.Rect(395, 120, 455, 300), black)
	fillRect(img, image.Rect(395, 120, 590, 170), black)
	fillRect(img, image.Rect(395, 250, 590, 300), black)
	for i := 0; i < 6; i++ {
		x := 130 + i*78
		fillRect(img, image.Rect(x, 335, x+46, 345), black)
		fillRect(img, image.Rect(x, 360, x+62, 370), black)
	}

	speckles := []image.Rectangle{
		image.Rect(44, 58, 46, 60),
		image.Rect(96, 350, 99, 353),
		image.Rect(230, 82, 232, 84),
		image.Rect(360, 338, 363, 341),
		image.Rect(640, 72, 642, 74),
		image.Rect(612, 352, 615, 355),
		image.Rect(48, 238, 51, 241),
		image.Rect(670, 228, 673, 231),
	}
	for i, rect := range speckles {
		fill := black
		if i%2 == 1 {
			fill = gray
		}
		fillRect(img, rect, fill)
	}
	writeTestPNG(t, path, img)
	return path
}

func generatedMicroTextLogo(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "micro-text-logo.png")
	largePath := filepath.Join(dir, "micro-text-logo-large.png")
	img := image.NewRGBA(image.Rect(0, 0, 1440, 840))
	white := color.RGBA{R: 0xfd, G: 0xfd, B: 0xfd, A: 0xff}
	black := color.RGBA{R: 0x13, G: 0x13, B: 0x13, A: 0xff}
	blue := color.RGBA{R: 0x24, G: 0x58, B: 0xe6, A: 0xff}
	fillRect(img, image.Rect(0, 0, 1440, 840), white)

	fillRect(img, image.Rect(190, 170, 310, 570), blue)
	fillRect(img, image.Rect(190, 170, 690, 270), blue)
	fillRect(img, image.Rect(190, 470, 690, 570), blue)
	fillRect(img, image.Rect(800, 170, 920, 570), black)
	fillRect(img, image.Rect(800, 470, 1190, 570), black)

	for row := 0; row < 3; row++ {
		y := 650 + row*46
		for col := 0; col < 28; col++ {
			x := 170 + col*38
			fillRect(img, image.Rect(x, y, x+8, y+22), black)
			fillRect(img, image.Rect(x+14, y, x+22, y+22), black)
			fillRect(img, image.Rect(x, y+28, x+28, y+36), black)
		}
	}

	writeTestPNG(t, largePath, img)
	magickPath, err := requireCommand("magick")
	if err != nil {
		t.Skipf("magick is not available: %v", err)
	}
	if output, err := exec.Command(magickPath, largePath, "-resize", "720x420!", "-colorspace", "sRGB", "-strip", path).CombinedOutput(); err != nil {
		t.Fatalf("resize generated micro-text sample: %v: %s", err, string(output))
	}
	return path
}

func generatedRealFontTextLogo(t *testing.T, dir string) string {
	t.Helper()
	fontPath := testFontPath(t)
	path := filepath.Join(dir, "real-font-text-logo.png")
	magickPath, err := requireCommand("magick")
	if err != nil {
		t.Skipf("magick is not available: %v", err)
	}
	args := []string{
		"-size", "1200x700",
		"xc:#fdfdfd",
		"-font", fontPath,
		"-fill", "#08758d",
		"-draw", "roundrectangle 110,120 360,370 40,40",
		"-fill", "#fdfdfd",
		"-pointsize", "132",
		"-gravity", "northwest",
		"-annotate", "+150+170", "AI",
		"-fill", "#08758d",
		"-pointsize", "112",
		"-annotate", "+410+138", "BRAND",
		"-fill", "#141414",
		"-pointsize", "42",
		"-annotate", "+418+286", "Precision Vector Service",
		"-fill", "#141414",
		"-pointsize", "26",
		"-annotate", "+420+358", "SMALL TEXT 1998  -  CLEAN EDGES",
		"-resize", "720x420!",
		"-colorspace", "sRGB",
		"-strip",
		path,
	}
	if output, err := exec.Command(magickPath, args...).CombinedOutput(); err != nil {
		t.Fatalf("generate real-font text sample: %v: %s", err, string(output))
	}
	return path
}

func generatedLongTaglineTextLogo(t *testing.T, dir string) string {
	t.Helper()
	fontPath := testFontPath(t)
	path := filepath.Join(dir, "long-tagline-text-logo.png")
	magickPath, err := requireCommand("magick")
	if err != nil {
		t.Skipf("magick is not available: %v", err)
	}
	args := []string{
		"-size", "1200x700",
		"xc:#fdfdfd",
		"-font", fontPath,
		"-fill", "#08758d",
		"-draw", "roundrectangle 90,120 310,360 36,36",
		"-fill", "#fdfdfd",
		"-pointsize", "118",
		"-gravity", "northwest",
		"-annotate", "+126+176", "V",
		"-fill", "#08758d",
		"-pointsize", "104",
		"-annotate", "+350+120", "VECTORIA",
		"-fill", "#141414",
		"-pointsize", "34",
		"-annotate", "+354+246", "Global Logistic Service",
		"-pointsize", "24",
		"-annotate", "+354+310", "PRECISION BRAND ASSETS  SINCE 1998  CLEAN EDGES",
		"-pointsize", "22",
		"-annotate", "+354+358", "SMALL TYPE SHOULD REMAIN READABLE AFTER SVG TRACE",
		"-fill", "#08758d",
		"-draw", "rectangle 354,430 980,452",
		"-resize", "720x420!",
		"-colorspace", "sRGB",
		"-strip",
		path,
	}
	if output, err := exec.Command(magickPath, args...).CombinedOutput(); err != nil {
		t.Fatalf("generate long-tagline text sample: %v: %s", err, string(output))
	}
	return path
}

func generatedSerifLogisticLogo(t *testing.T, dir string) string {
	t.Helper()
	fontPath := testSerifFontPath(t)
	path := filepath.Join(dir, "serif-logistic-logo.png")
	magickPath, err := requireCommand("magick")
	if err != nil {
		t.Skipf("magick is not available: %v", err)
	}
	args := []string{
		"-size", "1536x1024",
		"xc:#fdfdfd",
		"-fill", "#c4d9e3",
		"-draw", "path 'M 270,475 C 470,590 650,610 860,535 C 1080,455 1285,450 1440,535 L 1440,650 C 1200,545 1015,540 820,610 C 590,690 405,655 270,600 Z'",
		"-fill", "#ecf3f6",
		"-draw", "path 'M 340,520 C 560,610 760,600 950,540 C 1135,485 1290,500 1440,565 L 1440,600 C 1255,535 1115,530 950,585 C 735,655 560,655 340,570 Z'",
		"-font", fontPath,
		"-fill", "#00768e",
		"-pointsize", "300",
		"-gravity", "northwest",
		"-annotate", "+110+70", "30",
		"-fill", "#00768e",
		"-pointsize", "72",
		"-annotate", "+300+246", "YEARS",
		"-fill", "#00768e",
		"-pointsize", "300",
		"-annotate", "+600+320", "BLS",
		"-fill", "#00768e",
		"-pointsize", "96",
		"-annotate", "+335+720", "Bremer Logistic Service",
		"-resize", "768x512!",
		"-colorspace", "sRGB",
		"-strip",
		path,
	}
	if output, err := exec.Command(magickPath, args...).CombinedOutput(); err != nil {
		t.Fatalf("generate serif logistic logo sample: %v: %s", err, string(output))
	}
	return path
}

func generatedLowContrastTaglineLogo(t *testing.T, dir string) string {
	t.Helper()
	fontPath := testFontPath(t)
	path := filepath.Join(dir, "low-contrast-tagline-logo.png")
	magickPath, err := requireCommand("magick")
	if err != nil {
		t.Skipf("magick is not available: %v", err)
	}
	args := []string{
		"-size", "1200x700",
		"xc:#fdfdfd",
		"-font", fontPath,
		"-fill", "#08758d",
		"-draw", "roundrectangle 95,128 305,358 38,38",
		"-fill", "#fdfdfd",
		"-pointsize", "112",
		"-gravity", "northwest",
		"-annotate", "+132+182", "L",
		"-fill", "#08758d",
		"-pointsize", "100",
		"-annotate", "+350+128", "LUMEN",
		"-fill", "#777777",
		"-pointsize", "30",
		"-annotate", "+354+252", "Soft gray tagline must stay visible",
		"-pointsize", "24",
		"-annotate", "+354+310", "LOW CONTRAST SERVICE MARK  2026  FINE TEXT",
		"-fill", "#08758d",
		"-draw", "rectangle 354,426 900,448",
		"-resize", "720x420!",
		"-colorspace", "sRGB",
		"-strip",
		path,
	}
	if output, err := exec.Command(magickPath, args...).CombinedOutput(); err != nil {
		t.Fatalf("generate low-contrast tagline sample: %v: %s", err, string(output))
	}
	return path
}

func generatedCJKRealFontTextLogo(t *testing.T, dir string) string {
	t.Helper()
	fontPath := testCJKFontPath(t)
	path := filepath.Join(dir, "cjk-real-font-text-logo.png")
	magickPath, err := requireCommand("magick")
	if err != nil {
		t.Skipf("magick is not available: %v", err)
	}
	args := []string{
		"-size", "1200x700",
		"xc:#fdfdfd",
		"-font", fontPath,
		"-fill", "#08758d",
		"-draw", "roundrectangle 115,120 360,370 42,42",
		"-fill", "#fdfdfd",
		"-pointsize", "118",
		"-gravity", "northwest",
		"-annotate", "+155+178", "图",
		"-fill", "#08758d",
		"-pointsize", "102",
		"-annotate", "+410+132", "好图秀",
		"-fill", "#141414",
		"-pointsize", "42",
		"-annotate", "+418+286", "智能商品图生成",
		"-fill", "#141414",
		"-pointsize", "28",
		"-annotate", "+420+360", "中文小字 2026  -  边缘清晰",
		"-resize", "720x420!",
		"-colorspace", "sRGB",
		"-strip",
		path,
	}
	if output, err := exec.Command(magickPath, args...).CombinedOutput(); err != nil {
		t.Fatalf("generate cjk real-font text sample: %v: %s", err, string(output))
	}
	return path
}

func generatedFlatCJKSloganBanner(t *testing.T, dir string) string {
	t.Helper()
	fontPath := testCJKFontPath(t)
	path := filepath.Join(dir, "flat-cjk-slogan-banner.png")
	magickPath, err := requireCommand("magick")
	if err != nil {
		t.Skipf("magick is not available: %v", err)
	}
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
		"-font", fontPath,
		"-pointsize", "104",
		"-gravity", "north",
		"-annotate", "+0+92", "人民有信仰  国家有力量  民族有希望",
		"-fill", "#15547a",
		"-draw", "rectangle 1848,122 1878,268",
		"-draw", "polygon 1828,268 1898,268 1884,286 1842,286",
		"-draw", "rectangle 1860,92 1868,126",
		"-draw", "circle 1864,88 1864,76",
		"-fill", "#eef4f8",
		"-draw", "rectangle 1858,142 1868,176",
		"-resize", "1024x180!",
		"-colorspace", "sRGB",
		"-strip",
		path,
	}
	if output, err := exec.Command(magickPath, args...).CombinedOutput(); err != nil {
		t.Fatalf("generate flat cjk slogan banner sample: %v: %s", err, string(output))
	}
	return path
}

func generatedDarkRealFontTextLogo(t *testing.T, dir string) string {
	t.Helper()
	fontPath := testFontPath(t)
	path := filepath.Join(dir, "dark-real-font-text-logo.png")
	magickPath, err := requireCommand("magick")
	if err != nil {
		t.Skipf("magick is not available: %v", err)
	}
	args := []string{
		"-size", "1200x700",
		"xc:#0f172a",
		"-font", fontPath,
		"-fill", "#38bdf8",
		"-draw", "roundrectangle 110,120 360,370 40,40",
		"-fill", "#0f172a",
		"-pointsize", "132",
		"-gravity", "northwest",
		"-annotate", "+150+170", "AI",
		"-fill", "#f4f4f4",
		"-pointsize", "104",
		"-annotate", "+410+140", "NOVA",
		"-fill", "#f4f4f4",
		"-pointsize", "40",
		"-annotate", "+418+286", "Light Text Vector Service",
		"-fill", "#f4f4f4",
		"-pointsize", "25",
		"-annotate", "+420+358", "SMALL WHITE TEXT 2026  -  CLEAN EDGES",
		"-resize", "720x420!",
		"-colorspace", "sRGB",
		"-strip",
		path,
	}
	if output, err := exec.Command(magickPath, args...).CombinedOutput(); err != nil {
		t.Fatalf("generate dark real-font text sample: %v: %s", err, string(output))
	}
	return path
}

func testFontPath(t *testing.T) string {
	t.Helper()
	candidates := []string{
		"/System/Library/Fonts/HelveticaNeue.ttc",
		"/System/Library/Fonts/SFNS.ttf",
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		"/usr/share/fonts/truetype/liberation2/LiberationSans-Regular.ttf",
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	t.Skip("no usable TrueType/OpenType font found for real-font text logo sample")
	return ""
}

func testSerifFontPath(t *testing.T) string {
	t.Helper()
	candidates := []string{
		"/System/Library/Fonts/Times.ttc",
		"/System/Library/Fonts/Supplemental/Times New Roman.ttf",
		"/System/Library/Fonts/Supplemental/Georgia.ttf",
		"/usr/share/fonts/truetype/dejavu/DejaVuSerif.ttf",
		"/usr/share/fonts/truetype/liberation2/LiberationSerif-Regular.ttf",
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	t.Skip("no usable serif TrueType/OpenType font found for logistic logo sample")
	return ""
}

func testCJKFontPath(t *testing.T) string {
	t.Helper()
	candidates := []string{
		"/System/Library/Fonts/STHeiti Medium.ttc",
		"/System/Library/Fonts/STHeiti Light.ttc",
		"/System/Library/Fonts/Supplemental/Songti.ttc",
		"/System/Library/Fonts/Supplemental/Arial Unicode.ttf",
		"/Library/Fonts/Arial Unicode.ttf",
		"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	t.Skip("no usable CJK font found for real-font text logo sample")
	return ""
}

func generatedRingDiagonalLogo(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "ring-diagonal-logo.png")
	largePath := filepath.Join(dir, "ring-diagonal-logo-large.png")
	magickPath, err := requireCommand("magick")
	if err != nil {
		t.Skipf("magick is not available: %v", err)
	}
	args := []string{
		"-size", "1440x840",
		"xc:#fdfdfd",
		"-fill", "#08758d",
		"-draw", "circle 430,420 430,185",
		"-fill", "#fdfdfd",
		"-draw", "circle 430,420 430,305",
		"-fill", "#e63223",
		"-draw", "polygon 760,190 1190,190 1010,650 580,650",
		"-fill", "#fdfdfd",
		"-draw", "polygon 845,310 1035,310 945,530 755,530",
		largePath,
	}
	if output, err := exec.Command(magickPath, args...).CombinedOutput(); err != nil {
		t.Fatalf("draw generated ring-diagonal sample: %v: %s", err, string(output))
	}
	if output, err := exec.Command(magickPath, largePath, "-resize", "720x420!", "-colorspace", "sRGB", "-strip", path).CombinedOutput(); err != nil {
		t.Fatalf("resize generated ring-diagonal sample: %v: %s", err, string(output))
	}
	return path
}

func generatedSoftShadowLogo(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "soft-shadow-logo.png")
	largePath := filepath.Join(dir, "soft-shadow-logo-large.png")
	magickPath, err := requireCommand("magick")
	if err != nil {
		t.Skipf("magick is not available: %v", err)
	}
	args := []string{
		"-size", "1440x840",
		"xc:#fdfdfd",
		"-fill", "#c8c8c8",
		"-draw", "roundrectangle 338,258 928,588 54,54",
		"-fill", "#2458e6",
		"-draw", "roundrectangle 300,220 890,550 54,54",
		"-fill", "#fdfdfd",
		"-draw", "roundrectangle 430,335 760,435 24,24",
		"-fill", "#c8c8c8",
		"-draw", "rectangle 320,650 1000,670",
		"-draw", "rectangle 320,710 840,730",
		largePath,
	}
	if output, err := exec.Command(magickPath, args...).CombinedOutput(); err != nil {
		t.Fatalf("draw generated soft-shadow sample: %v: %s", err, string(output))
	}
	if output, err := exec.Command(magickPath, largePath, "-resize", "720x420!", "-colorspace", "sRGB", "-strip", path).CombinedOutput(); err != nil {
		t.Fatalf("resize generated soft-shadow sample: %v: %s", err, string(output))
	}
	return path
}

func generatedJPEGArtifactLogo(t *testing.T, dir string) string {
	t.Helper()
	pngPath := filepath.Join(dir, "jpeg-artifact-logo-source.png")
	jpegPath := filepath.Join(dir, "jpeg-artifact-logo.jpg")
	img := image.NewRGBA(image.Rect(0, 0, 720, 420))
	white := color.RGBA{R: 0xfd, G: 0xfd, B: 0xfd, A: 0xff}
	blue := color.RGBA{R: 0x24, G: 0x58, B: 0xe6, A: 0xff}
	red := color.RGBA{R: 0xe6, G: 0x32, B: 0x23, A: 0xff}
	fillRect(img, image.Rect(0, 0, 720, 420), white)
	fillRect(img, image.Rect(100, 90, 310, 285), blue)
	fillRect(img, image.Rect(165, 150, 245, 225), white)
	fillRect(img, image.Rect(385, 90, 620, 140), red)
	fillRect(img, image.Rect(385, 175, 620, 225), red)
	fillRect(img, image.Rect(385, 260, 620, 310), red)
	for i := 0; i < 8; i++ {
		x := 105 + i*64
		fillRect(img, image.Rect(x, 345, x+42, 355), blue)
		fillRect(img, image.Rect(x, 370, x+54, 380), red)
	}
	writeTestPNG(t, pngPath, img)
	magickPath, err := requireCommand("magick")
	if err != nil {
		t.Skipf("magick is not available: %v", err)
	}
	if output, err := exec.Command(magickPath, pngPath, "-sampling-factor", "4:2:0", "-quality", "72", jpegPath).CombinedOutput(); err != nil {
		t.Fatalf("write generated jpeg-artifact sample: %v: %s", err, string(output))
	}
	return jpegPath
}

func generatedThinOutlineLogo(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "thin-outline-logo.png")
	largePath := filepath.Join(dir, "thin-outline-logo-large.png")
	magickPath, err := requireCommand("magick")
	if err != nil {
		t.Skipf("magick is not available: %v", err)
	}
	args := []string{
		"-size", "1440x840",
		"xc:#fdfdfd",
		"-fill", "none",
		"-stroke", "#08758d",
		"-strokewidth", "32",
		"-draw", "circle 360,330 360,150",
		"-draw", "roundrectangle 690,160 1180,500 36,36",
		"-strokewidth", "24",
		"-draw", "line 250,650 1110,650",
		"-draw", "line 250,710 960,710",
		"-strokewidth", "20",
		"-draw", "line 790,245 1080,245",
		"-draw", "line 790,335 1080,335",
		"-draw", "line 790,425 1000,425",
		largePath,
	}
	if output, err := exec.Command(magickPath, args...).CombinedOutput(); err != nil {
		t.Fatalf("draw generated thin-outline sample: %v: %s", err, string(output))
	}
	if output, err := exec.Command(magickPath, largePath, "-resize", "720x420!", "-colorspace", "sRGB", "-strip", path).CombinedOutput(); err != nil {
		t.Fatalf("resize generated thin-outline sample: %v: %s", err, string(output))
	}
	return path
}

func generatedTintedBackgroundLogo(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "tinted-background-logo.png")
	img := image.NewRGBA(image.Rect(0, 0, 720, 420))
	bg := color.RGBA{R: 0xe7, G: 0xf8, B: 0xf2, A: 0xff}
	green := color.RGBA{R: 0x0f, G: 0x76, B: 0x6e, A: 0xff}
	orange := color.RGBA{R: 0xf9, G: 0x73, B: 0x16, A: 0xff}
	dark := color.RGBA{R: 0x1f, G: 0x29, B: 0x37, A: 0xff}
	fillRect(img, image.Rect(0, 0, 720, 420), bg)
	fillRect(img, image.Rect(120, 95, 300, 275), green)
	fillRect(img, image.Rect(175, 150, 245, 220), bg)
	fillRect(img, image.Rect(365, 95, 600, 145), orange)
	fillRect(img, image.Rect(365, 185, 600, 235), orange)
	fillRect(img, image.Rect(365, 275, 520, 325), orange)
	for i := 0; i < 6; i++ {
		x := 125 + i*78
		fillRect(img, image.Rect(x, 352, x+48, 362), dark)
		fillRect(img, image.Rect(x, 378, x+66, 388), dark)
	}
	writeTestPNG(t, path, img)
	return path
}

func generatedKnockoutMicroHolesLogo(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "knockout-micro-holes-logo.png")
	img := image.NewRGBA(image.Rect(0, 0, 720, 420))
	white := color.RGBA{R: 0xfd, G: 0xfd, B: 0xfd, A: 0xff}
	blue := color.RGBA{R: 0x24, G: 0x58, B: 0xe6, A: 0xff}
	orange := color.RGBA{R: 0xf9, G: 0x73, B: 0x16, A: 0xff}
	fillRect(img, image.Rect(0, 0, 720, 420), white)
	fillRect(img, image.Rect(105, 95, 390, 285), blue)
	fillRect(img, image.Rect(450, 95, 610, 285), orange)

	for row := 0; row < 3; row++ {
		y := 130 + row*48
		for col := 0; col < 6; col++ {
			x := 140 + col*38
			fillRect(img, image.Rect(x, y, x+10, y+24), white)
			fillRect(img, image.Rect(x+17, y, x+27, y+24), white)
			fillRect(img, image.Rect(x, y+32, x+30, y+42), white)
		}
	}
	fillRect(img, image.Rect(500, 135, 560, 195), white)
	fillRect(img, image.Rect(520, 155, 540, 175), orange)
	fillRect(img, image.Rect(490, 225, 570, 245), white)
	fillRect(img, image.Rect(520, 205, 540, 265), white)
	for i := 0; i < 7; i++ {
		x := 120 + i*70
		fillRect(img, image.Rect(x, 340, x+44, 350), blue)
		fillRect(img, image.Rect(x, 365, x+56, 375), orange)
	}
	writeTestPNG(t, path, img)
	return path
}

func generatedWhiteTextOnBrandBlockLogo(t *testing.T, dir string) string {
	t.Helper()
	fontPath := testFontPath(t)
	path := filepath.Join(dir, "white-text-brand-block-logo.png")
	magickPath, err := requireCommand("magick")
	if err != nil {
		t.Skipf("magick is not available: %v", err)
	}
	args := []string{
		"-size", "1200x700",
		"xc:#fdfdfd",
		"-font", fontPath,
		"-fill", "#08758d",
		"-draw", "roundrectangle 120,120 900,420 50,50",
		"-fill", "#fdfdfd",
		"-pointsize", "128",
		"-gravity", "northwest",
		"-annotate", "+188+198", "WHITE",
		"-pointsize", "62",
		"-annotate", "+198+340", "TEXT KNOCKOUT",
		"-fill", "#08758d",
		"-pointsize", "44",
		"-annotate", "+130+500", "Brand block text should stay open",
		"-resize", "720x420!",
		"-colorspace", "sRGB",
		"-strip",
		path,
	}
	if output, err := exec.Command(magickPath, args...).CombinedOutput(); err != nil {
		t.Fatalf("generate white-text brand-block sample: %v: %s", err, string(output))
	}
	return path
}

func generatedSemiTransparentShadowLogo(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "semi-transparent-shadow-logo.png")
	img := image.NewRGBA(image.Rect(0, 0, 720, 420))
	shadow := color.RGBA{R: 0x80, G: 0x80, B: 0x80, A: 0x50}
	blue := color.RGBA{R: 0x24, G: 0x58, B: 0xe6, A: 0xff}
	orange := color.RGBA{R: 0xf9, G: 0x73, B: 0x16, A: 0xff}
	white := color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	fillRect(img, image.Rect(140, 130, 370, 290), shadow)
	fillRect(img, image.Rect(400, 142, 620, 302), shadow)
	fillRect(img, image.Rect(110, 100, 340, 260), blue)
	fillRect(img, image.Rect(175, 145, 275, 215), white)
	fillRect(img, image.Rect(370, 112, 590, 272), orange)
	fillRect(img, image.Rect(430, 160, 530, 224), white)
	for i := 0; i < 6; i++ {
		x := 120 + i*80
		fillRect(img, image.Rect(x+8, 342, x+62, 354), shadow)
		fillRect(img, image.Rect(x, 334, x+54, 346), blue)
		fillRect(img, image.Rect(x, 370, x+66, 382), orange)
	}
	writeTestPNG(t, path, img)
	return path
}

func generatedNarrowGapLogo(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "narrow-gap-logo.png")
	img := image.NewRGBA(image.Rect(0, 0, 720, 420))
	white := color.RGBA{R: 0xfd, G: 0xfd, B: 0xfd, A: 0xff}
	blue := color.RGBA{R: 0x24, G: 0x58, B: 0xe6, A: 0xff}
	red := color.RGBA{R: 0xe6, G: 0x32, B: 0x23, A: 0xff}
	green := color.RGBA{R: 0x18, G: 0xa0, B: 0x58, A: 0xff}
	fillRect(img, image.Rect(0, 0, 720, 420), white)
	fillRect(img, image.Rect(110, 95, 305, 290), blue)
	fillRect(img, image.Rect(321, 95, 516, 290), red)
	fillRect(img, image.Rect(532, 95, 625, 290), green)
	fillRect(img, image.Rect(184, 168, 232, 216), white)
	fillRect(img, image.Rect(392, 168, 444, 216), white)
	fillRect(img, image.Rect(560, 168, 596, 216), white)
	fillRect(img, image.Rect(305, 95, 321, 290), white)
	fillRect(img, image.Rect(516, 95, 532, 290), white)
	for i := 0; i < 8; i++ {
		x := 105 + i*66
		fillRect(img, image.Rect(x, 340, x+46, 350), blue)
		fillRect(img, image.Rect(x, 365, x+56, 375), red)
	}
	writeTestPNG(t, path, img)
	return path
}

func generatedSmallMarksLogo(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "small-marks-logo.png")
	largePath := filepath.Join(dir, "small-marks-logo-large.png")
	magickPath, err := requireCommand("magick")
	if err != nil {
		t.Skipf("magick is not available: %v", err)
	}
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
		"-draw", "rectangle 1000,176 1024,184",
		"-draw", "rectangle 1000,190 1026,198",
		"-draw", "circle 1065,185 1065,172",
		"-draw", "circle 1110,185 1110,174",
		"-draw", "circle 1150,185 1150,176",
		"-draw", "rectangle 260,610 365,630",
		"-draw", "rectangle 420,610 548,630",
		"-draw", "rectangle 610,610 700,630",
		largePath,
	}
	if output, err := exec.Command(magickPath, args...).CombinedOutput(); err != nil {
		t.Fatalf("draw generated small-marks sample: %v: %s", err, string(output))
	}
	if output, err := exec.Command(magickPath, largePath, "-resize", "720x420!", "-colorspace", "sRGB", "-strip", path).CombinedOutput(); err != nil {
		t.Fatalf("resize generated small-marks sample: %v: %s", err, string(output))
	}
	return path
}

func generatedEdgeAlignedLogo(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "edge-aligned-logo.png")
	img := image.NewRGBA(image.Rect(0, 0, 720, 420))
	white := color.RGBA{R: 0xfd, G: 0xfd, B: 0xfd, A: 0xff}
	blue := color.RGBA{R: 0x24, G: 0x58, B: 0xe6, A: 0xff}
	orange := color.RGBA{R: 0xf9, G: 0x73, B: 0x16, A: 0xff}
	fillRect(img, image.Rect(0, 0, 720, 420), white)
	fillRect(img, image.Rect(0, 90, 86, 300), blue)
	fillRect(img, image.Rect(140, 90, 360, 150), blue)
	fillRect(img, image.Rect(140, 185, 360, 245), blue)
	fillRect(img, image.Rect(420, 90, 650, 300), orange)
	fillRect(img, image.Rect(495, 155, 575, 235), white)
	fillRect(img, image.Rect(70, 388, 620, 420), blue)
	fillRect(img, image.Rect(110, 342, 510, 360), orange)
	writeTestPNG(t, path, img)
	return path
}

func generatedSameHueGradientLogo(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "same-hue-gradient-logo.png")
	img := image.NewRGBA(image.Rect(0, 0, 720, 420))
	white := color.RGBA{R: 0xfd, G: 0xfd, B: 0xfd, A: 0xff}
	dark := color.RGBA{R: 0x00, G: 0x6d, B: 0x87, A: 0xff}
	mid := color.RGBA{R: 0x09, G: 0x82, B: 0x9d, A: 0xff}
	light := color.RGBA{R: 0x40, G: 0xa9, B: 0xba, A: 0xff}
	fillRect(img, image.Rect(0, 0, 720, 420), white)
	for y := 90; y < 250; y++ {
		var fill color.RGBA
		switch {
		case y < 145:
			fill = light
		case y < 200:
			fill = mid
		default:
			fill = dark
		}
		fillRect(img, image.Rect(110, y, 610, y+1), fill)
	}
	fillRect(img, image.Rect(175, 145, 545, 195), white)
	fillRect(img, image.Rect(110, 275, 610, 305), dark)
	fillRect(img, image.Rect(145, 330, 325, 352), mid)
	fillRect(img, image.Rect(370, 330, 575, 352), light)
	writeTestPNG(t, path, img)
	return path
}

func generatedWhiteTextOffWhiteLogo(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "white-text-off-white-logo.png")
	img := image.NewRGBA(image.Rect(0, 0, 720, 420))
	background := color.RGBA{R: 0xf3, G: 0xf4, B: 0xf6, A: 0xff}
	blue := color.RGBA{R: 0x12, G: 0x3a, B: 0x5a, A: 0xff}
	white := color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	fillRect(img, image.Rect(0, 0, 720, 420), background)
	fillRect(img, image.Rect(105, 95, 615, 270), blue)
	fillRect(img, image.Rect(170, 145, 270, 190), white)
	fillRect(img, image.Rect(305, 145, 405, 190), white)
	fillRect(img, image.Rect(440, 145, 555, 190), white)
	fillRect(img, image.Rect(115, 300, 605, 326), blue)
	fillRect(img, image.Rect(165, 350, 260, 370), white)
	fillRect(img, image.Rect(315, 350, 405, 370), white)
	fillRect(img, image.Rect(465, 350, 560, 370), white)
	writeTestPNG(t, path, img)
	return path
}

func fillRect(img *image.RGBA, rect image.Rectangle, fill color.Color) {
	draw.Draw(img, rect, &image.Uniform{C: fill}, image.Point{}, draw.Src)
}

func writeTestPNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create generated sample: %v", err)
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		t.Fatalf("write generated sample: %v", err)
	}
}

func sampleSourcePath(fixtureName string, fallbackID string) string {
	candidates := []string{
		filepath.Join("fixtures", fixtureName),
		filepath.Join("demos", "svg", "fixtures", fixtureName),
		filepath.Join("outputs", fallbackID, "source.png"),
		filepath.Join("demos", "svg", "outputs", fallbackID, "source.png"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return candidates[0]
}
