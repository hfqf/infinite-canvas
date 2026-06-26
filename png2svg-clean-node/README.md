# PNG2SVG Generic 85

这是 85 分通用版 PNG 转 SVG 方案，适合迁移到 Go 工程里通过命令行调用。

这个版本只包含通用处理：

- ImageMagick 少色预处理
- `@neplex/vectorizer` 通用矢量化参数
- SVG safe optimize

它不包含任何单图专用修复，例如 BLS 飘带 path 注入。

## 依赖

- Node.js 18+
- ImageMagick 7+，命令名需要是 `magick`

安装：

```bash
cd handoff/png2svg-generic-85
npm install
```

## CLI 用法

```bash
node bin/png2svg-generic-85.mjs input.png output.svg \
  --optimized-png output.optimized.png \
  --preview output.preview.png
```

成功时会打印 JSON：

```json
{
  "profile": "generic-85",
  "input": "input.png",
  "output": "output.svg",
  "optimizedPng": "output.optimized.png",
  "preview": "output.preview.png"
}
```

## 85 分参数

预处理：

```bash
magick input.png \
  -alpha off \
  -fuzz 2% \
  -fill white \
  -opaque white \
  -colors 8 \
  -dither none \
  output.optimized.png
```

矢量化参数见：

```text
profiles/generic-85.json
```

## 适用边界

适合：

- logo
- 图标
- 扁平色块插画
- 背景简单、颜色数量少的 PNG

不适合：

- 照片
- 复杂渐变
- 大量纹理
- 原图文字已经模糊或错误的图片
