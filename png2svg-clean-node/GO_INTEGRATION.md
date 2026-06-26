# Go 工程集成文档

## 推荐方式

Go 工程不重写矢量化算法，直接调用这个 Node CLI：

```text
Go service
  -> exec node bin/png2svg-generic-85.mjs
  -> CLI 调 ImageMagick 做通用预处理
  -> CLI 调 @neplex/vectorizer 生成 SVG
  -> Go 读取 output.svg
```

## 目录放置

建议复制整个目录到 Go 工程：

```text
your-go-project/
  tools/
    png2svg-generic-85/
      package.json
      package-lock.json
      bin/
      profiles/
      README.md
      GO_INTEGRATION.md
```

安装依赖：

```bash
cd tools/png2svg-generic-85
npm install
```

服务器还需要 ImageMagick：

```bash
magick -version
```

## Go 调用示例

```go
package vectorize

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type Result struct {
	SVGPath       string
	OptimizedPNG string
	PreviewPNG   string
}

func ConvertPNGToSVG(ctx context.Context, toolDir, inputPNG, outputSVG string) (Result, error) {
	base := strings.TrimSuffix(outputSVG, ".svg")
	optimizedPNG := base + ".optimized.png"
	previewPNG := base + ".preview.png"

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
		return Result{}, fmt.Errorf("png2svg generic 85 failed: %w: %s", err, stderr.String())
	}

	return Result{
		SVGPath:       outputSVG,
		OptimizedPNG: optimizedPNG,
		PreviewPNG:   previewPNG,
	}, nil
}
```

## 部署检查

```bash
node -v
magick -version
cd tools/png2svg-generic-85 && npm install
node bin/png2svg-generic-85.mjs sample.png sample.svg --preview sample.preview.png
```

## 注意

这是通用 85 分方案，不包含任何针对单张图片的路径补丁。复杂渐变、纹理背景、小字模糊的图片仍然需要源图预处理质量配合。
