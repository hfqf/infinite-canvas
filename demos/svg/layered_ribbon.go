package main

import (
	"context"
	"errors"
	"fmt"
	"image"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type ribbonLayer struct {
	layer traceLayer
	box   image.Rectangle
	boxes []image.Rectangle
}

func runLayeredRibbonJob(ctx context.Context, input vectorizeRequest, data []byte) (jobResult, error) {
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
	background := detectBackgroundLayer(quantizedImage, palette, defaultMergeDistance, defaultMergeHueDistance, defaultMergeLightness, defaultMergeSaturation)
	layers := buildTraceLayers(palette, width*height, background.Keys, defaultMergeDistance, defaultMergeHueDistance, defaultMergeLightness, defaultMergeSaturation)
	if len(layers) == 0 {
		return jobResult{}, errors.New("no traceable color layers")
	}
	sort.SliceStable(layers, func(i, j int) bool {
		return luma(layers[i].R, layers[i].G, layers[i].B) > luma(layers[j].R, layers[j].G, layers[j].B)
	})

	ribbons := make([]ribbonLayer, 0, 2)
	metrics := metricsResult{SourceColors: sourceColors, QuantizedColors: quantizedColors, Background: background.Hex}
	var body strings.Builder
	body.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	body.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`, width, height, width, height) + "\n")
	body.WriteString(fmt.Sprintf(`<rect width="100%%" height="100%%" fill="%s"/>`, background.Hex) + "\n")
	minComponentArea := int(math.Max(4, float64(width*height)*defaultMinComponentRatio))
	maxHoleArea := int(math.Max(4, float64(width*height)*defaultMaxHoleRatio))

	for _, layer := range layers {
		if ribbon, ok := detectRibbonLayer(quantizedImage, layer, width, height); ok {
			ribbons = append(ribbons, ribbon)
			continue
		}
	}
	for index, ribbon := range ribbons {
		maskPath := filepath.Join(jobDir, fmt.Sprintf("ribbon-%02d-mask.png", index))
		layerSVGPath := filepath.Join(jobDir, fmt.Sprintf("ribbon-%02d.svg", index))
		if err := writeLayerMask(quantizedImage, ribbon.layer, maskPath); err != nil {
			return jobResult{}, err
		}
		if err := smoothMask(ctx, magickPath, maskPath, 1, 0); err != nil {
			return jobResult{}, err
		}
		stats, err := cleanMask(maskPath, false, true, minComponentArea, maxHoleArea)
		if err != nil {
			return jobResult{}, err
		}
		if err := runPotrace(ctx, magickPath, potracePath, maskPath, layerSVGPath, ribbon.layer.Hex); err != nil {
			return jobResult{}, err
		}
		group, err := extractPotraceGroup(layerSVGPath)
		if err != nil {
			return jobResult{}, err
		}
		body.WriteString(group)
		body.WriteString("\n")
		metrics.Layers = append(metrics.Layers, layerMetrics{
			Hex:                ribbon.layer.Hex,
			Pixels:             ribbon.layer.Count,
			MaskURL:            artifactURL(jobID, filepath.Base(maskPath)),
			SVGURL:             artifactURL(jobID, filepath.Base(layerSVGPath)),
			HolesFilled:        stats.HolesFilled,
			HolePixelsFilled:   stats.HolePixelsFilled,
			RemainingBlackArea: stats.RemainingBlack,
		})
	}

	for index, layer := range layers {
		if isRibbonTraceLayer(ribbons, layer) {
			continue
		}
		maskPath := filepath.Join(jobDir, fmt.Sprintf("layer-%02d-mask.png", index))
		layerSVGPath := filepath.Join(jobDir, fmt.Sprintf("layer-%02d.svg", index))
		if err := writeLayerMask(quantizedImage, layer, maskPath); err != nil {
			return jobResult{}, err
		}
		stats, err := cleanMask(maskPath, true, true, minComponentArea, maxHoleArea)
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

func detectRibbonLayer(img image.Image, layer traceLayer, width int, height int) (ribbonLayer, bool) {
	if saturation(layer.R, layer.G, layer.B) >= 0.38 || luma(layer.R, layer.G, layer.B) < 125 {
		return ribbonLayer{}, false
	}
	box, boxes, count := significantLayerBounds(img, layer, width, height)
	if count == 0 {
		return ribbonLayer{}, false
	}
	boxWidth := box.Dx()
	boxHeight := box.Dy()
	if boxWidth < width/3 || boxHeight < height/18 || boxHeight > height/2 {
		return ribbonLayer{}, false
	}
	if float64(count)/float64(width*height) < 0.015 {
		return ribbonLayer{}, false
	}
	return ribbonLayer{layer: layer, box: box, boxes: boxes}, true
}

func significantLayerBounds(img image.Image, layer traceLayer, width int, height int) (image.Rectangle, []image.Rectangle, int) {
	bounds := img.Bounds()
	visited := make([]bool, width*height)
	queue := make([]image.Point, 0, 1024)
	directions := [...]image.Point{{X: 1}, {X: -1}, {Y: 1}, {Y: -1}}
	type componentBox struct {
		box   image.Rectangle
		count int
	}
	components := make([]componentBox, 0)
	largest := 0
	minArea := max(512, width*height/5000)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			index := y*width + x
			if visited[index] {
				continue
			}
			if _, ok := layer.Keys[colorKey(img.At(bounds.Min.X+x, bounds.Min.Y+y))]; !ok {
				visited[index] = true
				continue
			}
			visited[index] = true
			queue = append(queue[:0], image.Point{X: x, Y: y})
			cMinX, cMinY, cMaxX, cMaxY := x, y, x, y
			count := 0
			for len(queue) > 0 {
				point := queue[len(queue)-1]
				queue = queue[:len(queue)-1]
				count++
				if point.X < cMinX {
					cMinX = point.X
				}
				if point.Y < cMinY {
					cMinY = point.Y
				}
				if point.X > cMaxX {
					cMaxX = point.X
				}
				if point.Y > cMaxY {
					cMaxY = point.Y
				}
				for _, direction := range directions {
					next := image.Point{X: point.X + direction.X, Y: point.Y + direction.Y}
					if next.X < 0 || next.X >= width || next.Y < 0 || next.Y >= height {
						continue
					}
					nextIndex := next.Y*width + next.X
					if visited[nextIndex] {
						continue
					}
					if _, ok := layer.Keys[colorKey(img.At(bounds.Min.X+next.X, bounds.Min.Y+next.Y))]; !ok {
						visited[nextIndex] = true
						continue
					}
					visited[nextIndex] = true
					queue = append(queue, next)
				}
			}
			box := image.Rect(cMinX, cMinY, cMaxX+1, cMaxY+1)
			if count < minArea || box.Dx() < width/30 || box.Dy() < height/50 || float64(box.Dx())/float64(box.Dy()) < 1.45 {
				continue
			}
			components = append(components, componentBox{box: box, count: count})
			if count > largest {
				largest = count
			}
		}
	}
	totalCount := 0
	minX, minY := width, height
	maxX, maxY := -1, -1
	minKeepArea := max(minArea, int(float64(largest)*0.08))
	keptBoxes := make([]image.Rectangle, 0, len(components))
	for _, component := range components {
		if component.count < minKeepArea {
			continue
		}
		keptBoxes = append(keptBoxes, component.box)
		totalCount += component.count
		if component.box.Min.X < minX {
			minX = component.box.Min.X
		}
		if component.box.Min.Y < minY {
			minY = component.box.Min.Y
		}
		if component.box.Max.X-1 > maxX {
			maxX = component.box.Max.X - 1
		}
		if component.box.Max.Y-1 > maxY {
			maxY = component.box.Max.Y - 1
		}
	}
	if totalCount == 0 {
		return image.Rectangle{}, nil, 0
	}
	return image.Rect(minX, minY, maxX+1, maxY+1), keptBoxes, totalCount
}

func layerBounds(img image.Image, layer traceLayer) (image.Rectangle, int) {
	bounds := img.Bounds()
	minX, minY := bounds.Dx(), bounds.Dy()
	maxX, maxY := -1, -1
	count := 0
	for y := 0; y < bounds.Dy(); y++ {
		for x := 0; x < bounds.Dx(); x++ {
			if _, ok := layer.Keys[colorKey(img.At(bounds.Min.X+x, bounds.Min.Y+y))]; !ok {
				continue
			}
			count++
			if x < minX {
				minX = x
			}
			if y < minY {
				minY = y
			}
			if x > maxX {
				maxX = x
			}
			if y > maxY {
				maxY = y
			}
		}
	}
	if count == 0 {
		return image.Rectangle{}, 0
	}
	return image.Rect(minX, minY, maxX+1, maxY+1), count
}

func reconstructRibbonPath(img image.Image, ribbon ribbonLayer, width int, height int) string {
	layer := ribbon.layer
	columns := make([][]int, width)
	bounds := img.Bounds()
	startX := max(0, ribbon.box.Min.X)
	endX := min(width-1, ribbon.box.Max.X-1)
	startY := max(0, ribbon.box.Min.Y-height/30)
	endY := min(height-1, ribbon.box.Max.Y+height/30)
	for y := startY; y <= endY; y++ {
		for x := startX; x <= endX; x++ {
			if !pointInRibbonBoxes(x, y, ribbon.boxes) {
				continue
			}
			if _, ok := layer.Keys[colorKey(img.At(bounds.Min.X+x, bounds.Min.Y+y))]; !ok {
				continue
			}
			columns[x] = append(columns[x], y)
		}
	}
	heights := make([]int, 0, width)
	for x := startX; x <= endX; x++ {
		if len(columns[x]) > 0 {
			sort.Ints(columns[x])
			heights = append(heights, ribbonColumnBottom(columns[x])-ribbonColumnTop(columns[x])+1)
		}
	}
	sort.Ints(heights)
	minColumnHeight := 8
	if len(heights) > 0 {
		minColumnHeight = max(8, int(float64(heights[len(heights)/2])*0.45))
	}
	valid := make([]bool, width)
	first, last := -1, -1
	topValues := make([]int, width)
	bottomValues := make([]int, width)
	for x := startX; x <= endX; x++ {
		if len(columns[x]) >= 4 {
			topValues[x] = ribbonColumnTop(columns[x])
			bottomValues[x] = ribbonColumnBottom(columns[x])
		}
		valid[x] = len(columns[x]) >= 4 && bottomValues[x]-topValues[x]+1 >= minColumnHeight
		if valid[x] {
			if first < 0 {
				first = x
			}
			last = x
		}
	}
	if first < 0 || last <= first {
		return ""
	}
	top := interpolateRibbonEdge(topValues, valid, first, last)
	bottom := interpolateRibbonEdge(bottomValues, valid, first, last)
	top = smoothRibbonEdge(top, first, last, max(10, width/180))
	bottom = smoothRibbonEdge(bottom, first, last, max(10, width/180))

	step := max(8, width/260)
	topPoints := sampleRibbonPoints(top, first, last, step)
	bottomPoints := sampleRibbonPoints(bottom, first, last, step)
	if len(topPoints) < 2 || len(bottomPoints) < 2 {
		return ""
	}
	var body strings.Builder
	body.WriteString(fmt.Sprintf(`<path fill="%s" d="`, layer.Hex))
	body.WriteString(pointsToRibbonPath(topPoints, bottomPoints))
	body.WriteString(`"/>`)
	return body.String()
}

func ribbonColumnTop(values []int) int {
	if len(values) == 0 {
		return 0
	}
	return values[min(len(values)-1, len(values)/10)]
}

func ribbonColumnBottom(values []int) int {
	if len(values) == 0 {
		return 0
	}
	return values[min(len(values)-1, len(values)*9/10)]
}

func pointInRibbonBoxes(x int, y int, boxes []image.Rectangle) bool {
	for _, box := range boxes {
		if x >= box.Min.X && x < box.Max.X && y >= box.Min.Y && y < box.Max.Y {
			return true
		}
	}
	return false
}

func interpolateRibbonEdge(values []int, valid []bool, first int, last int) []float64 {
	out := make([]float64, len(values))
	prev := first
	for x := first; x <= last; x++ {
		if valid[x] {
			prev = x
			out[x] = float64(values[x])
			continue
		}
		next := x + 1
		for next <= last && !valid[next] {
			next++
		}
		if next > last {
			out[x] = float64(values[prev])
			continue
		}
		ratio := float64(x-prev) / float64(next-prev)
		out[x] = float64(values[prev])*(1-ratio) + float64(values[next])*ratio
	}
	return out
}

func smoothRibbonEdge(values []float64, first int, last int, radius int) []float64 {
	out := append([]float64(nil), values...)
	for x := first; x <= last; x++ {
		sum := 0.0
		count := 0
		for dx := -radius; dx <= radius; dx++ {
			nx := x + dx
			if nx < first || nx > last {
				continue
			}
			sum += values[nx]
			count++
		}
		if count > 0 {
			out[x] = sum / float64(count)
		}
	}
	return out
}

func sampleRibbonPoints(values []float64, first int, last int, step int) []image.Point {
	points := make([]image.Point, 0, (last-first)/step+2)
	for x := first; x <= last; x += step {
		points = append(points, image.Point{X: x, Y: int(math.Round(values[x]))})
	}
	if points[len(points)-1].X != last {
		points = append(points, image.Point{X: last, Y: int(math.Round(values[last]))})
	}
	return points
}

func pointsToRibbonPath(top []image.Point, bottom []image.Point) string {
	var body strings.Builder
	body.WriteString(fmt.Sprintf("M%d %d", top[0].X, top[0].Y))
	for i := 1; i < len(top); i++ {
		body.WriteString(fmt.Sprintf(" L%d %d", top[i].X, top[i].Y))
	}
	for i := len(bottom) - 1; i >= 0; i-- {
		body.WriteString(fmt.Sprintf(" L%d %d", bottom[i].X, bottom[i].Y))
	}
	body.WriteString(" Z")
	return body.String()
}

func isRibbonTraceLayer(ribbons []ribbonLayer, layer traceLayer) bool {
	for _, ribbon := range ribbons {
		if ribbon.layer.Hex == layer.Hex {
			return true
		}
	}
	return false
}
