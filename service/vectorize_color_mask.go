package service

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	colorMaskDefaultMinLayerRatio = 0.00008
	colorMaskDefaultMaxColors     = 32
	colorMaskSeedDistance         = 42
	colorMaskBackgroundDistance   = 30
	colorMaskForegroundDistance   = 24
	colorMaskMaxLongEdge          = 4096
	colorMaskMinLongEdge          = 1024
)

type colorMaskLayer struct {
	Hex   string
	R     uint8
	G     uint8
	B     uint8
	Count int
	Index int
}

type colorMaskAssignment struct {
	Width      int
	Height     int
	Background colorMaskLayer
	Layers     []colorMaskLayer
	Pixels     []int
}

type colorMaskPaletteColor struct {
	Hex   string
	R     uint8
	G     uint8
	B     uint8
	Count int
}

func vectorizeColorMaskImage(ctx context.Context, tempDir string, inputPath string, outputPath string) error {
	imageMagickPath, err := resolveImageMagickPath()
	if err != nil {
		return err
	}
	potracePath, err := resolveRequiredCommand("potrace", "后端未安装 potrace，请安装 potrace")
	if err != nil {
		return err
	}
	normalizedPath := filepath.Join(tempDir, "color-mask-normalized.png")
	longEdge, err := colorMaskTargetLongEdge(ctx, imageMagickPath, inputPath)
	if err != nil {
		return err
	}
	if err := normalizeColorMaskPNG(ctx, imageMagickPath, inputPath, normalizedPath, longEdge); err != nil {
		return err
	}
	img, width, height, err := readPNGImage(normalizedPath)
	if err != nil {
		return err
	}
	assignment := buildColorMaskAssignment(img, colorMaskDefaultMaxColors)
	if len(assignment.Layers) == 0 {
		return safeMessageError{message: "图片没有可追踪的有效色块"}
	}

	var body strings.Builder
	body.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	body.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`, width, height, width, height) + "\n")
	body.WriteString(fmt.Sprintf(`<rect width="100%%" height="100%%" fill="%s"/>`, assignment.Background.Hex) + "\n")

	sort.SliceStable(assignment.Layers, func(i, j int) bool {
		return colorMaskLuma(assignment.Layers[i].R, assignment.Layers[i].G, assignment.Layers[i].B) > colorMaskLuma(assignment.Layers[j].R, assignment.Layers[j].G, assignment.Layers[j].B)
	})
	minComponentArea := int(math.Max(4, float64(width*height)*colorMaskDefaultMinLayerRatio))
	maxHoleArea := int(math.Max(4, float64(width*height)*0.00004))
	for index, layer := range assignment.Layers {
		smoothLayer := shouldSmoothColorMaskLayer(layer, width*height)
		maskPath := filepath.Join(tempDir, fmt.Sprintf("color-mask-%d.png", index))
		layerSVGPath := filepath.Join(tempDir, fmt.Sprintf("color-mask-%d.svg", index))
		if err := writeColorMaskLayerMask(assignment, layer.Index, maskPath); err != nil {
			return err
		}
		stats, err := cleanColorMask(maskPath, minComponentArea, maxHoleArea, smoothLayer)
		if err != nil {
			return err
		}
		if stats.RemainingBlack == 0 {
			continue
		}
		if err := runColorMaskPotrace(ctx, imageMagickPath, potracePath, maskPath, layerSVGPath, layer.Hex, smoothLayer); err != nil {
			return err
		}
		group, err := readColorMaskPotraceGroup(layerSVGPath)
		if err != nil {
			return err
		}
		body.WriteString(group)
		body.WriteString("\n")
	}
	body.WriteString("</svg>\n")
	return os.WriteFile(outputPath, []byte(body.String()), 0o600)
}

func colorMaskTargetLongEdge(ctx context.Context, imageMagickPath string, inputPath string) (int, error) {
	output, err := exec.CommandContext(ctx, imageMagickPath, "identify", "-format", "%w %h", inputPath).Output()
	if err != nil && filepath.Base(imageMagickPath) != "magick" {
		output, err = exec.CommandContext(ctx, "identify", "-format", "%w %h", inputPath).Output()
	}
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return 0, safeMessageError{message: "图片尺寸读取超时，请稍后重试"}
		}
		return 0, err
	}
	fields := strings.Fields(string(output))
	if len(fields) < 2 {
		return 0, safeMessageError{message: "图片尺寸读取失败"}
	}
	width, _ := strconv.Atoi(fields[0])
	height, _ := strconv.Atoi(fields[1])
	longEdge := max(width, height)
	if longEdge <= 0 {
		return 0, safeMessageError{message: "图片尺寸无效"}
	}
	target := longEdge * 4
	if target < colorMaskMinLongEdge {
		target = colorMaskMinLongEdge
	}
	if target > colorMaskMaxLongEdge {
		target = colorMaskMaxLongEdge
	}
	return target, nil
}

func normalizeColorMaskPNG(ctx context.Context, imageMagickPath string, inputPath string, outputPath string, longEdge int) error {
	args := []string{
		inputPath,
		"-auto-orient",
		"-alpha", "remove",
		"-background", "white",
		"-resize", fmt.Sprintf("%dx%d", longEdge, longEdge),
		"-colorspace", "sRGB",
		"-strip",
		outputPath,
	}
	cmd := exec.CommandContext(ctx, imageMagickPath, args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return safeMessageError{message: "图片预处理超时，请稍后重试"}
		}
		return fmt.Errorf("imagemagick color mask normalize failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
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

func buildColorMaskAssignment(img image.Image, maxColors int) colorMaskAssignment {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	background := detectColorMaskBackground(img)
	seeds := selectColorMaskSeeds(img, background, maxColors)
	layers := make([]colorMaskLayer, 0, len(seeds))
	for index, seed := range seeds {
		layers = append(layers, colorMaskLayer{Index: index, Hex: seed.Hex, R: seed.R, G: seed.G, B: seed.B})
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
			r, g, b := colorMaskRGB8(img.At(bounds.Min.X+x, bounds.Min.Y+y))
			if colorMaskColorDistance(r, g, b, background.R, background.G, background.B) <= colorMaskBackgroundDistance && colorMaskSaturation(r, g, b) < 0.18 {
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
		layer.Hex = colorMaskHex(r, g, b)
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
	absorbColorMaskLightLayerPixels(&assignment, img)
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
		assignment.Layers[targetIndex].Hex = colorMaskHex(mergedR, mergedG, mergedB)
		assignment.Layers[targetIndex].Count = combinedCount
		for index := range assignment.Layers {
			assignment.Layers[index].Index = index
		}
	}
}

func findColorMaskMergePair(layers []colorMaskLayer) (int, int) {
	for i := 0; i < len(layers); i++ {
		for j := i + 1; j < len(layers); j++ {
			a := colorMaskPaletteColor{R: layers[i].R, G: layers[i].G, B: layers[i].B, Count: layers[i].Count}
			b := colorMaskPaletteColor{R: layers[j].R, G: layers[j].G, B: layers[j].B, Count: layers[j].Count}
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
	layerSat := colorMaskSaturation(layer.R, layer.G, layer.B)
	layerLuma := colorMaskLuma(layer.R, layer.G, layer.B)
	if layerSat > 0.55 && layerLuma < 180 {
		return false
	}
	for _, target := range layers {
		if target.Index == layer.Index || target.Count < layer.Count*5 {
			continue
		}
		targetSat := colorMaskSaturation(target.R, target.G, target.B)
		targetLuma := colorMaskLuma(target.R, target.G, target.B)
		if targetSat <= layerSat && targetLuma >= layerLuma {
			continue
		}
		if colorMaskHueGap(layer.R, layer.G, layer.B, target.R, target.G, target.B) <= 24 {
			return true
		}
		if colorMaskSameChannelBias(layer.R, layer.G, layer.B, target.R, target.G, target.B) && colorMaskHueGap(layer.R, layer.G, layer.B, target.R, target.G, target.B) <= 42 {
			return true
		}
	}
	return false
}

func absorbColorMaskLightLayerPixels(assignment *colorMaskAssignment, img image.Image) {
	totalPixels := assignment.Width * assignment.Height
	if totalPixels <= 0 {
		return
	}
	eligible := make([]bool, len(assignment.Layers))
	for index, layer := range assignment.Layers {
		eligible[index] = shouldSmoothColorMaskLayer(layer, totalPixels)
	}
	bounds := img.Bounds()
	directions := [...]image.Point{
		{X: -1, Y: -1}, {Y: -1}, {X: 1, Y: -1},
		{X: -1}, {X: 1},
		{X: -1, Y: 1}, {Y: 1}, {X: 1, Y: 1},
	}
	for pass := 0; pass < 3; pass++ {
		updates := make([]int, 0)
		targets := make([]int, 0)
		for y := 0; y < assignment.Height; y++ {
			for x := 0; x < assignment.Width; x++ {
				pixelIndex := y*assignment.Width + x
				if assignment.Pixels[pixelIndex] >= 0 {
					continue
				}
				r, g, b := colorMaskRGB8(img.At(bounds.Min.X+x, bounds.Min.Y+y))
				layerIndex := nearestAdjacentColorMaskLightLayer(assignment, eligible, directions[:], x, y, r, g, b)
				if layerIndex < 0 {
					continue
				}
				updates = append(updates, pixelIndex)
				targets = append(targets, layerIndex)
			}
		}
		if len(updates) == 0 {
			return
		}
		for index, pixelIndex := range updates {
			layerIndex := targets[index]
			assignment.Pixels[pixelIndex] = layerIndex
			assignment.Layers[layerIndex].Count++
		}
	}
}

func nearestAdjacentColorMaskLightLayer(assignment *colorMaskAssignment, eligible []bool, directions []image.Point, x int, y int, r uint8, g uint8, b uint8) int {
	bestIndex := -1
	bestDistance := math.MaxFloat64
	for _, direction := range directions {
		nx, ny := x+direction.X, y+direction.Y
		if nx < 0 || ny < 0 || nx >= assignment.Width || ny >= assignment.Height {
			continue
		}
		layerIndex := assignment.Pixels[ny*assignment.Width+nx]
		if layerIndex < 0 || layerIndex >= len(assignment.Layers) || !eligible[layerIndex] {
			continue
		}
		layer := assignment.Layers[layerIndex]
		if colorMaskIsNearWhite(r, g, b) && colorMaskColorDistance(r, g, b, layer.R, layer.G, layer.B) > 58 {
			continue
		}
		if !colorMaskSameChannelBias(r, g, b, layer.R, layer.G, layer.B) {
			continue
		}
		distance := colorMaskColorDistance(r, g, b, layer.R, layer.G, layer.B)
		if distance > 82 || math.Abs(colorMaskLuma(r, g, b)-colorMaskLuma(layer.R, layer.G, layer.B)) > 72 {
			continue
		}
		if distance < bestDistance {
			bestDistance = distance
			bestIndex = layerIndex
		}
	}
	return bestIndex
}

func detectColorMaskBackground(img image.Image) colorMaskLayer {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	counts := make(map[uint32]int)
	add := func(x int, y int) {
		r, g, b := colorMaskRGB8(img.At(bounds.Min.X+x, bounds.Min.Y+y))
		counts[colorMaskQuantizedKey(r, g, b, 5)]++
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
	r, g, b := colorMaskDequantizedColor(best, 5)
	return colorMaskLayer{Hex: colorMaskHex(r, g, b), R: r, G: g, B: b, Count: bestCount}
}

func selectColorMaskSeeds(img image.Image, background colorMaskLayer, maxColors int) []colorMaskPaletteColor {
	bounds := img.Bounds()
	counts := make(map[uint32]int)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b := colorMaskRGB8(img.At(x, y))
			if colorMaskColorDistance(r, g, b, background.R, background.G, background.B) <= colorMaskForegroundDistance && colorMaskSaturation(r, g, b) < 0.18 {
				continue
			}
			if colorMaskIsNearWhite(r, g, b) && colorMaskIsNearWhite(background.R, background.G, background.B) {
				continue
			}
			counts[colorMaskQuantizedKey(r, g, b, 5)]++
		}
	}
	candidates := make([]colorMaskPaletteColor, 0, len(counts))
	for key, count := range counts {
		r, g, b := colorMaskDequantizedColor(key, 5)
		candidates = append(candidates, colorMaskPaletteColor{Hex: colorMaskHex(r, g, b), R: r, G: g, B: b, Count: count})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].Count > candidates[j].Count
	})
	minSeedPixels := max(8, bounds.Dx()*bounds.Dy()/20000)
	seeds := make([]colorMaskPaletteColor, 0, maxColors)
	for _, candidate := range candidates {
		if candidate.Count < minSeedPixels || len(seeds) >= maxColors {
			break
		}
		tooClose := false
		for _, seed := range seeds {
			if colorMaskColorDistance(candidate.R, candidate.G, candidate.B, seed.R, seed.G, seed.B) < colorMaskSeedDistance || sameColorMaskFamily(candidate, seed) {
				tooClose = true
				break
			}
		}
		if !tooClose {
			seeds = append(seeds, candidate)
		}
	}
	return seeds
}

func sameColorMaskFamily(a colorMaskPaletteColor, b colorMaskPaletteColor) bool {
	aSat := colorMaskSaturation(a.R, a.G, a.B)
	bSat := colorMaskSaturation(b.R, b.G, b.B)
	aLuma := colorMaskLuma(a.R, a.G, a.B)
	bLuma := colorMaskLuma(b.R, b.G, b.B)
	aIsLight := aLuma > 150
	bIsLight := bLuma > 150
	if aIsLight && bIsLight && colorMaskSameChannelBias(a.R, a.G, a.B, b.R, b.G, b.B) {
		return true
	}
	if aSat < 0.38 && bSat < 0.38 && colorMaskSameChannelBias(a.R, a.G, a.B, b.R, b.G, b.B) {
		return colorMaskHueGap(a.R, a.G, a.B, b.R, b.G, b.B) <= 48
	}
	if aIsLight != bIsLight {
		return false
	}
	if math.Min(aSat, bSat) < 0.65 && colorMaskSameChannelBias(a.R, a.G, a.B, b.R, b.G, b.B) {
		return colorMaskHueGap(a.R, a.G, a.B, b.R, b.G, b.B) <= 54
	}
	if aSat >= 0.38 && bSat >= 0.38 {
		return colorMaskHueGap(a.R, a.G, a.B, b.R, b.G, b.B) <= 8 && math.Abs(colorMaskLuma(a.R, a.G, a.B)-colorMaskLuma(b.R, b.G, b.B)) <= 28
	}
	return false
}

func nearestColorMaskLayer(r uint8, g uint8, b uint8, layers []colorMaskLayer) int {
	bestIndex := -1
	bestDistance := math.MaxFloat64
	for index, layer := range layers {
		distance := colorMaskColorDistance(r, g, b, layer.R, layer.G, layer.B)
		if distance < bestDistance {
			bestDistance = distance
			bestIndex = index
		}
	}
	return bestIndex
}

func writeColorMaskLayerMask(assignment colorMaskAssignment, layerIndex int, outputPath string) error {
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

type colorMaskCleanStats struct {
	RemainingBlack int
}

func cleanColorMask(path string, minComponentArea int, maxHoleArea int, smooth bool) (colorMaskCleanStats, error) {
	img, width, height, err := readColorMaskMask(path)
	if err != nil {
		return colorMaskCleanStats{}, err
	}
	if smooth {
		closeColorMaskBlack(img, width, height, 2)
	}
	removeColorMaskSmallComponents(img, width, height, minComponentArea)
	fillColorMaskHoles(img, width, height, maxHoleArea)
	stats := colorMaskCleanStats{RemainingBlack: countColorMaskBlack(img)}
	return stats, writePNG(path, img)
}

func shouldSmoothColorMaskLayer(layer colorMaskLayer, totalPixels int) bool {
	if totalPixels <= 0 || layer.Count < int(float64(totalPixels)*0.01) {
		return false
	}
	return colorMaskSaturation(layer.R, layer.G, layer.B) < 0.45 && colorMaskLuma(layer.R, layer.G, layer.B) > 120
}

func readColorMaskMask(path string) (*image.RGBA, int, int, error) {
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
			if colorMaskIsBlack(src.At(bounds.Min.X+x, bounds.Min.Y+y)) {
				out.Set(x, y, black)
			} else {
				out.Set(x, y, white)
			}
		}
	}
	return out, width, height, nil
}

func removeColorMaskSmallComponents(img *image.RGBA, width int, height int, minArea int) {
	visited := make([]bool, width*height)
	walkColorMaskComponents(width, height, visited, func(x int, y int) bool {
		return colorMaskIsBlack(img.At(x, y))
	}, func(points []image.Point, _ bool) {
		if len(points) >= minArea {
			return
		}
		for _, point := range points {
			img.Set(point.X, point.Y, color.RGBA{R: 255, G: 255, B: 255, A: 255})
		}
	})
}

func fillColorMaskHoles(img *image.RGBA, width int, height int, maxArea int) {
	visited := make([]bool, width*height)
	walkColorMaskComponents(width, height, visited, func(x int, y int) bool {
		return !colorMaskIsBlack(img.At(x, y))
	}, func(points []image.Point, touchesBorder bool) {
		if touchesBorder || len(points) > maxArea {
			return
		}
		for _, point := range points {
			img.Set(point.X, point.Y, color.RGBA{A: 255})
		}
	})
}

func closeColorMaskBlack(img *image.RGBA, width int, height int, radius int) {
	dilated := image.NewRGBA(image.Rect(0, 0, width, height))
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	black := color.RGBA{A: 255}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			hasBlack := false
			for dy := -radius; dy <= radius && !hasBlack; dy++ {
				for dx := -radius; dx <= radius; dx++ {
					nx, ny := x+dx, y+dy
					if nx >= 0 && ny >= 0 && nx < width && ny < height && colorMaskIsBlack(img.At(nx, ny)) {
						hasBlack = true
						break
					}
				}
			}
			if hasBlack {
				dilated.Set(x, y, black)
			} else {
				dilated.Set(x, y, white)
			}
		}
	}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			allBlack := true
			for dy := -radius; dy <= radius && allBlack; dy++ {
				for dx := -radius; dx <= radius; dx++ {
					nx, ny := x+dx, y+dy
					if nx < 0 || ny < 0 || nx >= width || ny >= height || !colorMaskIsBlack(dilated.At(nx, ny)) {
						allBlack = false
						break
					}
				}
			}
			if allBlack {
				img.Set(x, y, black)
			} else {
				img.Set(x, y, white)
			}
		}
	}
}

func walkColorMaskComponents(width int, height int, visited []bool, match func(int, int) bool, handle func([]image.Point, bool)) {
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

func countColorMaskBlack(img *image.RGBA) int {
	bounds := img.Bounds()
	count := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if colorMaskIsBlack(img.At(x, y)) {
				count++
			}
		}
	}
	return count
}

func runColorMaskPotrace(ctx context.Context, imageMagickPath string, potracePath string, maskPath string, outputPath string, fill string, smooth bool) error {
	bitmapPath := strings.TrimSuffix(outputPath, filepath.Ext(outputPath)) + ".pbm"
	convertArgs := []string{maskPath}
	if smooth {
		convertArgs = append(convertArgs, "-colorspace", "Gray", "-blur", "0x1", "-threshold", "50%")
	}
	convertArgs = append(convertArgs, "-type", "bilevel", "pbm:"+bitmapPath)
	if output, err := exec.CommandContext(ctx, imageMagickPath, convertArgs...).CombinedOutput(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return safeMessageError{message: "色层位图转换超时，请稍后重试"}
		}
		return fmt.Errorf("imagemagick color mask pbm failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	potraceArgs := []string{bitmapPath, "-s", "--group", "--flat", "-t", "8", "-a", "0.75", "-O", "0.90", "-u", "10", "-C", fill, "-o", outputPath}
	if output, err := exec.CommandContext(ctx, potracePath, potraceArgs...).CombinedOutput(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return safeMessageError{message: "Potrace 转换超时，请稍后重试"}
		}
		return fmt.Errorf("potrace color mask failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func readColorMaskPotraceGroup(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	group := svgGroupPattern.FindString(string(data))
	if group == "" {
		return "", nil
	}
	return group, nil
}

func colorMaskRGB8(c color.Color) (uint8, uint8, uint8) {
	r, g, b, _ := c.RGBA()
	return uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)
}

func colorMaskHex(r uint8, g uint8, b uint8) string {
	return fmt.Sprintf("#%02X%02X%02X", r, g, b)
}

func colorMaskQuantizedKey(r uint8, g uint8, b uint8, bits uint) uint32 {
	shift := 8 - bits
	return uint32(r>>shift)<<(bits*2) | uint32(g>>shift)<<bits | uint32(b>>shift)
}

func colorMaskDequantizedColor(key uint32, bits uint) (uint8, uint8, uint8) {
	mask := uint32(1<<bits) - 1
	scale := float64(255) / float64(mask)
	b := uint8(math.Round(float64(key&mask) * scale))
	g := uint8(math.Round(float64((key>>bits)&mask) * scale))
	r := uint8(math.Round(float64((key>>(bits*2))&mask) * scale))
	return r, g, b
}

func colorMaskColorDistance(ar uint8, ag uint8, ab uint8, br uint8, bg uint8, bb uint8) float64 {
	dr := float64(int(ar) - int(br))
	dg := float64(int(ag) - int(bg))
	db := float64(int(ab) - int(bb))
	return math.Sqrt(dr*dr + dg*dg + db*db)
}

func colorMaskLuma(r uint8, g uint8, b uint8) float64 {
	return 0.2126*float64(r) + 0.7152*float64(g) + 0.0722*float64(b)
}

func colorMaskHueAndSaturation(r uint8, g uint8, b uint8) (float64, float64) {
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

func colorMaskSaturation(r uint8, g uint8, b uint8) float64 {
	_, sat := colorMaskHueAndSaturation(r, g, b)
	return sat
}

func colorMaskHueGap(ar uint8, ag uint8, ab uint8, br uint8, bg uint8, bb uint8) float64 {
	aHue, _ := colorMaskHueAndSaturation(ar, ag, ab)
	bHue, _ := colorMaskHueAndSaturation(br, bg, bb)
	diff := math.Abs(aHue - bHue)
	if diff > 180 {
		return 360 - diff
	}
	return diff
}

func colorMaskSameChannelBias(ar uint8, ag uint8, ab uint8, br uint8, bg uint8, bb uint8) bool {
	return colorMaskChannelOrder(ar, ag, ab) == colorMaskChannelOrder(br, bg, bb)
}

func colorMaskChannelOrder(r uint8, g uint8, b uint8) string {
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

func colorMaskIsNearWhite(r uint8, g uint8, b uint8) bool {
	minChannel := min(r, min(g, b))
	maxChannel := max(r, max(g, b))
	return minChannel > 235 && maxChannel-minChannel < 28
}

func colorMaskIsBlack(c color.Color) bool {
	r, g, b, _ := c.RGBA()
	return int(r>>8)+int(g>>8)+int(b>>8) < 384
}
