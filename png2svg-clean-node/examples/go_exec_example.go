package main

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		panic("cannot locate example file")
	}

	toolDir := filepath.Dir(filepath.Dir(currentFile))
	repoDir := filepath.Dir(filepath.Dir(toolDir))
	inputPNG := filepath.Join(repoDir, "pngs", "bdf825e7c7e63eb5-canvas-image.png")
	outputSVG := filepath.Join(repoDir, "outputs", "go-generic-85-example", "result.svg")
	optimizedPNG := filepath.Join(repoDir, "outputs", "go-generic-85-example", "result.optimized.png")
	previewPNG := filepath.Join(repoDir, "outputs", "go-generic-85-example", "result.preview.png")

	cmd := exec.CommandContext(
		ctx,
		"node",
		"bin/png2svg-generic-85.mjs",
		inputPNG,
		outputSVG,
		"--optimized-png",
		optimizedPNG,
		"--preview",
		previewPNG,
	)
	cmd.Dir = toolDir

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		panic(fmt.Errorf("convert failed: %w: %s", err, stderr.String()))
	}

	fmt.Println("SVG written:", outputSVG)
}
