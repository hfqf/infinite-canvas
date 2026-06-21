package main

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	colorMaskDefaultMinLayerRatio = 0.00008
	colorMaskDefaultMaxColors     = 32
	colorMaskSeedDistance         = 42
	colorMaskBackgroundDistance   = 30
	colorMaskForegroundDistance   = 24
)

type colorMaskLayer struct {
	traceLayer
	Index int
}

type colorMaskAssignment struct {
	Width      int
	Height     int
	Background traceLayer
	Layers     []colorMaskLayer
	Pixels     []int
}

func runColorMaskJob(ctx context.Context, input vectorizeRequest, data []byte) (jobResult, error) {
	magickPath, err := requireCommand("magick")
	if err != nil {
		return jobResult{}, err
	}
	potracePath, err := requireCommand("potrace")
	if err != nil {
		return jobResult{}, err
	}
	jobID := newID()
	jobDir := filepath.Join(outputRoot(), jobID)
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		return jobResult{}, err
	}

	sourcePath := filepath.Join(jobDir, "source.png")
	normalizedPath := filepath.Join(jobDir, "normalized.png")
	clusteredPath := filepath.Join(jobDir, "clustered.png")
	outputSVGPath := filepath.Join(jobDir, "output.svg")
	previewPath := filepath.Join(jobDir, "preview.png")
	if err := writeInputPNG(data, sourcePath); err != nil {
		return jobResult{}, err
	}

	longEdge := input.LongEdge
	if longEdge <= 0 {
		longEdge = defaultLongEdge
	}
	maxColors := input.Colors
	if maxColors <= 0 {
		maxColors = colorMaskDefaultMaxColors
	}
	if maxColors < 2 {
		maxColors = 2
	}
	if err := normalizePNG(ctx, magickPath, sourcePath, normalizedPath, longEdge); err != nil {
		return jobResult{}, err
	}

	sourceColors, _ := countUniqueColors(sourcePath)
	img, width, height, err := readPNGImage(normalizedPath)
	if err != nil {
		return jobResult{}, err
	}
	assignment := buildColorMaskAssignment(img, maxColors)
	if len(assignment.Layers) == 0 {
		return jobResult{}, errors.New("no traceable color layers")
	}
	if err := writeColorMaskClusteredImage(clusteredPath, assignment); err != nil {
		return jobResult{}, err
	}

	removeSpeckles := boolDefault(input.RemoveSpeckles, true)
	fillSmallHoles := boolDefault(input.FillSmallHoles, true)
	minComponentRatio := input.MinComponentRatio
	if minComponentRatio <= 0 {
		minComponentRatio = colorMaskDefaultMinLayerRatio
	}
	maxHoleRatio := input.MaxHoleRatio
	if maxHoleRatio <= 0 {
		maxHoleRatio = defaultMaxHoleRatio
	}
	minComponentArea := int(math.Max(4, float64(width*height)*minComponentRatio))
	maxHoleArea := int(math.Max(4, float64(width*height)*maxHoleRatio))

	sort.SliceStable(assignment.Layers, func(i, j int) bool {
		return luma(assignment.Layers[i].R, assignment.Layers[i].G, assignment.Layers[i].B) > luma(assignment.Layers[j].R, assignment.Layers[j].G, assignment.Layers[j].B)
	})

	var body strings.Builder
	body.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	body.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`, width, height, width, height) + "\n")
	body.WriteString(fmt.Sprintf(`<rect width="100%%" height="100%%" fill="%s"/>`, assignment.Background.Hex) + "\n")

	metrics := metricsResult{SourceColors: sourceColors, QuantizedColors: len(assignment.Layers) + 1, Background: assignment.Background.Hex}
	for layerIndex, layer := range assignment.Layers {
		maskPath := filepath.Join(jobDir, fmt.Sprintf("color-%02d-mask.png", layerIndex))
		layerSVGPath := filepath.Join(jobDir, fmt.Sprintf("color-%02d.svg", layerIndex))
		if err := writeColorAssignmentMask(assignment, layer.Index, maskPath); err != nil {
			return jobResult{}, err
		}
		if err := smoothMask(ctx, magickPath, maskPath, input.MaskCloseRadius, 0); err != nil {
			return jobResult{}, err
		}
		stats, err := cleanMask(maskPath, removeSpeckles, fillSmallHoles, minComponentArea, maxHoleArea)
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
			makeArtifact(jobID, "clustered", clusteredPath),
			makeArtifact(jobID, "output.svg", outputSVGPath),
			makeArtifact(jobID, "preview.png", previewPath),
			makeArtifact(jobID, "metrics.json", filepath.Join(jobDir, "metrics.json")),
		},
		Metrics: metrics,
	}, nil
}

func buildColorMaskAssignment(img image.Image, maxColors int) colorMaskAssignment {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	background := detectColorMaskBackground(img)
	seeds := selectColorMaskSeeds(img, background, maxColors)
	layers := make([]colorMaskLayer, 0, len(seeds))
	for index, seed := range seeds {
		layers = append(layers, colorMaskLayer{
			Index: index,
			traceLayer: traceLayer{
				Hex: seed.Hex,
				R:   seed.R,
				G:   seed.G,
				B:   seed.B,
				Keys: map[uint32]struct{}{
					rgbKey(seed.R, seed.G, seed.B): {},
				},
			},
		})
	}

	pixels := make([]int, width*height)
	for index := range pixels {
		pixels[index] = -1
	}
	type sums struct {
		r, g, b int64
		count   int
	}
	layerSums := make([]sums, len(layers))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r, g, b := rgb8(img.At(bounds.Min.X+x, bounds.Min.Y+y))
			if colorDistanceRGB(r, g, b, background.R, background.G, background.B) <= colorMaskBackgroundDistance && saturation(r, g, b) < 0.18 {
				continue
			}
			index := nearestColorMaskLayer(r, g, b, layers)
			if index < 0 {
				continue
			}
			pixels[y*width+x] = index
			layerSums[index].r += int64(r)
			layerSums[index].g += int64(g)
			layerSums[index].b += int64(b)
			layerSums[index].count++
		}
	}

	minLayerPixels := max(12, int(float64(width*height)*colorMaskDefaultMinLayerRatio))
	remap := make([]int, len(layers))
	for index := range remap {
		remap[index] = -1
	}
	kept := make([]colorMaskLayer, 0, len(layers))
	for index, layer := range layers {
		if layerSums[index].count < minLayerPixels {
			continue
		}
		count := layerSums[index].count
		r := uint8(layerSums[index].r / int64(count))
		g := uint8(layerSums[index].g / int64(count))
		b := uint8(layerSums[index].b / int64(count))
		layer.Index = len(kept)
		layer.R = r
		layer.G = g
		layer.B = b
		layer.Hex = hexColor(r, g, b)
		layer.Count = count
		remap[index] = layer.Index
		kept = append(kept, layer)
	}
	for index, value := range pixels {
		if value < 0 || remap[value] < 0 {
			pixels[index] = -1
			continue
		}
		pixels[index] = remap[value]
	}
	assignment := colorMaskAssignment{Width: width, Height: height, Background: background, Layers: kept, Pixels: pixels}
	mergeColorMaskAssignmentLayers(&assignment)
	suppressColorMaskEdgeLayers(&assignment)
	return assignment
}

func mergeColorMaskAssignmentLayers(assignment *colorMaskAssignment) {
	for {
		sourceIndex, targetIndex := findColorMaskMergePair(assignment.Layers)
		if sourceIndex < 0 {
			return
		}
		source := assignment.Layers[sourceIndex]
		target := assignment.Layers[targetIndex]
		combinedCount := max(1, source.Count+target.Count)
		mergedR := uint8((int(source.R)*source.Count + int(target.R)*target.Count) / combinedCount)
		mergedG := uint8((int(source.G)*source.Count + int(target.G)*target.Count) / combinedCount)
		mergedB := uint8((int(source.B)*source.Count + int(target.B)*target.Count) / combinedCount)
		for index, value := range assignment.Pixels {
			if value == sourceIndex {
				assignment.Pixels[index] = targetIndex
			} else if value > sourceIndex {
				assignment.Pixels[index] = value - 1
			}
		}
		assignment.Layers = append(assignment.Layers[:sourceIndex], assignment.Layers[sourceIndex+1:]...)
		if sourceIndex < targetIndex {
			targetIndex--
		}
		assignment.Layers[targetIndex].R = mergedR
		assignment.Layers[targetIndex].G = mergedG
		assignment.Layers[targetIndex].B = mergedB
		assignment.Layers[targetIndex].Hex = hexColor(mergedR, mergedG, mergedB)
		assignment.Layers[targetIndex].Count = combinedCount
		for index := range assignment.Layers {
			assignment.Layers[index].Index = index
		}
	}
}

func findColorMaskMergePair(layers []colorMaskLayer) (int, int) {
	for i := 0; i < len(layers); i++ {
		for j := i + 1; j < len(layers); j++ {
			a := paletteColor{R: layers[i].R, G: layers[i].G, B: layers[i].B, Count: layers[i].Count}
			b := paletteColor{R: layers[j].R, G: layers[j].G, B: layers[j].B, Count: layers[j].Count}
			if !sameColorMaskFamily(a, b) {
				continue
			}
			if layers[i].Count >= layers[j].Count {
				return j, i
			}
			return i, j
		}
	}
	return -1, -1
}

func suppressColorMaskEdgeLayers(assignment *colorMaskAssignment) {
	totalPixels := assignment.Width * assignment.Height
	if totalPixels <= 0 || len(assignment.Layers) == 0 {
		return
	}
	remove := make([]bool, len(assignment.Layers))
	for index, layer := range assignment.Layers {
		if isColorMaskEdgeLayer(layer, assignment.Layers, totalPixels) {
			remove[index] = true
		}
	}
	remap := make([]int, len(assignment.Layers))
	nextIndex := 0
	for index := range assignment.Layers {
		if remove[index] {
			remap[index] = -1
			continue
		}
		remap[index] = nextIndex
		nextIndex++
	}
	if nextIndex == len(assignment.Layers) {
		return
	}
	for index, value := range assignment.Pixels {
		if value < 0 || value >= len(remap) || remap[value] < 0 {
			assignment.Pixels[index] = -1
			continue
		}
		assignment.Pixels[index] = remap[value]
	}
	layers := make([]colorMaskLayer, 0, nextIndex)
	for index, layer := range assignment.Layers {
		if remove[index] {
			continue
		}
		layer.Index = len(layers)
		layers = append(layers, layer)
	}
	assignment.Layers = layers
}

func isColorMaskEdgeLayer(layer colorMaskLayer, layers []colorMaskLayer, totalPixels int) bool {
	if layer.Count > max(24, int(float64(totalPixels)*0.006)) {
		return false
	}
	layerSat := saturation(layer.R, layer.G, layer.B)
	layerLuma := luma(layer.R, layer.G, layer.B)
	if layerSat > 0.55 && layerLuma < 180 {
		return false
	}
	for _, target := range layers {
		if target.Index == layer.Index || target.Count < layer.Count*5 {
			continue
		}
		targetSat := saturation(target.R, target.G, target.B)
		targetLuma := luma(target.R, target.G, target.B)
		if targetSat <= layerSat && targetLuma >= layerLuma {
			continue
		}
		if hueGap(layer.R, layer.G, layer.B, target.R, target.G, target.B) <= 24 {
			return true
		}
		if sameChannelBias(layer.R, layer.G, layer.B, target.R, target.G, target.B) && hueGap(layer.R, layer.G, layer.B, target.R, target.G, target.B) <= 42 {
			return true
		}
	}
	return false
}

func detectColorMaskBackground(img image.Image) traceLayer {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	counts := make(map[uint32]int)
	add := func(x int, y int) {
		r, g, b := rgb8(img.At(bounds.Min.X+x, bounds.Min.Y+y))
		key := quantizedColorKey(r, g, b, 5)
		counts[key]++
	}
	for x := 0; x < width; x++ {
		add(x, 0)
		add(x, height-1)
	}
	for y := 1; y < height-1; y++ {
		add(0, y)
		add(width-1, y)
	}
	var best uint32
	bestCount := -1
	for key, count := range counts {
		if count > bestCount {
			best = key
			bestCount = count
		}
	}
	r, g, b := dequantizedColor(best, 5)
	return traceLayer{Hex: hexColor(r, g, b), R: r, G: g, B: b, Count: bestCount, Keys: map[uint32]struct{}{rgbKey(r, g, b): {}}}
}

func selectColorMaskSeeds(img image.Image, background traceLayer, maxColors int) []paletteColor {
	bounds := img.Bounds()
	counts := make(map[uint32]int)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b := rgb8(img.At(x, y))
			if colorDistanceRGB(r, g, b, background.R, background.G, background.B) <= colorMaskForegroundDistance && saturation(r, g, b) < 0.18 {
				continue
			}
			if isNearWhiteRGB(r, g, b) && isNearWhiteRGB(background.R, background.G, background.B) {
				continue
			}
			key := quantizedColorKey(r, g, b, 5)
			counts[key]++
		}
	}
	candidates := make([]paletteColor, 0, len(counts))
	for key, count := range counts {
		r, g, b := dequantizedColor(key, 5)
		candidates = append(candidates, paletteColor{Hex: hexColor(r, g, b), R: r, G: g, B: b, Count: count})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].Count > candidates[j].Count
	})
	minSeedPixels := max(8, bounds.Dx()*bounds.Dy()/20000)
	seeds := make([]paletteColor, 0, maxColors)
	for _, candidate := range candidates {
		if candidate.Count < minSeedPixels {
			break
		}
		if len(seeds) >= maxColors {
			break
		}
		tooClose := false
		for _, seed := range seeds {
			if colorDistanceRGB(candidate.R, candidate.G, candidate.B, seed.R, seed.G, seed.B) < colorMaskSeedDistance {
				tooClose = true
				break
			}
			if sameColorMaskFamily(candidate, seed) {
				tooClose = true
				break
			}
		}
		if tooClose {
			continue
		}
		seeds = append(seeds, candidate)
	}
	return seeds
}

func sameColorMaskFamily(a paletteColor, b paletteColor) bool {
	aSat := saturation(a.R, a.G, a.B)
	bSat := saturation(b.R, b.G, b.B)
	aLuma := luma(a.R, a.G, a.B)
	bLuma := luma(b.R, b.G, b.B)
	aIsLight := aLuma > 150
	bIsLight := bLuma > 150
	if aIsLight && bIsLight && sameChannelBias(a.R, a.G, a.B, b.R, b.G, b.B) {
		return true
	}
	if aSat < 0.38 && bSat < 0.38 && sameChannelBias(a.R, a.G, a.B, b.R, b.G, b.B) {
		return hueGap(a.R, a.G, a.B, b.R, b.G, b.B) <= 48
	}
	if aIsLight != bIsLight {
		return false
	}
	if math.Min(aSat, bSat) < 0.65 && sameChannelBias(a.R, a.G, a.B, b.R, b.G, b.B) {
		return hueGap(a.R, a.G, a.B, b.R, b.G, b.B) <= 54
	}
	if aSat >= 0.38 && bSat >= 0.38 {
		return sameFunctionalHue(a.R, a.G, a.B, b.R, b.G, b.B, 8, 28)
	}
	return false
}

func nearestColorMaskLayer(r uint8, g uint8, b uint8, layers []colorMaskLayer) int {
	bestIndex := -1
	bestDistance := math.MaxFloat64
	for index, layer := range layers {
		distance := colorDistanceRGB(r, g, b, layer.R, layer.G, layer.B)
		if distance < bestDistance {
			bestDistance = distance
			bestIndex = index
		}
	}
	return bestIndex
}

func writeColorAssignmentMask(assignment colorMaskAssignment, layerIndex int, outputPath string) error {
	out := image.NewRGBA(image.Rect(0, 0, assignment.Width, assignment.Height))
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	black := color.RGBA{A: 255}
	for y := 0; y < assignment.Height; y++ {
		for x := 0; x < assignment.Width; x++ {
			if assignment.Pixels[y*assignment.Width+x] == layerIndex {
				out.Set(x, y, black)
			} else {
				out.Set(x, y, white)
			}
		}
	}
	return writePNG(outputPath, out)
}

func writeColorMaskClusteredImage(path string, assignment colorMaskAssignment) error {
	out := image.NewRGBA(image.Rect(0, 0, assignment.Width, assignment.Height))
	background := color.RGBA{R: assignment.Background.R, G: assignment.Background.G, B: assignment.Background.B, A: 255}
	for y := 0; y < assignment.Height; y++ {
		for x := 0; x < assignment.Width; x++ {
			index := assignment.Pixels[y*assignment.Width+x]
			if index < 0 || index >= len(assignment.Layers) {
				out.Set(x, y, background)
				continue
			}
			layer := assignment.Layers[index]
			out.Set(x, y, color.RGBA{R: layer.R, G: layer.G, B: layer.B, A: 255})
		}
	}
	return writePNG(path, out)
}

func quantizedColorKey(r uint8, g uint8, b uint8, bits uint) uint32 {
	shift := 8 - bits
	return uint32(r>>shift)<<(bits*2) | uint32(g>>shift)<<bits | uint32(b>>shift)
}

func dequantizedColor(key uint32, bits uint) (uint8, uint8, uint8) {
	mask := uint32(1<<bits) - 1
	scale := float64(255) / float64(mask)
	b := uint8(math.Round(float64(key&mask) * scale))
	g := uint8(math.Round(float64((key>>bits)&mask) * scale))
	r := uint8(math.Round(float64((key>>(bits*2))&mask) * scale))
	return r, g, b
}

func colorDistanceRGB(ar uint8, ag uint8, ab uint8, br uint8, bg uint8, bb uint8) float64 {
	dr := float64(int(ar) - int(br))
	dg := float64(int(ag) - int(bg))
	db := float64(int(ab) - int(bb))
	return math.Sqrt(dr*dr + dg*dg + db*db)
}

func isNearWhiteRGB(r uint8, g uint8, b uint8) bool {
	return min3(r, g, b) > 235 && max3(r, g, b)-min3(r, g, b) < 28
}
