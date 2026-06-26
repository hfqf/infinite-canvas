# PNG2SVG Clean Node Kit

这是给 Go 工程调用的 Node CLI 包，用来复用当前已经调好的 PNG 到 SVG 方案。

## 能力

- 使用 ImageMagick 对 PNG 做本地预处理。
- 使用 `@neplex/vectorizer` 做 SVG 矢量化。
- 使用 profile 管理参数。
- 支持 BLS logo 的 clean ribbon 后处理。
- 可输出 SVG、预处理 PNG、渲染预览 PNG。

## 依赖

- Node.js 18+
- ImageMagick 7+，命令名需要是 `magick`

macOS 安装：

```bash
brew install imagemagick
```

安装 Node 依赖：

```bash
cd handoff/png2svg-clean-node
npm install
```

## CLI 用法

通用 logo：

```bash
node bin/png2svg-clean.mjs input.png output.svg \
  --profile generic-clean-logo \
  --optimized-png output.optimized.png \
  --preview output.preview.png
```

BLS clean ribbon：

```bash
node bin/png2svg-clean.mjs input.png output.svg \
  --profile bls-clean-ribbon \
  --optimized-png output.optimized.png \
  --preview output.preview.png
```

输出成功时会打印 JSON：

```json
{
  "profile": "bls-clean-ribbon",
  "input": "input.png",
  "output": "output.svg",
  "optimizedPng": "output.optimized.png",
  "preview": "output.preview.png"
}
```

## Profile 说明

`profiles/generic-clean-logo.json`：

- 适合普通 logo。
- 只做少色预处理和矢量化。
- 不插入 BLS 飘带。

`profiles/bls-clean-ribbon.json`：

- 适合当前 BLS logo 尺寸和构图。
- 会把原图里脏的浅蓝飘带先清掉。
- 再插入一条干净 SVG 飘带 path。

注意：`bls-clean-ribbon` 里的飘带 path 是按 `1672x941` 这张 BLS 图调的。换尺寸或构图时，需要重新调整 path。
