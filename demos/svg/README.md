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

Batch review a folder of real logos:

```bash
go run ./demos/svg --batch /path/to/logo-folder --batch-out /tmp/logo-svg-review
```

Use strict mode for CI or release gates:

```bash
go run ./demos/svg --batch /path/to/logo-folder --batch-out /tmp/logo-svg-review --batch-strict --batch-min-count 10
```

Generate the built-in stress corpus and batch-review it:

```bash
go run ./demos/svg --corpus-out /tmp/svg-logo-corpus
go run ./demos/svg --batch /tmp/svg-logo-corpus --batch-out /tmp/svg-logo-corpus-batch --batch-strict --batch-min-count 10
```

This writes one output folder per image, a `batch-summary.json`, a `batch-report.md`, and a `contact-sheet.png` that stacks every `visual-comparison.png` for quick manual review. The summary includes per-image `quality` checks and top-level `passed` / `warnings` / `failed` / `errored` counts so a real-logo folder can be triaged quickly.

`--batch-strict` exits non-zero when any item has `failed` or `errored` status. Warnings still produce a report for review but do not fail the command.

`--batch-min-count` exits non-zero when fewer than the required number of images were reviewed. Use `--batch-min-count 10` as the minimum gate for real-logo validation, and raise it to `20` before treating a preset as broadly production-proven.

The CLI also prints those quality counts directly:

```text
quality: 7 passed, 2 warnings, 1 failed, 0 errored
```

For real-logo tuning, open `batch-report.md` first. Its Check Breakdown section groups repeated failure/warning types, and its item table lists failed items first, then warnings, then errors/unknowns, then passing items. Start with any `failed` item, inspect its listed checks and actual-value/limit pairs, then open the matching `visual-comparison.png` to decide whether the failure is background drift, color drift, lost text/contour, rough edges, or excess fragments. Use `batch-summary.json` when you need the raw metric values programmatically.

## API

```bash
curl -X POST http://127.0.0.1:8091/api/vectorize \
  -H 'Content-Type: application/json' \
  -d '{
    "filePath": "/path/to/source.png",
    "mode": "cleanLogo",
    "colors": 0
  }'
```

`dataUrl` and `imageUrl` are also supported.

Modes:

- `cleanLogo`: recommended logo preset. It uses the same color-mask/Potrace pipeline as the default lab flow, but applies conservative cleanup parameters and automatic color-count selection for logo artwork.
- `colorMask`: dynamic foreground color extraction, anti-alias edge-layer suppression, mask cleanup, Potrace per functional color layer, then SVG composition. It keeps the source aspect ratio, detects the background from the image border, and uses source-derived colors instead of hard-coded fills.
- `backendLogo`: calls the current backend logo vectorization flow for comparison.
- `layeredRibbon`: failed layered-output experiment kept only for comparison; do not migrate it to production.

## Recommended Logo Preset

Use `mode: "cleanLogo"` and leave `colors` as `0` for automatic selection:

```json
{
  "mode": "cleanLogo",
  "colors": 0,
  "longEdge": 4096,
  "minComponentRatio": 0.00005,
  "maxHoleRatio": 0.00008,
  "mergeDistance": 64,
  "mergeHueDistance": 16,
  "mergeLightness": 80,
  "mergeSaturation": 0.55,
  "lightMinAreaRatio": 0.004,
  "maskCloseRadius": 0,
  "lightDilateRadius": 0,
  "darkDilateRadius": 0
}
```

Automatic color selection first quantizes a small preview to detect meaningful brand-color hue groups:

- one hue group: 3 colors, for compact single-color logos without meaningful tonal accents
- one hue group with meaningful light same-hue accents, pale same-hue tinted ribbons, or tonal gradients: 16 colors, to keep highlight, ribbon, and gradient steps from drifting
- two hue groups: 8 colors, for two-color logos
- colored brand layer with a small dark-neutral real-font text layer: 12 colors, to keep anti-aliased black text clearer
- three or more hue groups: 12 colors, for multi-color logos
- dark-background logos with small light-neutral details: 32 colors, to preserve light text and strokes

Manual `colors` values still work when you need to force a specific palette size during debugging.

For dark-background logos, lower-luma light-neutral antialias pixels are restored before tracing so white or gray text edges stay neutral instead of drifting into nearby blue/cyan brand layers.

The SVG Lab defaults, production `cleanLogo` constants, and the preset above are intentionally kept in sync. `main_test.go` includes a guard test so local H5 experiments and backend `/api/v1/images/vectorize` use the same tuned values. The production vectorize API returns both `engine` and the effective `preset` for logo modes, so online SVG nodes can be checked against the exact parameters used to generate them.

## Current Validation

The regression tests in `main_test.go` run the preset against three logo-like fixtures under `demos/svg/fixtures/`:

- BLS same-hue logo with a light ribbon accent: auto-selects 16 colors, stays under 3 SVG paths / 80 subpaths, keeps the teal foreground color at `0` foreground color delta, and preserves the light accent without fragmenting.
- Red mark on red background: auto-selects 3 colors, stays under 2 SVG paths / 12 subpaths, and keeps the red fill.
- GJ multi-color logo: auto-selects 12 colors, stays under 6 SVG paths / 48 subpaths, and keeps blue, red, and orange fills.
- Runtime stress samples in the Lab tests cover dark-background light strokes, multi-color block/text cases, monochrome thin-line icons, transparent-background logos, speckled/noisy logos, blue-plus-dark-micro-text logos, real-font small-text logos, long real-font tagline logos, serif logistic/service logos with pale same-hue ribbons, low-contrast gray tagline logos, CJK real-font text logos, flat CJK slogan banners with pale illustration layers, dark-background real-font white-text logos, ring/diagonal logos, soft neutral shadow logos, JPEG-compressed logos, thin colored outline logos, tinted-background logos, knockout micro-hole logos, white text on brand-color block logos, semi-transparent shadow logos, narrow negative-space logos, legitimate small-mark logos, edge-aligned logos, same-hue gradient logos, and white-text off-white logos without adding more binary fixtures.

For flat banner-like artwork, large pale tinted illustration layers are protected from background merging when their area is meaningful and their color is visibly different from the detected background. This keeps scenic/ribbon layers and large CJK slogan text from disappearing while still allowing tiny antialias/background companion pixels to merge cleanly.

The built-in corpus currently writes 10 stress images covering pale same-hue ribbons, multicolor block text, dense micro text, serif service logos, flat CJK slogan banners, dark-background white text, white knockout text on brand blocks, same-hue gradients, narrow gaps, and legitimate small marks. The latest local corpus batch run returned:

```text
quality: 10 passed, 0 warnings, 0 failed, 0 errored
```

`cleanLogo` is tuned for compact logos and marks. Large illustrated banners or dense decorative strips should get a separate banner/illustration preset instead of relaxing the logo cleanup thresholds.

The Lab metrics also include:

- `backgroundDelta`: source border background vs. SVG background fill RGB distance. Logo regression tests require this to stay at `0`.
- `sourceForegroundColor`, `renderedForegroundColor`, and `foregroundColorDelta`: dominant foreground color drift between source and rendered SVG, used as an automatic brand-color preservation check for batch review.
- `sourceForegroundCoverage`, `renderedForegroundCoverage`, and `foregroundCoverageDelta`: foreground coverage drift between source and rendered SVG.
- `renderedRMSE`: ImageMagick RMSE between the normalized source and same-size rendered SVG.

Batch quality checks use these metrics directly:

- `backgroundDelta` fails above `0`, because the main background should not change.
- `foregroundColorDelta` warns above `14` and fails above `24`, catching obvious brand-color or white-mark drift in real-logo batches.
- `foregroundCoverageDelta` warns above `0.03` and fails above `0.04`, catching missing text, bloated strokes, or contour loss.
- `renderedRMSE` warns above `0.04` for ordinary logo cases, above `0.045` for 16-color light-accent/tonal cases, and above `0.05` for high-palette dark-background cases. It fails above `0.05` for ordinary and 16-color cases, or `0.055` for high-palette dark-background cases.
- `svgPaths` and `svgSubpaths` warn/fail when output becomes too fragmented; high-palette dark-background cases get a looser ceiling because preserving white text can legitimately require more subpaths.

Each `cleanLogo` run also writes and displays `visual-comparison.png`, a left-to-right visual sheet of normalized source, rendered SVG, and amplified pixel difference. Use it for quick manual review of text clarity, contour drift, edge cleanliness, and unexpected fragments.

The H5 page renders these values as a Quality Check panel, shows the visual comparison image inline, and lists output artifacts with `visual-comparison.png` first. Slider changes start a new SVG refresh immediately when the lab is idle; when a run is already processing, the page queues only the latest parameter set and prevents stale output from replacing the current preview. For 16-color light-accent cases, the RMSE warning line is relaxed to `0.045`; for dark-background/high-palette cases (`colors >= 32`), RMSE and path/subpath warning thresholds are relaxed to match the stress-test ceiling.

Run:

```bash
go test ./demos/svg -v
```

This preset is for logos and simple brand marks. Large illustrative banners, textured photos, or poster-like compositions need a separate preset; forcing this logo preset onto those images can over-clean background illustration detail.

## Output

Each run creates one folder under `demos/svg/outputs/{jobId}/`:

- `source.png`
- `normalized.png`
- `clustered.png` or `quantized.png`
- `color-xx-mask.png` / `layer-xx-mask.png`
- `color-xx.svg` / `layer-xx.svg`
- `output.svg`
- `preview.png`
- `rendered-metrics.png`
- `visual-diff.png`
- `visual-comparison.png`
- `metrics.json`

The important debugging files are `visual-comparison.png`, the layer masks, and `metrics.json`. If holes or speckles appear in `output.svg`, first inspect the matching `layer-xx-mask.png`.
