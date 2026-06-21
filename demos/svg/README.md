# PNG to SVG Lab

Local experiment service for testing PNG to SVG vectorization without touching the canvas production flow.

## Start

```bash
PORT=8091 go run ./demos/svg
```

Open:

```text
http://127.0.0.1:8091
```

## API

```bash
curl -X POST http://127.0.0.1:8091/api/vectorize \
  -H 'Content-Type: application/json' \
  -d '{
    "filePath": "/path/to/source.png",
    "mode": "colorMask",
    "colors": 8,
    "longEdge": 4096,
    "mergeDistance": 64,
    "minComponentRatio": 0.00001,
    "maxHoleRatio": 0.00004
  }'
```

`dataUrl` and `imageUrl` are also supported.

Modes:

- `colorMask`: dynamic foreground color extraction, anti-alias edge-layer suppression, mask cleanup, Potrace per functional color layer, then SVG composition. It keeps the source aspect ratio, detects the background from the image border, and uses source-derived colors instead of hard-coded fills.
- `backendLogo`: calls the current backend logo vectorization flow for comparison.
- `layeredRibbon`: failed layered-output experiment kept only for comparison; do not migrate it to production.

## Output

Each run creates one folder under `demos/svg/outputs/{jobId}/`:

- `source.png`
- `normalized.png`
- `clustered.png` or `quantized.png`
- `color-xx-mask.png` / `layer-xx-mask.png`
- `color-xx.svg` / `layer-xx.svg`
- `output.svg`
- `preview.png`
- `metrics.json`

The important debugging files are the layer masks and `metrics.json`. If holes or speckles appear in `output.svg`, first inspect the matching `layer-xx-mask.png`.
