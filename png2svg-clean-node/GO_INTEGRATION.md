# Go 工程集成文档

## 推荐集成方式

Go 不重写矢量化算法，只调用本目录的 Node CLI：

```text
Go service
  -> exec node bin/png2svg-clean.mjs
  -> Node CLI 调 magick 预处理
  -> Node CLI 调 @neplex/vectorizer
  -> Node CLI 做 SVG 后处理
  -> Go 读取 output.svg
```

## 目录放置

可以把整个 `png2svg-clean-node` 文件夹复制到 Go 工程，例如：

```text
your-go-project/
  tools/
    png2svg-clean-node/
      package.json
      bin/png2svg-clean.mjs
      profiles/
      README.md
      GO_INTEGRATION.md
  internal/
  cmd/
```

安装依赖：

```bash
cd tools/png2svg-clean-node
npm install
```

服务器上还需要安装 ImageMagick：

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
)

type Result struct {
	SVGPath       string
	OptimizedPNG string
	PreviewPNG   string
}

func ConvertBLSLogo(ctx context.Context, toolDir, inputPNG, outputSVG string) (Result, error) {
	optimizedPNG := outputSVG[:len(outputSVG)-4] + ".optimized.png"
	previewPNG := outputSVG[:len(outputSVG)-4] + ".preview.png"

	cmd := exec.CommandContext(
		ctx,
		"node",
		"bin/png2svg-clean.mjs",
		inputPNG,
		outputSVG,
		"--profile",
		"bls-clean-ribbon",
		"--optimized-png",
		optimizedPNG,
		"--preview",
		previewPNG,
	)
	cmd.Dir = toolDir

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return Result{}, fmt.Errorf("png2svg clean failed: %w: %s", err, stderr.String())
	}

	return Result{
		SVGPath:       outputSVG,
		OptimizedPNG: optimizedPNG,
		PreviewPNG:   previewPNG,
	}, nil
}
```

调用：

```go
result, err := ConvertBLSLogo(
	ctx,
	"tools/png2svg-clean-node",
	"/absolute/path/input.png",
	"/absolute/path/output.svg",
)
```

## Profile 选择

普通 logo：

```text
--profile generic-clean-logo
```

当前 BLS 图：

```text
--profile bls-clean-ribbon
```

## 部署检查清单

上线前确认：

```bash
node -v
magick -version
cd tools/png2svg-clean-node && npm install
node bin/png2svg-clean.mjs sample.png sample.svg --profile generic-clean-logo
```

## 错误处理建议

Go 层建议：

- 给 `exec.CommandContext` 设置超时。
- 捕获 stderr 并写入业务日志。
- 对上传 PNG 做大小限制。
- 对输出 SVG 做存在性和大小检查。
- 如果 profile 是 `bls-clean-ribbon`，只用于同类 BLS 构图，不要泛用到其它 logo。
