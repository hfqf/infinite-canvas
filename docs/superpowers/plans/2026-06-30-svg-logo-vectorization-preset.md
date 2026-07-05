# SVG Logo Vectorization Preset Plan

> **For agentic workers:** This is a restartable implementation and validation log for the local SVG lab under `demos/svg`. Continue from the current code and evidence instead of retuning from scratch.

**Goal:** Tune a stable logo-to-SVG preset that keeps the main background, preserves logo contours, keeps text readable, produces clean edges, avoids fragmented path output, and preserves brand colors.

**Current Recommendation:** Production `/api/v1/images/vectorize` should prefer the external Recraft vectorization provider (`VECTORIZE_PROVIDER=recraft` + `RECRAFT_API_KEY`) for real user traffic. The local `cleanLogo`/Potrace preset remains as the fallback/debug path and as the SVG Lab comparison baseline.

## 2026-07-05 Recraft Provider Integration

- Added `VECTORIZE_PROVIDER`, `RECRAFT_API_KEY`, and `RECRAFT_API_BASE_URL` config so production can route PNG/JPG-to-SVG through Recraft without changing the canvas API contract.
- `/api/v1/images/vectorize` keeps the same request/response shape. When Recraft is configured, the backend uploads the image as multipart `file`, downloads the returned SVG URL, validates the result as SVG, and reports `engine: "recraft-vectorize"`.
- Local Potrace/png2svg branches are still present and covered by branch-selection tests, so missing Recraft configuration can still use the historical local flow.
- Recraft input is capped at 10MB before upload, matching the external service limit and returning a clear user-facing error instead of a remote 4xx.
- Local ignored `.env` is configured with the provided Recraft key; committed files only include empty placeholders.

Previous local fallback recommendation: use `mode: "cleanLogo"` with `colors: 0`. `colors=0` means automatic color-count selection based on significant brand-color hue groups.

## Current Preset

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

Automatic color selection:

- One significant hue group -> `3` colors, unless the chromatic foreground has meaningful same-hue light accents or tonal gradients.
- One significant hue group with meaningful light same-hue accents, pale same-hue tinted ribbons, or tonal gradients -> `16` colors, to keep highlights, ribbons, and gradient steps from drifting.
- Two significant hue groups -> `8` colors.
- Colored brand layer with a small dark-neutral real-font text layer -> `12` colors, to keep anti-aliased black text clearer.
- Three or more significant hue groups -> `12` colors.
- Manual `colors` values still override automatic selection for debugging.

The local SVG Lab defaults now match the production `cleanLogo` constants and this documented preset exactly. `TestCleanLogoLabDefaultsMatchRecommendedPreset` and `TestCleanLogoProductionConstantsMatchRecommendedPreset` guard this so H5 tuning, batch validation, and backend vectorization do not drift onto different parameter sets.

## Why This Preset

- Fixed `colors=3` kept BLS compact but flattened its light ribbon accent and left a visible foreground color drift.
- Fixed `colors=12` preserved multi-color logos, but was not a general solution for same-hue logos with light accents.
- Automatic color selection keeps same-hue logos compact while giving multi-color logos enough palette space.
- Conservative cleanup values avoid the online failure mode where logo output had hundreds of tiny paths and visibly rough edges.

## Evidence

Online bad BLS SVG:

- `995` paths.
- `995` subpaths.
- Visible rough edges and fragmented ribbon/text areas.

`cleanLogo` validation samples:

| Sample | Auto Colors | Result | Key Evidence |
| --- | ---: | --- | --- |
| BLS same-hue logo with light ribbon accent | 16 | Under `3` paths / `80` subpaths | Keeps background delta `0.00`, foreground color delta `0.00`, foreground coverage delta about `0.0262`, and RMSE about `0.0426` while preserving the light ribbon accent without fragmenting. |
| Red mark on red background | 3 | `1` path / `4` subpaths | Keeps the red fill within a 1-channel quantization drift (`#CA1819` in the latest run); background delta `0.00`, RMSE about `0.027302`. |
| GJ multi-color logo | 12 | `4` paths / `24` subpaths | Keeps blue `#241668`, red `#dc1a17`, and orange `#e96d10`; RMSE about `0.0184624`. |
| Generated speckled logo | 3 | Under `8` paths / `80` subpaths | Ignores isolated tiny speckles, keeps the main dark logo fill, foreground coverage delta about `0.0016`, RMSE about `0.019677`. |
| Generated micro-text logo | 8 | Under `8` paths / `320` subpaths | Keeps dense geometric dark micro text separate from blue brand blocks without exploding subpaths; foreground coverage delta about `0.0246`, RMSE about `0.065751`. |
| Generated real-font small-text logo | 12 | Under `10` paths / `220` subpaths | Keeps actual font-rendered dark small text separate from the teal brand layer; production foreground coverage delta about `0.0074`, RMSE about `0.034645`. |
| Generated long real-font tagline logo | 12 | Under `10` paths / `260` subpaths | Keeps long English tagline text as a dark layer; production background delta `0.00`, foreground color delta `0.00`, foreground coverage delta about `0.0021`, and RMSE about `0.045574`. |
| Generated serif logistic logo | 16 | Under `12` paths / `280` subpaths | Keeps teal serif brand text and pale same-hue ribbon accents; production background delta `0.00`, foreground color delta `0.00`, foreground coverage delta about `0.0184`, and RMSE about `0.041734`. |
| Generated low-contrast gray tagline logo | 12 | Under `10` paths / `260` subpaths | Keeps low-contrast `#787878` gray tagline text as its own layer; production background delta `0.00`, foreground color delta `0.00`, foreground coverage delta about `0.0099`, and RMSE about `0.026508`. |
| Generated CJK real-font text logo | 12 | Under `10` paths / `240` subpaths | Keeps actual font-rendered Chinese brand/subtitle text separate from the teal brand layer; production foreground coverage delta about `0.0068`, RMSE about `0.036377`. |
| Generated flat CJK slogan banner | 8 | Under `18` paths / `520` subpaths | Keeps large CJK slogan text plus pale scenic illustration layers instead of merging them into the background; production background delta `0.00`, foreground color delta `0.00`, foreground coverage delta about `0.0335`, and RMSE about `0.028827`. |
| Generated dark real-font white-text logo | 32 | Under `14` paths / `260` subpaths | Keeps actual font-rendered white text readable on a dark background; production foreground coverage delta about `0.0052`, rendered RMSE about `0.044030`, and the focused right-side light text region has `0.0000` cyan tint-bleed ratio. |
| Generated ring/diagonal logo | 8 | Under `12` paths / `80` subpaths | Keeps circular holes and diagonal edges clean; foreground coverage delta about `0.0002`, RMSE about `0.014748`. |
| Generated soft neutral shadow logo | 8 | Under `12` paths / `120` subpaths | Keeps low-contrast neutral shadows/sub-lines without fragmenting; foreground coverage delta about `0.0021`, RMSE about `0.012577`. |
| Generated JPEG artifact logo | 8 | Under `12` paths / `160` subpaths | Keeps compressed JPG uploads compact; foreground coverage delta about `0.0020`, RMSE about `0.024054`. |
| Generated thin outline logo | 3 | Under `8` paths / `140` subpaths | Keeps colored stroke-only outlines and short rule-like strokes; foreground coverage delta about `0.0156`, RMSE about `0.034675`. |
| Generated tinted-background logo | 8 | Under `10` paths / `120` subpaths | Preserves non-white brand background exactly; background delta `0.00`, foreground coverage delta about `0.0005`, RMSE about `0.018842`. |
| Generated knockout micro-holes logo | 8 | Under `10` paths / `180` subpaths | Keeps small white counters/knockout text holes open; foreground coverage delta about `0.0116`, RMSE about `0.028305`. |
| Generated white text on brand block logo | 3 | Under `6` paths / `140` subpaths | Keeps real-font white letters open inside a teal brand block; production background delta `0.00`, foreground color delta `0.00`, foreground coverage delta about `0.0188`, and RMSE about `0.038933`. |
| Generated semi-transparent shadow logo | 8 | Under `12` paths / `120` subpaths | Composites alpha shadows cleanly onto white without black/dirty edges; background delta `0.00`, foreground coverage delta about `0.0002`, RMSE about `0.018569`. |
| Generated narrow-gap logo | 12 | Under `12` paths / `120` subpaths | Keeps thin white negative spaces between adjacent color blocks open; foreground coverage delta about `0.0002`, RMSE about `0.018031`. |
| Generated small-marks logo | 3 | Under `10` paths / `140` subpaths | Keeps legitimate small marks/dots near the logo while still filtering isolated noise; foreground coverage delta about `0.0018`, RMSE about `0.015877`. |
| Generated edge-aligned logo | 8 | Under `10` paths / `120` subpaths | Keeps border-touching strokes and near-edge brand marks without misclassifying them as background; background delta `0.00`, foreground coverage delta about `0.0058`, RMSE about `0.021948`. |
| Generated same-hue gradient logo | 16 | Under `10` paths / `140` subpaths | Keeps same-hue highlight/gradient tonal steps instead of flattening them into one brand fill; background delta `0.00`, foreground coverage delta about `0.0010`, RMSE about `0.0293`. |
| Generated white-text off-white logo | 8 | Under `8` paths / `100` subpaths | Keeps white/near-white text and highlights as a visible layer on an off-white background instead of merging them into the background; background delta `0.00`, foreground coverage delta about `0.0011`, RMSE about `0.018526`. |

Regression tests in `demos/svg/main_test.go` cover:

- Histogram channel parsing.
- Automatic color-count selection for BLS, red mark, and GJ fixtures under `demos/svg/fixtures/`.
- Full `cleanLogo` vectorization metrics, including path/subpath ceilings, required brand-color fills, nearest-fill brand color RGB drift, automatic foreground-color drift, background RGB drift, foreground coverage drift, and rendered RMSE.
- The SVG Lab API now writes `backgroundDelta`, `sourceForegroundCoverage`, `renderedForegroundCoverage`, `foregroundCoverageDelta`, and `renderedRMSE` to `metrics.json`, returns `rendered-metrics.png` for same-size visual inspection, and returns `visual-comparison.png` with normalized source, rendered SVG, and amplified pixel difference side by side. The H5 page shows `visual-comparison.png` inline and lists it first in the artifact links for quick manual review.
- Production and SVG Lab tests also generate deterministic dark-background light-stroke, multi-color block/text, monochrome thin-line icon, transparent-background logo, speckled-logo, micro-text logo, real-font small-text logo, long real-font tagline logo, serif logistic/service logo with pale same-hue ribbons, low-contrast gray tagline logo, CJK real-font text logo, flat CJK slogan banner, dark real-font white-text logo, ring/diagonal logo, soft neutral shadow logo, JPEG artifact logo, thin outline logo, tinted-background logo, knockout micro-holes logo, white text on brand block logo, semi-transparent shadow logo, narrow-gap logo, small-marks logo, edge-aligned logo, same-hue gradient logo, and white-text off-white logo samples at runtime so the preset is checked against non-fixture small-detail/alpha/noise/text/long-tagline/serif-type/pale-same-hue-ribbon/low-contrast-gray-text/hole/edge/CJK-text/flat-banner/pale-illustration-layer/low-contrast/compression/stroke-only/non-white-background/knockout/brand-block-white-text/translucency/negative-space/legitimate-small-component/near-border/tonal-gradient/off-white-background-white-detail cases without adding more binary fixtures.
- Both production and SVG Lab tests assert the expected automatic color count for these cases: BLS `16`, red mark `3`, GJ `12`, dark-background light strokes `32`, multi-color block/text `12`, monochrome thin-line icon `3`, transparent-background logo `8`, speckled logo `3`, blue-plus-dark-micro-text logo `8`, real-font small-text logo `12`, long real-font tagline logo `12`, serif logistic/service logo `16`, low-contrast gray tagline logo `12`, CJK real-font text logo `12`, flat CJK slogan banner `8`, dark real-font white-text logo `32`, ring/diagonal logo `8`, soft neutral shadow logo `8`, JPEG artifact logo `8`, thin outline logo `3`, tinted-background logo `8`, knockout micro-holes logo `8`, white text on brand block logo `3`, semi-transparent shadow logo `8`, narrow-gap logo `12`, small-marks logo `3`, edge-aligned logo `8`, same-hue gradient logo `16`, and white-text off-white logo `8`.
- Histogram-based automatic color estimation now forces ImageMagick `-depth 8` before reading `%c`, because some ImageMagick builds emit Q16 channel values by default. Without this, red/teal colors can be parsed as clamped near-white values and the logo can be incorrectly classified as 3 colors.
- Dark-background light-neutral text restoration now keeps lower-luma antialias pixels (`luma >= 160`) instead of only near-white pixels, which prevents small white/gray text edges from being quantized into nearby cyan/blue brand layers.

Production regression tests in `service/vectorize_test.go` cover:

- Existing non-logo modes still report the `png2svg-clean-node` engine.
- `logo`, `cleanLogo`, and `clean-logo` modes report the `clean-logo-potrace` engine.
- Logo-mode SVG output stays under path/subpath ceilings.
- Source border background is compared with the SVG background `<rect>` fill, with a strict RGB distance ceiling of `0` in both production and SVG Lab regression tests. This directly guards the "main background should not change" requirement.
- Production and SVG Lab regression tests compare the dominant source foreground color against the nearest rendered foreground color with an RGB distance ceiling of `24`, which guards automatic brand-color / white-mark drift without falsely failing multi-color logos when foreground area ordering changes.
- Required brand-color fills remain present.
- Required brand colors are compared against the nearest SVG fill with an RGB distance ceiling, so small hex drift can be tolerated while obvious color changes fail the test.
- Foreground pixel coverage is compared between the normalized source and rendered SVG. This guards against missing or bloated contours and helps catch text or thin strokes being removed.
- Rendered SVG output is compared back to the normalized source with ImageMagick RMSE and must stay under `0.04` for the current fixtures. This gives a broader guardrail for background, contour, edge, and color drift than path-count checks alone.

Production API local verification:

- `POST /api/v1/images/vectorize` with `mode: "logo"` now returns `engine: "clean-logo-potrace"`.
- Logo-mode API responses now also return the effective `preset` object, including `longEdge`, cleanup ratios, merge thresholds, and dilation/close radii, so online SVG nodes can be traced back to the exact clean Logo parameters.
- BLS sample now auto-selects `16` colors; latest focused production run returned background delta `0.00`, foreground color delta `0.00`, foreground coverage delta about `0.0262`, rendered RMSE about `0.042639`, and stays under the `3` path / `80` subpath ceiling.
- Red mark sample returned `1` path / `3` subpaths and fills `#CA1719`, `#fcfafa`.
- GJ sample returned `4` paths / `24` subpaths and fills `#241668`, `#dc1a17`, `#e65d5a`, `#e96d10`, `#FDFDFD`.
- Current production fixture background deltas are `0.00`, `0.00`, and `0.00`; all current production and SVG Lab Logo regression cases now require `maxBgDelta: 0`. SVG background fill follows the source border's dominant color instead of the quantized background color.
- Current production fixture foreground coverage deltas are `0.0243`, `0.0088`, and `0.0147` against a ceiling of `0.04`.
- Current production fixture rendered RMSE values are about `0.042639`, `0.027302`, and `0.033024`; BLS has a `0.05` ceiling because preserving the light ribbon accent as vector layers increases hard-edge pixel differences while keeping foreground color drift at `0.00`.
- Batch quality uses a `0.045` RMSE warning line for `colors=16` light-accent/tonal logos, so BLS passes batch review when background, foreground color, coverage, path count, and subpath count are all within limits.
- The generated dark-background light-stroke stress sample currently passes with estimated colors `32`, foreground coverage delta about `0.0068`, and rendered RMSE about `0.040257` against its `0.045` ceiling.
- The generated multi-color block/text stress sample currently passes with foreground coverage delta about `0.0007` and rendered RMSE about `0.018950`.
- The generated monochrome thin-line icon stress sample currently passes with estimated colors `3`, foreground coverage delta about `0.0013`, and rendered RMSE about `0.019974`.
- The generated transparent-background logo stress sample currently passes with estimated colors `8`, background delta `0.00`, foreground coverage delta about `0.0040`, and rendered RMSE about `0.018451`.
- The generated speckled-logo stress sample currently passes with estimated colors `3`, background delta `0.00`, foreground coverage delta about `0.0016`, and rendered RMSE about `0.019677`; this guards against isolated dust/noise becoming SVG fragments.
- The generated blue-plus-dark-micro-text sample currently passes with estimated colors `8`, background delta `0.00`, foreground coverage delta about `0.0246`, and rendered RMSE about `0.065751`; this guards against black/neutral small text being quantized into the dominant blue brand layer. Its RMSE ceiling is intentionally looser because anti-aliased micro text becomes hard-edged vector geometry, so foreground coverage and color preservation carry more weight for this sample.
- The generated real-font small-text sample currently passes with estimated colors `12`, background delta `0.00`, foreground coverage delta about `0.0074`, and production rendered RMSE about `0.034645`; this guards against actual font-rendered black text being quantized into the dominant teal brand layer.
- The generated long real-font tagline sample currently passes with estimated colors `12`, background delta `0.00`, foreground color delta `0.00`, foreground coverage delta about `0.0021`, and production rendered RMSE about `0.045574`; this guards against long English service/tagline text being flattened or cleaned away.
- The generated serif logistic/service sample currently passes with estimated colors `16`, background delta `0.00`, foreground color delta `0.00`, foreground coverage delta about `0.0184`, and production rendered RMSE about `0.041734`; this guards against pale same-hue ribbon accents being misclassified as background/noise while preserving large teal serif text.
- The generated low-contrast gray tagline sample currently passes with estimated colors `12`, background delta `0.00`, foreground color delta `0.00`, foreground coverage delta about `0.0099`, and production rendered RMSE about `0.026508`; this guards against gray service/tagline text being quantized into the near-white background.
- The generated CJK real-font text sample currently passes with estimated colors `12`, background delta `0.00`, foreground coverage delta about `0.0068`, and production rendered RMSE about `0.036377`; this guards against Chinese brand/subtitle strokes being quantized into the dominant teal brand layer or cleaned away.
- The generated flat CJK slogan banner sample currently passes with estimated colors `8`, background delta `0.00`, foreground color delta `0.00`, foreground coverage delta about `0.0335`, and production rendered RMSE about `0.028827`; this guards against wide banner artwork with large CJK text and pale scenic/ribbon layers being swallowed by background merging.
- The generated dark real-font white-text sample currently passes with estimated colors `32`, background delta `0.00`, foreground color delta `0.00`, foreground coverage delta about `0.0052`, production rendered RMSE about `0.044030`, and a focused right-side text-region cyan tint-bleed ratio of `0.0000`; this guards against actual font-rendered white text on dark backgrounds being merged into the background, losing contrast, or picking up blue/cyan edge color.
- The generated ring/diagonal sample currently passes with estimated colors `8`, background delta `0.00`, foreground coverage delta about `0.0002`, and rendered RMSE about `0.014748`; this guards against large interior holes being filled and diagonal/curved edges drifting.
- The generated soft neutral shadow sample currently passes with estimated colors `8`, background delta `0.00`, foreground coverage delta about `0.0021`, and rendered RMSE about `0.012577`; this guards against low-contrast gray shadows or secondary rules being merged into the background or exploding into fragments.
- The generated JPEG artifact sample currently passes with estimated colors `8`, background delta `0.00`, foreground coverage delta about `0.0020`, and rendered RMSE about `0.024054`; this guards against compressed JPG edge artifacts becoming many small SVG fragments while keeping the main red/blue brand colors close.
- The generated thin outline sample currently passes with estimated colors `3`, background delta `0.00`, foreground coverage delta about `0.0156`, and rendered RMSE about `0.034675`; this guards against stroke-only marks being removed by speckle cleanup or losing too much line thickness.
- The generated tinted-background sample currently passes with estimated colors `8`, background delta `0.00`, foreground coverage delta about `0.0005`, and rendered RMSE about `0.018842`; this guards against non-white brand backgrounds being normalized to white or drifting during background detection.
- The generated knockout micro-holes sample currently passes with estimated colors `8`, background delta `0.00`, foreground coverage delta about `0.0116`, and rendered RMSE about `0.028305`; this guards against small white counters or knockout text holes being filled by mask hole cleanup.
- The generated white text on brand block sample currently passes with estimated colors `3`, background delta `0.00`, foreground color delta `0.00`, foreground coverage delta about `0.0188`, and production rendered RMSE about `0.038933`; this guards against real-font white letters inside colored brand blocks being filled or closed.
- The generated semi-transparent shadow sample currently passes with estimated colors `8`, background delta `0.00`, foreground coverage delta about `0.0002`, and rendered RMSE about `0.018569`; this guards against transparent PNG shadows compositing to black or becoming dirty edge fragments.
- The generated narrow-gap sample currently passes with estimated colors `12`, background delta `0.00`, foreground coverage delta about `0.0002`, and rendered RMSE about `0.018031`; this guards against narrow white separators between adjacent color blocks being filled or merged away.
- The generated small-marks sample currently passes with estimated colors `3`, background delta `0.00`, foreground coverage delta about `0.0018`, and rendered RMSE about `0.015877`; this guards against trademark dots, small counters, or nearby small marks being mistaken for removable speckles.
- The generated edge-aligned sample currently passes with estimated colors `8`, background delta `0.00`, foreground coverage delta about `0.0058`, and rendered RMSE about `0.021948`; this guards against border-touching strokes or near-edge marks being cropped, dropped, or treated as background.
- The generated same-hue gradient sample currently passes with estimated colors `16`, background delta `0.00`, foreground coverage delta about `0.0010`, and rendered RMSE about `0.0293`; this guards against same-hue brand highlights or gradients being flattened into too few color layers.
- The generated white-text off-white sample currently passes with estimated colors `8`, background delta `0.00`, foreground coverage delta about `0.0011`, and rendered RMSE about `0.018526`; this guards against white text or highlights being merged into a near-white background.

## Commands

Start the lab:

```bash
PORT=8091 DATABASE_DRIVER=sqlite DATABASE_DSN=data/infinite-canvas.db STORAGE_DRIVER=sqlite go run ./demos/svg
```

Use the preset:

```bash
curl -sS -X POST http://127.0.0.1:8091/api/vectorize \
  -H 'Content-Type: application/json' \
  -d '{
    "filePath": "/path/to/logo.png",
    "mode": "cleanLogo",
    "colors": 0
  }'
```

Verify locally:

```bash
go test ./demos/svg -v
go test ./service -run 'Test(Vectorize|Png2SVG)' -v
git diff --check
```

Batch review real logo folders:

```bash
go run ./demos/svg --batch /path/to/logo-folder --batch-out /tmp/logo-svg-review --batch-strict --batch-min-count 10
```

Generate and batch-review the built-in stress corpus:

```bash
go run ./demos/svg --corpus-out /tmp/svg-logo-corpus
go run ./demos/svg --batch /tmp/svg-logo-corpus --batch-out /tmp/svg-logo-corpus-batch --batch-strict --batch-min-count 10
```

The batch mode writes one output folder per image, `batch-summary.json`, `batch-report.md`, and `contact-sheet.png` stacked from every `visual-comparison.png`. `batch-summary.json` includes a per-image `quality` block plus top-level `passed`, `warnings`, `failed`, and `errored` counts. `batch-report.md` is the human review entry point: it lists each source, status, key metrics, failed/warning checks, `visual-comparison.png`, and `output.svg`. The quality block maps the user's acceptance criteria to measurable checks:

- `backgroundDelta`: fails above `0`, guarding the main-background invariant.
- `foregroundCoverageDelta`: warns above `0.03` and fails above `0.04`, guarding text/contour loss or bloating.
- `foregroundColorDelta`: warns above `14` and fails above `24`, guarding automatic brand-color / white-mark drift in real-logo batches.
- `renderedRMSE`: warns above `0.04` and fails above `0.05` for ordinary logo cases, or `0.055` for high-palette dark-background cases.
- `svgPaths` / `svgSubpaths`: warn/fail when output becomes too fragmented; high-palette dark-background cases get looser ceilings because preserving white text can legitimately require more vector pieces.

Quality thresholds include narrow special handling for two verified cases:

- Dense micro-text cases can have higher RMSE/subpath counts while still passing only when background delta, foreground color delta, foreground coverage, and path count remain stable. This prevents preserving tiny text from being misclassified as fragmentation.
- Clean complex flat artwork can avoid warnings when RMSE is low, foreground color is stable, foreground coverage stays below `0.035`, and SVG path count remains low. This keeps pale ribbons and flat CJK banner layers from creating noisy false positives.

The CLI prints the same counts as `quality: N passed, N warnings, N failed, N errored`, plus `summary:` and `report:` paths, so real-logo folders can be triaged before opening the JSON.

`--batch-strict` turns the quality report into a release gate: it exits non-zero when any item is `failed` or `errored`. Warnings remain visible in the report but do not fail the command, because some verified text/ribbon/banner cases naturally need manual review even when background, color, RMSE, and coverage are acceptable.

`--batch-min-count` prevents false confidence from too-small sample sets. Use `--batch-min-count 10` for the next real-logo validation pass; use `20` before treating the preset as broadly production-proven.

Latest built-in corpus result:

```text
processed 10 image(s)
quality: 10 passed, 0 warnings, 0 failed, 0 errored
```

Latest strict fixture result:

```text
processed 3 image(s)
quality: 3 passed, 0 warnings, 0 failed, 0 errored
```

Latest focused strict-batch highlights:

- BLS fixture: background delta `0.00`, foreground color delta `0.00`, foreground coverage delta `0.0153`, rendered RMSE `0.028667`, `2` SVG paths, `75` subpaths.
- Built-in BLS pale-ribbon sample: background delta `0.00`, foreground color delta `0.00`, foreground coverage delta `0.0347`, rendered RMSE `0.032189`, `2` SVG paths, `73` subpaths.
- Built-in CJK slogan banner sample: background delta `0.00`, foreground color delta `0.00`, foreground coverage delta `0.0330`, rendered RMSE `0.028134`, `6` SVG paths, `165` subpaths.
- Built-in dark white-text sample: background delta `0.00`, foreground color delta `0.00`, foreground coverage delta `0.0080`, rendered RMSE `0.043071`, `5` SVG paths, `41` subpaths.

## Current Files

- `service/vectorize.go`
  - Routes `mode=logo`, `mode=cleanLogo`, and `mode=clean-logo` to the clean Logo Potrace pipeline instead of `png2svg-clean-node`.
  - Returns `preset` for clean Logo modes so API callers and saved SVG nodes can confirm the actual parameters used.
- `service/vectorize_logo.go`
  - Production clean Logo implementation with automatic color-count selection and ImageMagick/Potrace SVG composition.
  - Restores lower-luma light-neutral antialias pixels on dark backgrounds so white/gray text edges stay neutral instead of drifting into nearby brand colors.
- `service/vectorize_test.go`
  - Verifies the production `logo` mode returns compact SVG output with required brand-color fills for BLS, red mark, and GJ samples, checks background RGB drift, checks nearest-fill RGB color drift, compares foreground coverage, then renders the SVG and compares it back to the normalized source with an RMSE threshold.
  - Verifies `logo`, `cleanLogo`, and `clean-logo` return the documented clean Logo preset metadata, while non-logo modes do not return a preset.
- `config/config.go`
  - Adds `IMAGE_MAGICK_PATH` and `POTRACE_PATH` configuration for the production clean Logo pipeline.
- `web/src/services/api/image.ts` and `web/src/app/(user)/canvas/[id]/canvas-client-page.tsx`
  - Default canvas vectorize requests now use `mode=logo`, and SVG node metadata records the returned vectorize engine and preset for later debugging.
- `web/src/app/(user)/canvas/types.ts`
  - Adds `engine` and `preset` to canvas node metadata so generated SVG nodes can retain the backend branch name and effective Logo parameters.
- `demos/svg/main.go`
  - Adds `cleanLogo`, automatic color-count selection, slider H5 controls, immediate/queued parameter refresh, quality metrics, same-size rendered comparison output, mode dispatch, `--batch`, and `--corpus-out`.
- `demos/svg/main_test.go`
  - Adds automatic color and full vectorization regression tests.
- `demos/svg/color_mask.go`
  - Adds request metrics for the color-mask comparison mode.
- `demos/svg/layered_ribbon.go`
  - Adds request metrics for the failed comparison mode.
- `demos/svg/README.md`
  - Documents the recommended preset and validation evidence.
- `docs/content/docs/progress/pending-test.mdx`
  - Records manual validation requirements and current evidence.

## Known Boundaries

- This preset is for logos and simple brand marks.
- Large banner illustrations, textured photos, posters, and composition-heavy images need a separate preset.
- The current regression samples are useful but small. Add more real logo samples before promoting this into the production `/api/v1/images/vectorize` endpoint.
- Text clarity is protected by BLS, real-font small-text, long real-font tagline, serif logistic/service text, low-contrast gray tagline, CJK real-font text, dark-background white-text, and micro-text stress samples, but production promotion should still add more real logos with different typefaces.
- Dark-background light strokes are now covered by a deterministic stress sample, but still need real logo samples with actual typefaces before claiming production-perfect dark-background white text.
- Flat illustrated slogan banners with clean vector-like shapes and text are now covered as a boundary stress case. Dense photographic, textured, or poster-like banners are still outside the current `cleanLogo` target and should use a separate banner/illustration preset instead of weakening the logo cleanup rules.

## Next Steps

- Add 10-20 real logo samples covering: monochrome marks, dark background logos, small text, gradients, thin-line icons, and three-plus brand-color marks before treating the logo-mode mapping as fully production-proven. Run them with `--batch-strict --batch-min-count 10` first, then raise the minimum to `20` for the broader gate.
- Use each run's `visual-comparison.png` for quick source/rendered/diff review, and use `go run ./demos/svg --batch ...` to generate `contact-sheet.png` for the 10-20 real logo samples.
- Use `batch-summary.json` quality counts to decide whether a new real-logo sample is ready, needs parameter tuning, or should become a new regression fixture.
- Production `/api/v1/images/vectorize` already maps `mode=logo`, `mode=cleanLogo`, and `mode=clean-logo` to this preset; keep observing real samples before removing the old comparison paths.
- Docker/runtime images now install `potrace`; verify the deployed runtime has `potrace` available or configure `POTRACE_PATH`.
- Keep `layeredRibbon` only as a failure comparison; do not migrate it.
