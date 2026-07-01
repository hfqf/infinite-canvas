package service

import (
	"context"
	"encoding/base64"
	"fmt"
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

	"github.com/basketikun/infinite-canvas/config"

	_ "image/jpeg"
)

func TestPng2SVGCleanArgsUsesDefaultProfile(t *testing.T) {
	originalProfile := config.Cfg.Png2SVGCleanProfile
	originalBin := config.Cfg.Png2SVGCleanBin
	t.Cleanup(func() {
		config.Cfg.Png2SVGCleanProfile = originalProfile
		config.Cfg.Png2SVGCleanBin = originalBin
	})
	config.Cfg.Png2SVGCleanProfile = ""
	config.Cfg.Png2SVGCleanBin = ""

	got := png2SVGCleanArgs("input.png", "output.svg")
	want := []string{"bin/png2svg-generic-85.mjs", "input.png", "output.svg", "--profile", "generic-85"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("png2SVGCleanArgs()=%#v, want %#v", got, want)
	}
}

func TestPng2SVGCleanArgsUsesConfiguredProfile(t *testing.T) {
	originalProfile := config.Cfg.Png2SVGCleanProfile
	originalBin := config.Cfg.Png2SVGCleanBin
	t.Cleanup(func() {
		config.Cfg.Png2SVGCleanProfile = originalProfile
		config.Cfg.Png2SVGCleanBin = originalBin
	})
	config.Cfg.Png2SVGCleanProfile = "custom-profile"
	config.Cfg.Png2SVGCleanBin = "bin/custom.mjs"

	got := png2SVGCleanArgs("input.png", "output.svg")
	want := []string{"bin/custom.mjs", "input.png", "output.svg", "--profile", "custom-profile"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("png2SVGCleanArgs()=%#v, want %#v", got, want)
	}
}

func TestVectorizeEngineUsesPng2SVGCleanNode(t *testing.T) {
	if got := vectorizeEngine("colorMask"); got != "png2svg-clean-node" {
		t.Fatalf("vectorizeEngine()=%q, want png2svg-clean-node", got)
	}
}

func TestVectorizeEngineUsesCleanLogoPotraceForLogoMode(t *testing.T) {
	for _, mode := range []string{"logo", "cleanLogo", "clean-logo"} {
		if got := vectorizeEngine(mode); got != "clean-logo-potrace" {
			t.Fatalf("vectorizeEngine(%q)=%q, want clean-logo-potrace", mode, got)
		}
	}
}

func TestVectorizeEngineUsesIllustrationBranchForIllustrationMode(t *testing.T) {
	for _, mode := range []string{"illustration", "material", "asset"} {
		if got := vectorizeEngine(mode); got != "illustration-potrace" {
			t.Fatalf("vectorizeEngine(%q)=%q, want illustration-potrace", mode, got)
		}
	}
}

func TestVectorizePresetReturnsIllustrationParameters(t *testing.T) {
	got := vectorizePreset("illustration")
	if got == nil {
		t.Fatal("vectorizePreset(illustration)=nil, want illustration preset")
	}
	if got.Name != "illustration" {
		t.Fatalf("preset name = %q, want illustration", got.Name)
	}
	if got.Engine != "illustration-potrace" {
		t.Fatalf("preset engine = %q, want illustration-potrace", got.Engine)
	}
	if got.LongEdge != 1024 {
		t.Fatalf("preset longEdge = %d, want 1024", got.LongEdge)
	}
	if got.Colors != 64 {
		t.Fatalf("preset colors = %d, want 64", got.Colors)
	}
	if got.MinComponentRatio != 0.000005 {
		t.Fatalf("preset minComponentRatio = %.8f, want 0.000005", got.MinComponentRatio)
	}
	if got.MaxHoleRatio != 0.00001 {
		t.Fatalf("preset maxHoleRatio = %.8f, want 0.00001", got.MaxHoleRatio)
	}
	if got.MergeDistance != 18 {
		t.Fatalf("preset mergeDistance = %d, want 18", got.MergeDistance)
	}
	if got.MergeHueDistance != 8 {
		t.Fatalf("preset mergeHueDistance = %d, want 8", got.MergeHueDistance)
	}
	if got.MergeLightness != 32 {
		t.Fatalf("preset mergeLightness = %d, want 32", got.MergeLightness)
	}
	if got.MergeSaturation != 0.3 {
		t.Fatalf("preset mergeSaturation = %.2f, want 0.3", got.MergeSaturation)
	}
	if got.LightMinAreaRatio != 0.0005 {
		t.Fatalf("preset lightMinAreaRatio = %.6f, want 0.0005", got.LightMinAreaRatio)
	}
}

func TestIllustrationTraceLayersKeepSimilarPaletteColorsSeparate(t *testing.T) {
	palette := []cleanLogoPaletteColor{
		{Hex: "#FFFFFF", R: 255, G: 255, B: 255, Count: 5000},
		{Hex: "#F7B61F", R: 247, G: 182, B: 31, Count: 1000},
		{Hex: "#F6C04E", R: 246, G: 192, B: 78, Count: 900},
		{Hex: "#FFD56A", R: 255, G: 213, B: 106, Count: 800},
	}
	backgroundKeys := map[uint32]struct{}{cleanLogoRGBKey(255, 255, 255): {}}

	layers := cleanLogoBuildTraceLayersWithOptions(palette, 7700, backgroundKeys, illustrationVectorizeOptions())

	if len(layers) != 3 {
		t.Fatalf("illustration layers = %d, want 3 separate yellow layers: %#v", len(layers), layers)
	}
}

func TestIllustrationPotraceVectorizePreservesSimilarMaterialColors(t *testing.T) {
	if _, err := requireVectorizeCommand(config.Cfg.ImageMagickPath, "magick"); err != nil {
		t.Skipf("magick is not available: %v", err)
	}
	if _, err := requireVectorizeCommand(config.Cfg.PotracePath, "potrace"); err != nil {
		t.Skipf("potrace is not available: %v", err)
	}
	dir := t.TempDir()
	inputPath := vectorizeGeneratedIllustrationMaterial(t, dir)
	outputPath := filepath.Join(dir, "illustration.svg")
	options := illustrationVectorizeOptions()
	options.LongEdge = 360

	if err := runCleanLogoPotraceVectorize(context.Background(), inputPath, outputPath, options); err != nil {
		t.Fatalf("runCleanLogoPotraceVectorize() error = %v", err)
	}
	svg, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read illustration svg: %v", err)
	}
	for _, fill := range []string{"#F7B61F", "#F6C04E", "#FFD56A"} {
		if !vectorizeContainsFill(string(svg), fill) {
			t.Fatalf("svg does not preserve fill %s; fills=%v", fill, vectorizeSVGFillValues(string(svg)))
		}
	}
}

func TestVectorizePresetReturnsCleanLogoParametersForLogoModes(t *testing.T) {
	for _, mode := range []string{"logo", "cleanLogo", "clean-logo"} {
		got := vectorizePreset(mode)
		if got == nil {
			t.Fatalf("vectorizePreset(%q)=nil, want clean logo preset", mode)
		}
		if got.Name != "cleanLogo" {
			t.Fatalf("preset name = %q, want cleanLogo", got.Name)
		}
		if got.Engine != "clean-logo-potrace" {
			t.Fatalf("preset engine = %q, want clean-logo-potrace", got.Engine)
		}
		if got.LongEdge != 1024 {
			t.Fatalf("preset longEdge = %d, want 1024", got.LongEdge)
		}
		if got.MinComponentRatio != 0.00005 {
			t.Fatalf("preset minComponentRatio = %.8f, want 0.00005", got.MinComponentRatio)
		}
		if got.MaxHoleRatio != 0.00008 {
			t.Fatalf("preset maxHoleRatio = %.8f, want 0.00008", got.MaxHoleRatio)
		}
		if got.MergeDistance != 64 {
			t.Fatalf("preset mergeDistance = %d, want 64", got.MergeDistance)
		}
		if got.MergeHueDistance != 16 {
			t.Fatalf("preset mergeHueDistance = %d, want 16", got.MergeHueDistance)
		}
		if got.MergeLightness != 80 {
			t.Fatalf("preset mergeLightness = %d, want 80", got.MergeLightness)
		}
		if got.MergeSaturation != 0.55 {
			t.Fatalf("preset mergeSaturation = %.2f, want 0.55", got.MergeSaturation)
		}
		if got.LightMinAreaRatio != 0.004 {
			t.Fatalf("preset lightMinAreaRatio = %.6f, want 0.004", got.LightMinAreaRatio)
		}
		if got.MaskCloseRadius != 0 || got.LightDilateRadius != 0 || got.DarkDilateRadius != 0 {
			t.Fatalf("preset radii = mask:%d light:%d dark:%d, want all 0", got.MaskCloseRadius, got.LightDilateRadius, got.DarkDilateRadius)
		}
	}
}

func TestVectorizePresetReturnsNilForGeneralMode(t *testing.T) {
	if got := vectorizePreset("general"); got != nil {
		t.Fatalf("vectorizePreset(general)=%#v, want nil", got)
	}
}

func TestCleanLogoProductionConstantsMatchRecommendedPreset(t *testing.T) {
	if cleanLogoLongEdge != 1024 {
		t.Fatalf("cleanLogoLongEdge = %d, want 1024", cleanLogoLongEdge)
	}
	if cleanLogoMinComponentRatio != 0.00005 {
		t.Fatalf("cleanLogoMinComponentRatio = %.8f, want 0.00005", cleanLogoMinComponentRatio)
	}
	if cleanLogoMaxHoleRatio != 0.00008 {
		t.Fatalf("cleanLogoMaxHoleRatio = %.8f, want 0.00008", cleanLogoMaxHoleRatio)
	}
	if cleanLogoMergeDistance != 64 {
		t.Fatalf("cleanLogoMergeDistance = %d, want 64", cleanLogoMergeDistance)
	}
	if cleanLogoMergeHueDistance != 16 {
		t.Fatalf("cleanLogoMergeHueDistance = %d, want 16", cleanLogoMergeHueDistance)
	}
	if cleanLogoMergeLightness != 80 {
		t.Fatalf("cleanLogoMergeLightness = %d, want 80", cleanLogoMergeLightness)
	}
	if cleanLogoMergeSaturation != 0.55 {
		t.Fatalf("cleanLogoMergeSaturation = %.2f, want 0.55", cleanLogoMergeSaturation)
	}
	if cleanLogoLightMinAreaRatio != 0.004 {
		t.Fatalf("cleanLogoLightMinAreaRatio = %.6f, want 0.004", cleanLogoLightMinAreaRatio)
	}
}

func TestVectorizeImageLogoModeUsesCleanLogoPreset(t *testing.T) {
	if _, err := requireVectorizeCommand(config.Cfg.ImageMagickPath, "magick"); err != nil {
		t.Skipf("magick is not available: %v", err)
	}
	if _, err := requireVectorizeCommand(config.Cfg.PotracePath, "potrace"); err != nil {
		t.Skipf("potrace is not available: %v", err)
	}
	generatedDir := t.TempDir()

	tests := []struct {
		name            string
		samplePath      string
		mime            string
		wantColors      int
		maxPaths        int
		maxSubpaths     int
		maxRMSE         float64
		maxColorDelta   float64
		maxBgDelta      float64
		maxFgColorDelta float64
		maxFgDelta      float64
		requiredFills   []string
		anyFills        []string
	}{
		{
			name:            "BLS logo keeps light accent and exact teal colors",
			samplePath:      vectorizeSampleSourcePath("bls-logo.png", "d0454657e509e31a"),
			wantColors:      16,
			maxPaths:        3,
			maxSubpaths:     80,
			maxRMSE:         0.05,
			maxColorDelta:   12,
			maxBgDelta:      0,
			maxFgColorDelta: 1,
			maxFgDelta:      0.04,
			anyFills:        []string{"#02738b", "#05748c", "#08758d"},
		},
		{
			name:          "red mark stays compact",
			samplePath:    vectorizeSampleSourcePath("red-mark.png", "c2a368d3868d96b8"),
			wantColors:    3,
			maxPaths:      2,
			maxSubpaths:   12,
			maxRMSE:       0.04,
			maxColorDelta: 8,
			maxBgDelta:    0,
			maxFgDelta:    0.04,
			anyFills:      []string{"#CA1719", "#CA1819"},
		},
		{
			name:          "GJ logo keeps brand colors",
			samplePath:    vectorizeSampleSourcePath("gj-logo.png", "5bf827797c9f90ea"),
			wantColors:    12,
			maxPaths:      6,
			maxSubpaths:   48,
			maxRMSE:       0.04,
			maxColorDelta: 12,
			maxBgDelta:    0,
			maxFgDelta:    0.04,
			requiredFills: []string{"#241668", "#dc1a17", "#e96d10"},
		},
		{
			name:          "dark background thin stroke logo keeps contrast",
			samplePath:    vectorizeGeneratedDarkThinStrokeLogo(t, generatedDir),
			wantColors:    32,
			maxPaths:      12,
			maxSubpaths:   120,
			maxRMSE:       0.045,
			maxColorDelta: 10,
			maxBgDelta:    0,
			maxFgDelta:    0.05,
			requiredFills: []string{"#FEE715"},
		},
		{
			name:          "monochrome thin line icon keeps strokes",
			samplePath:    vectorizeGeneratedMonochromeThinLineIcon(t, generatedDir),
			wantColors:    3,
			maxPaths:      8,
			maxSubpaths:   80,
			maxRMSE:       0.04,
			maxColorDelta: 8,
			maxBgDelta:    0,
			maxFgDelta:    0.04,
			requiredFills: []string{"#131313"},
		},
		{
			name:          "multi color block text logo keeps separate colors",
			samplePath:    vectorizeGeneratedMultiColorBlockLogo(t, generatedDir),
			wantColors:    12,
			maxPaths:      10,
			maxSubpaths:   96,
			maxRMSE:       0.04,
			maxColorDelta: 12,
			maxBgDelta:    0,
			maxFgDelta:    0.05,
			requiredFills: []string{"#2458E6", "#E63223", "#18A058"},
		},
		{
			name:          "transparent background logo uses clean white canvas",
			samplePath:    vectorizeGeneratedTransparentLogo(t, generatedDir),
			wantColors:    8,
			maxPaths:      8,
			maxSubpaths:   80,
			maxRMSE:       0.04,
			maxColorDelta: 12,
			maxBgDelta:    0,
			maxFgDelta:    0.04,
			requiredFills: []string{"#2458E6", "#18A058"},
		},
		{
			name:          "speckled logo ignores isolated noise",
			samplePath:    vectorizeGeneratedSpeckledLogo(t, generatedDir),
			wantColors:    3,
			maxPaths:      8,
			maxSubpaths:   80,
			maxRMSE:       0.045,
			maxColorDelta: 8,
			maxBgDelta:    0,
			maxFgDelta:    0.05,
			requiredFills: []string{"#141414"},
		},
		{
			name:          "micro text strokes stay visible",
			samplePath:    vectorizeGeneratedMicroTextLogo(t, generatedDir),
			wantColors:    8,
			maxPaths:      8,
			maxSubpaths:   320,
			maxRMSE:       0.07,
			maxColorDelta: 8,
			maxBgDelta:    0,
			maxFgDelta:    0.025,
			requiredFills: []string{"#141414"},
		},
		{
			name:          "real font small text keeps dark letters",
			samplePath:    vectorizeGeneratedRealFontTextLogo(t, generatedDir),
			wantColors:    12,
			maxPaths:      10,
			maxSubpaths:   220,
			maxRMSE:       0.055,
			maxColorDelta: 8,
			maxBgDelta:    0,
			maxFgDelta:    0.035,
			requiredFills: []string{"#141414", "#08758D"},
		},
		{
			name:          "long real font tagline keeps dark letters",
			samplePath:    vectorizeGeneratedLongTaglineTextLogo(t, generatedDir),
			wantColors:    12,
			maxPaths:      10,
			maxSubpaths:   260,
			maxRMSE:       0.06,
			maxColorDelta: 8,
			maxBgDelta:    0,
			maxFgDelta:    0.04,
			requiredFills: []string{"#121212", "#08758D"},
		},
		{
			name:          "serif logistic logo keeps ribbon and service text",
			samplePath:    vectorizeGeneratedSerifLogisticLogo(t, generatedDir),
			wantColors:    16,
			maxPaths:      12,
			maxSubpaths:   280,
			maxRMSE:       0.055,
			maxColorDelta: 16,
			maxBgDelta:    0,
			maxFgDelta:    0.04,
			anyFills:      []string{"#00768E", "#08758D", "#0B778F"},
		},
		{
			name:          "low contrast real font tagline keeps gray letters",
			samplePath:    vectorizeGeneratedLowContrastTaglineLogo(t, generatedDir),
			wantColors:    12,
			maxPaths:      10,
			maxSubpaths:   260,
			maxRMSE:       0.055,
			maxColorDelta: 10,
			maxBgDelta:    0,
			maxFgDelta:    0.04,
			requiredFills: []string{"#787878", "#08758D"},
		},
		{
			name:          "cjk real font text stays readable",
			samplePath:    vectorizeGeneratedCJKRealFontTextLogo(t, generatedDir),
			wantColors:    12,
			maxPaths:      10,
			maxSubpaths:   240,
			maxRMSE:       0.04,
			maxColorDelta: 10,
			maxBgDelta:    0,
			maxFgDelta:    0.035,
			requiredFills: []string{"#141414", "#08758D"},
		},
		{
			name:          "flat cjk slogan banner keeps illustration and text",
			samplePath:    vectorizeGeneratedFlatCJKSloganBanner(t, generatedDir),
			wantColors:    8,
			maxPaths:      18,
			maxSubpaths:   520,
			maxRMSE:       0.055,
			maxColorDelta: 16,
			maxBgDelta:    0,
			maxFgDelta:    0.04,
			requiredFills: []string{"#114875", "#4E7498"},
			anyFills:      []string{"#EEF4F8", "#A8CBE2", "#E1D5B5"},
		},
		{
			name:          "dark real font white text stays readable",
			samplePath:    vectorizeGeneratedDarkRealFontTextLogo(t, generatedDir),
			wantColors:    32,
			maxPaths:      14,
			maxSubpaths:   260,
			maxRMSE:       0.055,
			maxColorDelta: 18,
			maxBgDelta:    0,
			maxFgDelta:    0.04,
			requiredFills: []string{"#0F172A", "#38BDF8"},
			anyFills:      []string{"#F4F4F4", "#F9F9F9"},
		},
		{
			name:          "ring holes and diagonal edges stay clean",
			samplePath:    vectorizeGeneratedRingDiagonalLogo(t, generatedDir),
			wantColors:    8,
			maxPaths:      12,
			maxSubpaths:   80,
			maxRMSE:       0.04,
			maxColorDelta: 10,
			maxBgDelta:    0,
			maxFgDelta:    0.04,
			requiredFills: []string{"#08758D", "#E63223"},
		},
		{
			name:          "soft neutral shadow stays controlled",
			samplePath:    vectorizeGeneratedSoftShadowLogo(t, generatedDir),
			wantColors:    8,
			maxPaths:      12,
			maxSubpaths:   120,
			maxRMSE:       0.045,
			maxColorDelta: 12,
			maxBgDelta:    0,
			maxFgDelta:    0.04,
			requiredFills: []string{"#2458E6", "#CACACA"},
		},
		{
			name:          "jpeg compression artifacts stay compact",
			samplePath:    vectorizeGeneratedJPEGArtifactLogo(t, generatedDir),
			mime:          "image/jpeg",
			wantColors:    8,
			maxPaths:      12,
			maxSubpaths:   160,
			maxRMSE:       0.055,
			maxColorDelta: 16,
			maxBgDelta:    0,
			maxFgDelta:    0.06,
			requiredFills: []string{"#2659E5", "#E53224"},
		},
		{
			name:          "thin colored outline logo keeps strokes",
			samplePath:    vectorizeGeneratedThinOutlineLogo(t, generatedDir),
			wantColors:    3,
			maxPaths:      8,
			maxSubpaths:   140,
			maxRMSE:       0.05,
			maxColorDelta: 10,
			maxBgDelta:    0,
			maxFgDelta:    0.035,
			requiredFills: []string{"#0E7990"},
		},
		{
			name:          "tinted background logo preserves canvas",
			samplePath:    vectorizeGeneratedTintedBackgroundLogo(t, generatedDir),
			wantColors:    8,
			maxPaths:      10,
			maxSubpaths:   120,
			maxRMSE:       0.04,
			maxColorDelta: 12,
			maxBgDelta:    0,
			maxFgDelta:    0.04,
			requiredFills: []string{"#0F766E", "#F97316", "#212B39"},
		},
		{
			name:          "knockout micro holes stay open",
			samplePath:    vectorizeGeneratedKnockoutMicroHolesLogo(t, generatedDir),
			wantColors:    8,
			maxPaths:      10,
			maxSubpaths:   180,
			maxRMSE:       0.045,
			maxColorDelta: 12,
			maxBgDelta:    0,
			maxFgDelta:    0.035,
			requiredFills: []string{"#2558E6", "#F97316"},
		},
		{
			name:          "white text on brand block keeps knockout letters",
			samplePath:    vectorizeGeneratedWhiteTextOnBrandBlockLogo(t, generatedDir),
			wantColors:    3,
			maxPaths:      6,
			maxSubpaths:   140,
			maxRMSE:       0.045,
			maxColorDelta: 10,
			maxBgDelta:    0,
			maxFgDelta:    0.04,
			requiredFills: []string{"#0B778F"},
		},
		{
			name:          "semi transparent shadow logo composites cleanly",
			samplePath:    vectorizeGeneratedSemiTransparentShadowLogo(t, generatedDir),
			wantColors:    8,
			maxPaths:      12,
			maxSubpaths:   120,
			maxRMSE:       0.045,
			maxColorDelta: 12,
			maxBgDelta:    0,
			maxFgDelta:    0.04,
			requiredFills: []string{"#2458E6", "#F97316", "#DFDFDF"},
		},
		{
			name:          "narrow negative gaps stay open",
			samplePath:    vectorizeGeneratedNarrowGapLogo(t, generatedDir),
			wantColors:    12,
			maxPaths:      12,
			maxSubpaths:   120,
			maxRMSE:       0.04,
			maxColorDelta: 12,
			maxBgDelta:    0,
			maxFgDelta:    0.035,
			requiredFills: []string{"#2458E6", "#E63223", "#18A058"},
		},
		{
			name:          "small legitimate marks stay visible",
			samplePath:    vectorizeGeneratedSmallMarksLogo(t, generatedDir),
			wantColors:    3,
			maxPaths:      10,
			maxSubpaths:   140,
			maxRMSE:       0.045,
			maxColorDelta: 10,
			maxBgDelta:    0,
			maxFgDelta:    0.035,
			requiredFills: []string{"#141414"},
		},
		{
			name:          "edge aligned logo keeps border strokes",
			samplePath:    vectorizeGeneratedEdgeAlignedLogo(t, generatedDir),
			wantColors:    8,
			maxPaths:      10,
			maxSubpaths:   120,
			maxRMSE:       0.04,
			maxColorDelta: 12,
			maxBgDelta:    0,
			maxFgDelta:    0.035,
			requiredFills: []string{"#2458E6", "#F97316"},
		},
		{
			name:          "same hue gradient logo keeps tonal steps",
			samplePath:    vectorizeGeneratedSameHueGradientLogo(t, generatedDir),
			wantColors:    16,
			maxPaths:      10,
			maxSubpaths:   140,
			maxRMSE:       0.04,
			maxColorDelta: 16,
			maxBgDelta:    0,
			maxFgDelta:    0.035,
			requiredFills: []string{"#006D87", "#40A9BA"},
		},
		{
			name:          "white knockout text on off white background stays white",
			samplePath:    vectorizeGeneratedWhiteTextOffWhiteLogo(t, generatedDir),
			wantColors:    8,
			maxPaths:      8,
			maxSubpaths:   100,
			maxRMSE:       0.04,
			maxColorDelta: 18,
			maxBgDelta:    0,
			maxFgDelta:    0.035,
			requiredFills: []string{"#123A5A"},
			anyFills:      []string{"#FFFFFF", "#F5F5F7"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := os.ReadFile(tt.samplePath)
			if err != nil {
				t.Skipf("sample image is not available: %v", err)
			}
			mime := tt.mime
			if mime == "" {
				mime = "image/png"
			}
			result, err := VectorizeImage(VectorizeInput{
				DataURL: "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data),
				Mode:    "logo",
			})
			if err != nil {
				t.Fatalf("VectorizeImage() error = %v", err)
			}
			if result.Engine != "clean-logo-potrace" {
				t.Fatalf("engine = %q, want clean-logo-potrace", result.Engine)
			}
			if result.Preset == nil {
				t.Fatalf("preset = nil, want cleanLogo preset")
			}
			if result.Preset.Name != "cleanLogo" || result.Preset.LongEdge != 1024 {
				t.Fatalf("preset = %#v, want cleanLogo preset with longEdge 1024", result.Preset)
			}
			if got := estimateCleanLogoVectorizeColors(context.Background(), vectorizeMagickPath(t), tt.samplePath); got != tt.wantColors {
				t.Fatalf("estimated colors = %d, want %d", got, tt.wantColors)
			}
			pathCount := len(regexp.MustCompile(`<path\b`).FindAllStringIndex(result.Content, -1))
			if pathCount > tt.maxPaths {
				t.Fatalf("path count = %d, want <= %d", pathCount, tt.maxPaths)
			}
			subpathCount := len(regexp.MustCompile(`\bM`).FindAllStringIndex(result.Content, -1))
			if subpathCount > tt.maxSubpaths {
				t.Fatalf("subpath count = %d, want <= %d", subpathCount, tt.maxSubpaths)
			}
			for _, fill := range tt.requiredFills {
				if !vectorizeContainsFill(result.Content, fill) {
					t.Fatalf("svg does not contain required fill %s; fills=%v", fill, vectorizeSVGFillValues(result.Content))
				}
			}
			if len(tt.anyFills) > 0 && !vectorizeContainsAnyFill(result.Content, tt.anyFills) {
				t.Fatalf("svg does not contain any acceptable fill from %v; fills=%v", tt.anyFills, vectorizeSVGFillValues(result.Content))
			}
			vectorizeAssertBackgroundDelta(t, tt.samplePath, result.Content, tt.maxBgDelta)
			vectorizeAssertBrandColorDelta(t, result.Content, append(tt.requiredFills, tt.anyFills...), tt.maxColorDelta)
			normalizedSourcePath, renderedPath := vectorizeRenderComparisonImages(t, tt.samplePath, result)
			if tt.name == "dark real font white text stays readable" {
				vectorizeAssertNoTintBleedInScaledRegion(t, renderedPath, image.Rect(400, 130, 700, 185), 720, 420, 0.01)
			}
			maxFgColorDelta := tt.maxFgColorDelta
			if maxFgColorDelta <= 0 {
				maxFgColorDelta = 24
			}
			vectorizeAssertForegroundColorDelta(t, normalizedSourcePath, renderedPath, tt.samplePath, maxFgColorDelta)
			vectorizeAssertForegroundCoverageDelta(t, normalizedSourcePath, renderedPath, tt.samplePath, tt.maxFgDelta)
			rmse := vectorizeRenderedRMSE(t, normalizedSourcePath, renderedPath)
			t.Logf("rendered RMSE = %.6f", rmse)
			if rmse > tt.maxRMSE {
				t.Fatalf("rendered RMSE = %.6f, want <= %.6f", rmse, tt.maxRMSE)
			}
		})
	}
}

func vectorizeRenderedRMSE(t *testing.T, normalizedSourcePath string, renderedPath string) float64 {
	t.Helper()
	magickPath := vectorizeMagickPath(t)
	output, err := exec.Command(magickPath, "compare", "-metric", "RMSE", normalizedSourcePath, renderedPath, "null:").CombinedOutput()
	if err != nil && len(output) == 0 {
		t.Fatalf("compare rendered svg: %v", err)
	}
	return vectorizeParseRMSE(t, string(output))
}

func vectorizeGeneratedDarkThinStrokeLogo(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "dark-thin-stroke-logo.png")
	img := image.NewRGBA(image.Rect(0, 0, 960, 420))
	vectorizeFillRect(img, image.Rect(0, 0, 960, 420), color.RGBA{R: 0x10, G: 0x18, B: 0x20, A: 0xff})
	yellow := color.RGBA{R: 0xfe, G: 0xe7, B: 0x15, A: 0xff}
	white := color.RGBA{R: 0xee, G: 0xef, B: 0xef, A: 0xff}

	vectorizeFillRect(img, image.Rect(120, 96, 172, 294), yellow)
	vectorizeFillRect(img, image.Rect(260, 96, 312, 294), yellow)
	vectorizeFillRect(img, image.Rect(120, 174, 312, 218), yellow)
	vectorizeFillRect(img, image.Rect(370, 96, 594, 144), yellow)
	vectorizeFillRect(img, image.Rect(456, 96, 508, 294), yellow)
	vectorizeFillRect(img, image.Rect(660, 96, 712, 294), yellow)
	vectorizeFillRect(img, image.Rect(660, 246, 836, 294), yellow)
	vectorizeFillRect(img, image.Rect(102, 326, 858, 342), white)
	for i := 0; i < 8; i++ {
		x := 130 + i*88
		vectorizeFillRect(img, image.Rect(x, 354, x+52, 366), white)
		vectorizeFillRect(img, image.Rect(x, 376, x+72, 388), white)
	}
	vectorizeWritePNG(t, path, img)
	return path
}

func vectorizeGeneratedMultiColorBlockLogo(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "multi-color-block-logo.png")
	img := image.NewRGBA(image.Rect(0, 0, 960, 420))
	white := color.RGBA{R: 0xfd, G: 0xfd, B: 0xfd, A: 0xff}
	blue := color.RGBA{R: 0x24, G: 0x58, B: 0xe6, A: 0xff}
	red := color.RGBA{R: 0xe6, G: 0x32, B: 0x23, A: 0xff}
	green := color.RGBA{R: 0x18, G: 0xa0, B: 0x58, A: 0xff}
	vectorizeFillRect(img, image.Rect(0, 0, 960, 420), white)

	vectorizeFillRect(img, image.Rect(105, 100, 255, 250), blue)
	vectorizeFillRect(img, image.Rect(150, 145, 210, 205), white)
	vectorizeFillRect(img, image.Rect(305, 100, 455, 250), red)
	vectorizeFillRect(img, image.Rect(305, 100, 455, 142), green)
	vectorizeFillRect(img, image.Rect(505, 100, 655, 250), green)
	vectorizeFillRect(img, image.Rect(548, 142, 612, 208), white)
	vectorizeFillRect(img, image.Rect(710, 100, 858, 142), blue)
	vectorizeFillRect(img, image.Rect(762, 100, 806, 250), blue)
	for i := 0; i < 7; i++ {
		x := 120 + i*100
		vectorizeFillRect(img, image.Rect(x, 310, x+58, 320), blue)
		vectorizeFillRect(img, image.Rect(x, 336, x+72, 346), red)
		vectorizeFillRect(img, image.Rect(x, 362, x+46, 372), green)
	}
	vectorizeWritePNG(t, path, img)
	return path
}

func vectorizeGeneratedMonochromeThinLineIcon(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "monochrome-thin-line-icon.png")
	img := image.NewRGBA(image.Rect(0, 0, 720, 420))
	white := color.RGBA{R: 0xfd, G: 0xfd, B: 0xfd, A: 0xff}
	black := color.RGBA{R: 0x11, G: 0x11, B: 0x11, A: 0xff}
	vectorizeFillRect(img, image.Rect(0, 0, 720, 420), white)

	vectorizeFillRect(img, image.Rect(150, 90, 570, 110), black)
	vectorizeFillRect(img, image.Rect(150, 310, 570, 330), black)
	vectorizeFillRect(img, image.Rect(150, 90, 170, 330), black)
	vectorizeFillRect(img, image.Rect(550, 90, 570, 330), black)
	vectorizeFillRect(img, image.Rect(245, 160, 475, 180), black)
	vectorizeFillRect(img, image.Rect(245, 240, 475, 260), black)
	vectorizeFillRect(img, image.Rect(245, 160, 265, 260), black)
	vectorizeFillRect(img, image.Rect(455, 160, 475, 260), black)
	vectorizeFillRect(img, image.Rect(335, 120, 385, 300), black)
	vectorizeFillRect(img, image.Rect(300, 340, 420, 356), black)
	vectorizeWritePNG(t, path, img)
	return path
}

func vectorizeGeneratedTransparentLogo(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "transparent-logo.png")
	img := image.NewRGBA(image.Rect(0, 0, 720, 420))
	blue := color.RGBA{R: 0x24, G: 0x58, B: 0xe6, A: 0xff}
	green := color.RGBA{R: 0x18, G: 0xa0, B: 0x58, A: 0xff}

	vectorizeFillRect(img, image.Rect(120, 120, 280, 280), blue)
	vectorizeFillRect(img, image.Rect(200, 200, 360, 320), green)
	vectorizeFillRect(img, image.Rect(430, 120, 470, 320), blue)
	vectorizeFillRect(img, image.Rect(430, 280, 600, 320), blue)
	vectorizeFillRect(img, image.Rect(500, 120, 600, 160), green)
	vectorizeWritePNG(t, path, img)
	return path
}

func vectorizeGeneratedSpeckledLogo(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "speckled-logo.png")
	img := image.NewRGBA(image.Rect(0, 0, 720, 420))
	white := color.RGBA{R: 0xfd, G: 0xfd, B: 0xfd, A: 0xff}
	black := color.RGBA{R: 0x13, G: 0x13, B: 0x13, A: 0xff}
	gray := color.RGBA{R: 0x6a, G: 0x6a, B: 0x6a, A: 0xff}
	vectorizeFillRect(img, image.Rect(0, 0, 720, 420), white)

	vectorizeFillRect(img, image.Rect(120, 120, 180, 300), black)
	vectorizeFillRect(img, image.Rect(120, 120, 335, 170), black)
	vectorizeFillRect(img, image.Rect(120, 250, 335, 300), black)
	vectorizeFillRect(img, image.Rect(395, 120, 455, 300), black)
	vectorizeFillRect(img, image.Rect(395, 120, 590, 170), black)
	vectorizeFillRect(img, image.Rect(395, 250, 590, 300), black)
	for i := 0; i < 6; i++ {
		x := 130 + i*78
		vectorizeFillRect(img, image.Rect(x, 335, x+46, 345), black)
		vectorizeFillRect(img, image.Rect(x, 360, x+62, 370), black)
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
		vectorizeFillRect(img, rect, fill)
	}
	vectorizeWritePNG(t, path, img)
	return path
}

func vectorizeGeneratedMicroTextLogo(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "micro-text-logo.png")
	largePath := filepath.Join(dir, "micro-text-logo-large.png")
	img := image.NewRGBA(image.Rect(0, 0, 1440, 840))
	white := color.RGBA{R: 0xfd, G: 0xfd, B: 0xfd, A: 0xff}
	black := color.RGBA{R: 0x13, G: 0x13, B: 0x13, A: 0xff}
	blue := color.RGBA{R: 0x24, G: 0x58, B: 0xe6, A: 0xff}
	vectorizeFillRect(img, image.Rect(0, 0, 1440, 840), white)

	vectorizeFillRect(img, image.Rect(190, 170, 310, 570), blue)
	vectorizeFillRect(img, image.Rect(190, 170, 690, 270), blue)
	vectorizeFillRect(img, image.Rect(190, 470, 690, 570), blue)
	vectorizeFillRect(img, image.Rect(800, 170, 920, 570), black)
	vectorizeFillRect(img, image.Rect(800, 470, 1190, 570), black)

	for row := 0; row < 3; row++ {
		y := 650 + row*46
		for col := 0; col < 28; col++ {
			x := 170 + col*38
			vectorizeFillRect(img, image.Rect(x, y, x+8, y+22), black)
			vectorizeFillRect(img, image.Rect(x+14, y, x+22, y+22), black)
			vectorizeFillRect(img, image.Rect(x, y+28, x+28, y+36), black)
		}
	}

	vectorizeWritePNG(t, largePath, img)
	magickPath := vectorizeMagickPath(t)
	if output, err := exec.Command(magickPath, largePath, "-resize", "720x420!", "-colorspace", "sRGB", "-strip", path).CombinedOutput(); err != nil {
		t.Fatalf("resize generated micro-text sample: %v: %s", err, string(output))
	}
	return path
}

func vectorizeGeneratedRealFontTextLogo(t *testing.T, dir string) string {
	t.Helper()
	fontPath := vectorizeTestFontPath(t)
	path := filepath.Join(dir, "real-font-text-logo.png")
	magickPath := vectorizeMagickPath(t)
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

func vectorizeGeneratedLongTaglineTextLogo(t *testing.T, dir string) string {
	t.Helper()
	fontPath := vectorizeTestFontPath(t)
	path := filepath.Join(dir, "long-tagline-text-logo.png")
	magickPath := vectorizeMagickPath(t)
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

func vectorizeGeneratedSerifLogisticLogo(t *testing.T, dir string) string {
	t.Helper()
	fontPath := vectorizeTestSerifFontPath(t)
	path := filepath.Join(dir, "serif-logistic-logo.png")
	magickPath := vectorizeMagickPath(t)
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

func vectorizeGeneratedLowContrastTaglineLogo(t *testing.T, dir string) string {
	t.Helper()
	fontPath := vectorizeTestFontPath(t)
	path := filepath.Join(dir, "low-contrast-tagline-logo.png")
	magickPath := vectorizeMagickPath(t)
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

func vectorizeGeneratedCJKRealFontTextLogo(t *testing.T, dir string) string {
	t.Helper()
	fontPath := vectorizeTestCJKFontPath(t)
	path := filepath.Join(dir, "cjk-real-font-text-logo.png")
	magickPath := vectorizeMagickPath(t)
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

func vectorizeGeneratedFlatCJKSloganBanner(t *testing.T, dir string) string {
	t.Helper()
	fontPath := vectorizeTestCJKFontPath(t)
	path := filepath.Join(dir, "flat-cjk-slogan-banner.png")
	magickPath := vectorizeMagickPath(t)
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

func vectorizeGeneratedDarkRealFontTextLogo(t *testing.T, dir string) string {
	t.Helper()
	fontPath := vectorizeTestFontPath(t)
	path := filepath.Join(dir, "dark-real-font-text-logo.png")
	magickPath := vectorizeMagickPath(t)
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

func vectorizeTestFontPath(t *testing.T) string {
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

func vectorizeTestSerifFontPath(t *testing.T) string {
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

func vectorizeTestCJKFontPath(t *testing.T) string {
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

func vectorizeGeneratedRingDiagonalLogo(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "ring-diagonal-logo.png")
	largePath := filepath.Join(dir, "ring-diagonal-logo-large.png")
	magickPath := vectorizeMagickPath(t)
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

func vectorizeGeneratedSoftShadowLogo(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "soft-shadow-logo.png")
	largePath := filepath.Join(dir, "soft-shadow-logo-large.png")
	magickPath := vectorizeMagickPath(t)
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

func vectorizeGeneratedJPEGArtifactLogo(t *testing.T, dir string) string {
	t.Helper()
	pngPath := filepath.Join(dir, "jpeg-artifact-logo-source.png")
	jpegPath := filepath.Join(dir, "jpeg-artifact-logo.jpg")
	img := image.NewRGBA(image.Rect(0, 0, 720, 420))
	white := color.RGBA{R: 0xfd, G: 0xfd, B: 0xfd, A: 0xff}
	blue := color.RGBA{R: 0x24, G: 0x58, B: 0xe6, A: 0xff}
	red := color.RGBA{R: 0xe6, G: 0x32, B: 0x23, A: 0xff}
	vectorizeFillRect(img, image.Rect(0, 0, 720, 420), white)
	vectorizeFillRect(img, image.Rect(100, 90, 310, 285), blue)
	vectorizeFillRect(img, image.Rect(165, 150, 245, 225), white)
	vectorizeFillRect(img, image.Rect(385, 90, 620, 140), red)
	vectorizeFillRect(img, image.Rect(385, 175, 620, 225), red)
	vectorizeFillRect(img, image.Rect(385, 260, 620, 310), red)
	for i := 0; i < 8; i++ {
		x := 105 + i*64
		vectorizeFillRect(img, image.Rect(x, 345, x+42, 355), blue)
		vectorizeFillRect(img, image.Rect(x, 370, x+54, 380), red)
	}
	vectorizeWritePNG(t, pngPath, img)
	magickPath := vectorizeMagickPath(t)
	if output, err := exec.Command(magickPath, pngPath, "-sampling-factor", "4:2:0", "-quality", "72", jpegPath).CombinedOutput(); err != nil {
		t.Fatalf("write generated jpeg-artifact sample: %v: %s", err, string(output))
	}
	return jpegPath
}

func vectorizeGeneratedThinOutlineLogo(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "thin-outline-logo.png")
	largePath := filepath.Join(dir, "thin-outline-logo-large.png")
	magickPath := vectorizeMagickPath(t)
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

func vectorizeGeneratedTintedBackgroundLogo(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "tinted-background-logo.png")
	img := image.NewRGBA(image.Rect(0, 0, 720, 420))
	bg := color.RGBA{R: 0xe7, G: 0xf8, B: 0xf2, A: 0xff}
	green := color.RGBA{R: 0x0f, G: 0x76, B: 0x6e, A: 0xff}
	orange := color.RGBA{R: 0xf9, G: 0x73, B: 0x16, A: 0xff}
	dark := color.RGBA{R: 0x1f, G: 0x29, B: 0x37, A: 0xff}
	vectorizeFillRect(img, image.Rect(0, 0, 720, 420), bg)
	vectorizeFillRect(img, image.Rect(120, 95, 300, 275), green)
	vectorizeFillRect(img, image.Rect(175, 150, 245, 220), bg)
	vectorizeFillRect(img, image.Rect(365, 95, 600, 145), orange)
	vectorizeFillRect(img, image.Rect(365, 185, 600, 235), orange)
	vectorizeFillRect(img, image.Rect(365, 275, 520, 325), orange)
	for i := 0; i < 6; i++ {
		x := 125 + i*78
		vectorizeFillRect(img, image.Rect(x, 352, x+48, 362), dark)
		vectorizeFillRect(img, image.Rect(x, 378, x+66, 388), dark)
	}
	vectorizeWritePNG(t, path, img)
	return path
}

func vectorizeGeneratedKnockoutMicroHolesLogo(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "knockout-micro-holes-logo.png")
	img := image.NewRGBA(image.Rect(0, 0, 720, 420))
	white := color.RGBA{R: 0xfd, G: 0xfd, B: 0xfd, A: 0xff}
	blue := color.RGBA{R: 0x24, G: 0x58, B: 0xe6, A: 0xff}
	orange := color.RGBA{R: 0xf9, G: 0x73, B: 0x16, A: 0xff}
	vectorizeFillRect(img, image.Rect(0, 0, 720, 420), white)
	vectorizeFillRect(img, image.Rect(105, 95, 390, 285), blue)
	vectorizeFillRect(img, image.Rect(450, 95, 610, 285), orange)

	for row := 0; row < 3; row++ {
		y := 130 + row*48
		for col := 0; col < 6; col++ {
			x := 140 + col*38
			vectorizeFillRect(img, image.Rect(x, y, x+10, y+24), white)
			vectorizeFillRect(img, image.Rect(x+17, y, x+27, y+24), white)
			vectorizeFillRect(img, image.Rect(x, y+32, x+30, y+42), white)
		}
	}
	vectorizeFillRect(img, image.Rect(500, 135, 560, 195), white)
	vectorizeFillRect(img, image.Rect(520, 155, 540, 175), orange)
	vectorizeFillRect(img, image.Rect(490, 225, 570, 245), white)
	vectorizeFillRect(img, image.Rect(520, 205, 540, 265), white)
	for i := 0; i < 7; i++ {
		x := 120 + i*70
		vectorizeFillRect(img, image.Rect(x, 340, x+44, 350), blue)
		vectorizeFillRect(img, image.Rect(x, 365, x+56, 375), orange)
	}
	vectorizeWritePNG(t, path, img)
	return path
}

func vectorizeGeneratedWhiteTextOnBrandBlockLogo(t *testing.T, dir string) string {
	t.Helper()
	fontPath := vectorizeTestFontPath(t)
	path := filepath.Join(dir, "white-text-brand-block-logo.png")
	magickPath := vectorizeMagickPath(t)
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

func vectorizeGeneratedSemiTransparentShadowLogo(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "semi-transparent-shadow-logo.png")
	img := image.NewRGBA(image.Rect(0, 0, 720, 420))
	shadow := color.RGBA{R: 0x80, G: 0x80, B: 0x80, A: 0x50}
	blue := color.RGBA{R: 0x24, G: 0x58, B: 0xe6, A: 0xff}
	orange := color.RGBA{R: 0xf9, G: 0x73, B: 0x16, A: 0xff}
	white := color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	vectorizeFillRect(img, image.Rect(140, 130, 370, 290), shadow)
	vectorizeFillRect(img, image.Rect(400, 142, 620, 302), shadow)
	vectorizeFillRect(img, image.Rect(110, 100, 340, 260), blue)
	vectorizeFillRect(img, image.Rect(175, 145, 275, 215), white)
	vectorizeFillRect(img, image.Rect(370, 112, 590, 272), orange)
	vectorizeFillRect(img, image.Rect(430, 160, 530, 224), white)
	for i := 0; i < 6; i++ {
		x := 120 + i*80
		vectorizeFillRect(img, image.Rect(x+8, 342, x+62, 354), shadow)
		vectorizeFillRect(img, image.Rect(x, 334, x+54, 346), blue)
		vectorizeFillRect(img, image.Rect(x, 370, x+66, 382), orange)
	}
	vectorizeWritePNG(t, path, img)
	return path
}

func vectorizeGeneratedNarrowGapLogo(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "narrow-gap-logo.png")
	img := image.NewRGBA(image.Rect(0, 0, 720, 420))
	white := color.RGBA{R: 0xfd, G: 0xfd, B: 0xfd, A: 0xff}
	blue := color.RGBA{R: 0x24, G: 0x58, B: 0xe6, A: 0xff}
	red := color.RGBA{R: 0xe6, G: 0x32, B: 0x23, A: 0xff}
	green := color.RGBA{R: 0x18, G: 0xa0, B: 0x58, A: 0xff}
	vectorizeFillRect(img, image.Rect(0, 0, 720, 420), white)
	vectorizeFillRect(img, image.Rect(110, 95, 305, 290), blue)
	vectorizeFillRect(img, image.Rect(321, 95, 516, 290), red)
	vectorizeFillRect(img, image.Rect(532, 95, 625, 290), green)
	vectorizeFillRect(img, image.Rect(184, 168, 232, 216), white)
	vectorizeFillRect(img, image.Rect(392, 168, 444, 216), white)
	vectorizeFillRect(img, image.Rect(560, 168, 596, 216), white)
	vectorizeFillRect(img, image.Rect(305, 95, 321, 290), white)
	vectorizeFillRect(img, image.Rect(516, 95, 532, 290), white)
	for i := 0; i < 8; i++ {
		x := 105 + i*66
		vectorizeFillRect(img, image.Rect(x, 340, x+46, 350), blue)
		vectorizeFillRect(img, image.Rect(x, 365, x+56, 375), red)
	}
	vectorizeWritePNG(t, path, img)
	return path
}

func vectorizeGeneratedSmallMarksLogo(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "small-marks-logo.png")
	largePath := filepath.Join(dir, "small-marks-logo-large.png")
	magickPath := vectorizeMagickPath(t)
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

func vectorizeGeneratedEdgeAlignedLogo(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "edge-aligned-logo.png")
	img := image.NewRGBA(image.Rect(0, 0, 720, 420))
	white := color.RGBA{R: 0xfd, G: 0xfd, B: 0xfd, A: 0xff}
	blue := color.RGBA{R: 0x24, G: 0x58, B: 0xe6, A: 0xff}
	orange := color.RGBA{R: 0xf9, G: 0x73, B: 0x16, A: 0xff}
	vectorizeFillRect(img, image.Rect(0, 0, 720, 420), white)
	vectorizeFillRect(img, image.Rect(0, 90, 86, 300), blue)
	vectorizeFillRect(img, image.Rect(140, 90, 360, 150), blue)
	vectorizeFillRect(img, image.Rect(140, 185, 360, 245), blue)
	vectorizeFillRect(img, image.Rect(420, 90, 650, 300), orange)
	vectorizeFillRect(img, image.Rect(495, 155, 575, 235), white)
	vectorizeFillRect(img, image.Rect(70, 388, 620, 420), blue)
	vectorizeFillRect(img, image.Rect(110, 342, 510, 360), orange)
	vectorizeWritePNG(t, path, img)
	return path
}

func vectorizeGeneratedSameHueGradientLogo(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "same-hue-gradient-logo.png")
	img := image.NewRGBA(image.Rect(0, 0, 720, 420))
	white := color.RGBA{R: 0xfd, G: 0xfd, B: 0xfd, A: 0xff}
	dark := color.RGBA{R: 0x00, G: 0x6d, B: 0x87, A: 0xff}
	mid := color.RGBA{R: 0x09, G: 0x82, B: 0x9d, A: 0xff}
	light := color.RGBA{R: 0x40, G: 0xa9, B: 0xba, A: 0xff}
	vectorizeFillRect(img, image.Rect(0, 0, 720, 420), white)
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
		vectorizeFillRect(img, image.Rect(110, y, 610, y+1), fill)
	}
	vectorizeFillRect(img, image.Rect(175, 145, 545, 195), white)
	vectorizeFillRect(img, image.Rect(110, 275, 610, 305), dark)
	vectorizeFillRect(img, image.Rect(145, 330, 325, 352), mid)
	vectorizeFillRect(img, image.Rect(370, 330, 575, 352), light)
	vectorizeWritePNG(t, path, img)
	return path
}

func vectorizeGeneratedWhiteTextOffWhiteLogo(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "white-text-off-white-logo.png")
	img := image.NewRGBA(image.Rect(0, 0, 720, 420))
	background := color.RGBA{R: 0xf3, G: 0xf4, B: 0xf6, A: 0xff}
	blue := color.RGBA{R: 0x12, G: 0x3a, B: 0x5a, A: 0xff}
	white := color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	vectorizeFillRect(img, image.Rect(0, 0, 720, 420), background)
	vectorizeFillRect(img, image.Rect(105, 95, 615, 270), blue)
	vectorizeFillRect(img, image.Rect(170, 145, 270, 190), white)
	vectorizeFillRect(img, image.Rect(305, 145, 405, 190), white)
	vectorizeFillRect(img, image.Rect(440, 145, 555, 190), white)
	vectorizeFillRect(img, image.Rect(115, 300, 605, 326), blue)
	vectorizeFillRect(img, image.Rect(165, 350, 260, 370), white)
	vectorizeFillRect(img, image.Rect(315, 350, 405, 370), white)
	vectorizeFillRect(img, image.Rect(465, 350, 560, 370), white)
	vectorizeWritePNG(t, path, img)
	return path
}

func vectorizeGeneratedIllustrationMaterial(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "illustration-material.png")
	img := image.NewRGBA(image.Rect(0, 0, 360, 240))
	vectorizeFillRect(img, image.Rect(0, 0, 360, 240), color.RGBA{R: 255, G: 255, B: 255, A: 255})
	vectorizeFillRect(img, image.Rect(24, 24, 156, 216), color.RGBA{R: 247, G: 182, B: 31, A: 255})
	vectorizeFillRect(img, image.Rect(180, 24, 336, 112), color.RGBA{R: 246, G: 192, B: 78, A: 255})
	vectorizeFillRect(img, image.Rect(180, 136, 336, 216), color.RGBA{R: 255, G: 213, B: 106, A: 255})
	vectorizeWritePNG(t, path, img)
	return path
}

func vectorizeFillRect(img *image.RGBA, rect image.Rectangle, fill color.Color) {
	draw.Draw(img, rect, &image.Uniform{C: fill}, image.Point{}, draw.Src)
}

func vectorizeWritePNG(t *testing.T, path string, img image.Image) {
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

func vectorizeRenderComparisonImages(t *testing.T, sourcePath string, result VectorizeResult) (string, string) {
	t.Helper()
	magickPath := vectorizeMagickPath(t)
	dir := t.TempDir()
	svgPath := filepath.Join(dir, "result.svg")
	normalizedSourcePath := filepath.Join(dir, "source-normalized.png")
	renderedPath := filepath.Join(dir, "result-rendered.png")
	if err := os.WriteFile(svgPath, []byte(result.Content), 0o600); err != nil {
		t.Fatalf("write svg: %v", err)
	}
	size := fmt.Sprintf("%dx%d!", result.Width, result.Height)
	if output, err := exec.Command(magickPath, sourcePath, "-background", "white", "-alpha", "remove", "-resize", size, "-colorspace", "sRGB", "-strip", normalizedSourcePath).CombinedOutput(); err != nil {
		t.Fatalf("normalize source: %v: %s", err, string(output))
	}
	if output, err := exec.Command(magickPath, "-background", "white", svgPath, "-resize", size, "-colorspace", "sRGB", "-strip", renderedPath).CombinedOutput(); err != nil {
		t.Fatalf("render svg: %v: %s", err, string(output))
	}
	return normalizedSourcePath, renderedPath
}

func vectorizeMagickPath(t *testing.T) string {
	t.Helper()
	magickPath, err := requireVectorizeCommand(config.Cfg.ImageMagickPath, "magick")
	if err != nil {
		t.Skipf("magick is not available: %v", err)
	}
	return magickPath
}

func vectorizeParseRMSE(t *testing.T, output string) float64 {
	t.Helper()
	match := regexp.MustCompile(`\(([0-9.]+(?:e[-+]?\d+)?)\)`).FindStringSubmatch(output)
	if match == nil {
		t.Fatalf("unable to parse RMSE from %q", output)
	}
	value, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		t.Fatalf("parse RMSE: %v", err)
	}
	return value
}

func vectorizeAssertForegroundCoverageDelta(t *testing.T, normalizedSourcePath string, renderedPath string, sourcePath string, maxDelta float64) {
	t.Helper()
	background := vectorizeSourceBorderBackground(t, sourcePath)
	sourceCoverage := vectorizeForegroundCoverage(t, normalizedSourcePath, background)
	renderedCoverage := vectorizeForegroundCoverage(t, renderedPath, background)
	delta := math.Abs(sourceCoverage - renderedCoverage)
	t.Logf("foreground coverage source=%.4f rendered=%.4f delta=%.4f", sourceCoverage, renderedCoverage, delta)
	if delta > maxDelta {
		t.Fatalf("foreground coverage delta = %.4f, want <= %.4f", delta, maxDelta)
	}
}

func vectorizeAssertNoTintBleedInScaledRegion(t *testing.T, imagePath string, baseRegion image.Rectangle, baseWidth int, baseHeight int, maxTintRatio float64) {
	t.Helper()
	img := vectorizeDecodeImage(t, imagePath)
	imageBounds := img.Bounds()
	region := image.Rect(
		imageBounds.Min.X+baseRegion.Min.X*imageBounds.Dx()/baseWidth,
		imageBounds.Min.Y+baseRegion.Min.Y*imageBounds.Dy()/baseHeight,
		imageBounds.Min.X+baseRegion.Max.X*imageBounds.Dx()/baseWidth,
		imageBounds.Min.Y+baseRegion.Max.Y*imageBounds.Dy()/baseHeight,
	)
	bounds := region.Intersect(img.Bounds())
	total := bounds.Dx() * bounds.Dy()
	if total == 0 {
		t.Fatalf("tint bleed region is empty")
	}
	tinted := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			rgb := vectorizeRGB8(img.At(x, y))
			hue, sat := cleanLogoHueAndSaturation(uint8(rgb[0]), uint8(rgb[1]), uint8(rgb[2]))
			luma := cleanLogoLuma(uint8(rgb[0]), uint8(rgb[1]), uint8(rgb[2]))
			if sat >= 0.35 && hue >= 170 && hue <= 230 && luma >= 90 && luma <= 230 {
				tinted++
			}
		}
	}
	ratio := float64(tinted) / float64(total)
	t.Logf("tint bleed ratio in text region = %.4f", ratio)
	if ratio > maxTintRatio {
		t.Fatalf("tint bleed ratio in text region = %.4f, want <= %.4f", ratio, maxTintRatio)
	}
}

func vectorizeAssertForegroundColorDelta(t *testing.T, normalizedSourcePath string, renderedPath string, sourcePath string, maxDelta float64) {
	t.Helper()
	background := vectorizeSourceBorderBackground(t, sourcePath)
	sourceImage := vectorizeDecodeImage(t, normalizedSourcePath)
	renderedImage := vectorizeDecodeImage(t, renderedPath)
	sourceColor, ok := vectorizeDominantForegroundColor(sourceImage, background)
	if !ok {
		t.Fatalf("source image has no foreground color")
	}
	renderedColor, delta, ok := vectorizeNearestForegroundColor(renderedImage, background, sourceColor)
	if !ok {
		t.Fatalf("rendered image has no foreground color")
	}
	t.Logf("foreground color source=%s rendered=%s delta=%.2f", vectorizeRGBHex(sourceColor), vectorizeRGBHex(renderedColor), delta)
	if delta > maxDelta {
		t.Fatalf("foreground color delta = %.2f, want <= %.2f (source=%s rendered=%s)", delta, maxDelta, vectorizeRGBHex(sourceColor), vectorizeRGBHex(renderedColor))
	}
}

func vectorizeDecodeImage(t *testing.T, imagePath string) image.Image {
	t.Helper()
	file, err := os.Open(imagePath)
	if err != nil {
		t.Fatalf("open image: %v", err)
	}
	defer file.Close()
	img, _, err := image.Decode(file)
	if err != nil {
		t.Fatalf("decode image: %v", err)
	}
	return img
}

func vectorizeForegroundCoverage(t *testing.T, imagePath string, background [3]int) float64 {
	t.Helper()
	img := vectorizeDecodeImage(t, imagePath)
	bounds := img.Bounds()
	total := bounds.Dx() * bounds.Dy()
	if total == 0 {
		t.Fatalf("image has empty bounds")
	}
	foreground := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if vectorizeRGBDistance(vectorizeRGB8(img.At(x, y)), background) > 24 {
				foreground++
			}
		}
	}
	return float64(foreground) / float64(total)
}

func vectorizeDominantForegroundColor(img image.Image, background [3]int) ([3]int, bool) {
	bounds := img.Bounds()
	counts := map[[3]int]int{}
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			rgb := vectorizeRGB8(img.At(x, y))
			if vectorizeRGBDistance(rgb, background) <= 24 {
				continue
			}
			counts[vectorizeQuantizedQualityColor(rgb)]++
		}
	}
	best := [3]int{}
	bestCount := 0
	for rgb, count := range counts {
		if count > bestCount {
			best = rgb
			bestCount = count
		}
	}
	return best, bestCount > 0
}

func vectorizeNearestForegroundColor(img image.Image, background [3]int, target [3]int) ([3]int, float64, bool) {
	bounds := img.Bounds()
	seen := map[[3]int]struct{}{}
	best := [3]int{}
	bestDistance := math.MaxFloat64
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			rgb := vectorizeRGB8(img.At(x, y))
			if vectorizeRGBDistance(rgb, background) <= 24 {
				continue
			}
			quantized := vectorizeQuantizedQualityColor(rgb)
			if _, ok := seen[quantized]; ok {
				continue
			}
			seen[quantized] = struct{}{}
			distance := vectorizeRGBDistance(target, quantized)
			if distance < bestDistance {
				best = quantized
				bestDistance = distance
			}
		}
	}
	return best, bestDistance, len(seen) > 0
}

func vectorizeQuantizedQualityColor(rgb [3]int) [3]int {
	return [3]int{
		vectorizeQuantizedQualityChannel(rgb[0]),
		vectorizeQuantizedQualityChannel(rgb[1]),
		vectorizeQuantizedQualityChannel(rgb[2]),
	}
}

func vectorizeQuantizedQualityChannel(value int) int {
	rounded := (value + 4) / 8 * 8
	if rounded > 255 {
		return 255
	}
	if rounded < 0 {
		return 0
	}
	return rounded
}

func vectorizeRGBHex(rgb [3]int) string {
	return fmt.Sprintf("#%02X%02X%02X", rgb[0], rgb[1], rgb[2])
}

func vectorizeAssertBackgroundDelta(t *testing.T, sourcePath string, svg string, maxDelta float64) {
	t.Helper()
	sourceBackground := vectorizeSourceBorderBackground(t, sourcePath)
	svgBackground, ok := vectorizeSVGBackgroundFill(svg)
	if !ok {
		t.Fatalf("svg does not contain a background rect fill")
	}
	delta := vectorizeRGBDistance(sourceBackground, svgBackground)
	t.Logf("background delta = %.2f", delta)
	if delta > maxDelta {
		t.Fatalf("background color delta = %.2f, want <= %.2f", delta, maxDelta)
	}
}

func vectorizeSourceBorderBackground(t *testing.T, sourcePath string) [3]int {
	t.Helper()
	file, err := os.Open(sourcePath)
	if err != nil {
		t.Fatalf("open source image: %v", err)
	}
	defer file.Close()
	img, _, err := image.Decode(file)
	if err != nil {
		t.Fatalf("decode source image: %v", err)
	}
	bounds := img.Bounds()
	counts := map[[3]int]int{}
	add := func(x int, y int) {
		counts[vectorizeRGB8(img.At(x, y))]++
	}
	for x := bounds.Min.X; x < bounds.Max.X; x++ {
		add(x, bounds.Min.Y)
		add(x, bounds.Max.Y-1)
	}
	for y := bounds.Min.Y + 1; y < bounds.Max.Y-1; y++ {
		add(bounds.Min.X, y)
		add(bounds.Max.X-1, y)
	}
	best := [3]int{255, 255, 255}
	bestCount := -1
	for rgb, count := range counts {
		if count > bestCount {
			best = rgb
			bestCount = count
		}
	}
	return best
}

func vectorizeRGB8(c color.Color) [3]int {
	r, g, b, a := c.RGBA()
	alpha := int(a >> 8)
	if alpha >= 255 {
		return [3]int{int(r >> 8), int(g >> 8), int(b >> 8)}
	}
	return [3]int{
		((int(r>>8) * alpha) + 255*(255-alpha)) / 255,
		((int(g>>8) * alpha) + 255*(255-alpha)) / 255,
		((int(b>>8) * alpha) + 255*(255-alpha)) / 255,
	}
}

func vectorizeSVGBackgroundFill(svg string) ([3]int, bool) {
	match := regexp.MustCompile(`<rect\b[^>]*\bfill="(#[0-9A-Fa-f]{6})"`).FindStringSubmatch(svg)
	if match == nil {
		return [3]int{}, false
	}
	return vectorizeParseHexColor(match[1])
}

func vectorizeAssertBrandColorDelta(t *testing.T, svg string, expectedFills []string, maxDelta float64) {
	t.Helper()
	actualFills := vectorizeSVGFillValues(svg)
	if len(actualFills) == 0 {
		t.Fatalf("svg does not contain fill colors")
	}
	for _, expected := range expectedFills {
		expectedColor, ok := vectorizeParseHexColor(expected)
		if !ok {
			t.Fatalf("invalid expected fill %q", expected)
		}
		best := float64(1 << 30)
		bestFill := ""
		for _, actual := range actualFills {
			actualColor, ok := vectorizeParseHexColor(strings.TrimPrefix(actual, `fill="`))
			if !ok {
				continue
			}
			delta := vectorizeRGBDistance(expectedColor, actualColor)
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

func vectorizeParseHexColor(value string) ([3]int, bool) {
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

func vectorizeRGBDistance(a [3]int, b [3]int) float64 {
	dr := float64(a[0] - b[0])
	dg := float64(a[1] - b[1])
	db := float64(a[2] - b[2])
	return math.Sqrt(dr*dr + dg*dg + db*db)
}

func vectorizeContainsAnyFill(svg string, fills []string) bool {
	for _, fill := range fills {
		if vectorizeContainsFill(svg, fill) {
			return true
		}
	}
	return false
}

func vectorizeContainsFill(svg string, fill string) bool {
	return strings.Contains(strings.ToLower(svg), `fill="`+strings.ToLower(fill)+`"`)
}

func vectorizeSVGFillValues(svg string) []string {
	matches := regexp.MustCompile(`fill="#[0-9A-Fa-f]{6}"`).FindAllString(svg, -1)
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

func vectorizeSampleSourcePath(fixtureName string, fallbackID string) string {
	candidates := []string{
		filepath.Join("..", "demos", "svg", "fixtures", fixtureName),
		filepath.Join("demos", "svg", "fixtures", fixtureName),
		filepath.Join("..", "demos", "svg", "outputs", fallbackID, "source.png"),
		filepath.Join("demos", "svg", "outputs", fallbackID, "source.png"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return candidates[0]
}
