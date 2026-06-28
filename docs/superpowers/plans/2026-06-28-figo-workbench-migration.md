# Figo Workbench Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Migrate the figo-style `haotushow.com` marketing homepage and `/workbench` experience into `infinite-canvas`, serve them through separate domains, and use only `infinite-canvas` remote services for models, upload, generation, billing, history, assets, and admin records.

**Architecture:** `figo` is a source for front-end product experience only: marketing homepage sections, scene definitions, form structure, prompt-building rules, visual layout, and interaction patterns. Runtime data and side effects stay inside `infinite-canvas`: `/api/settings`, `/api/v1/images/generations`, `/api/v1/images/edits`, `/api/v1/image-tasks`, `/api/v1/deduction-logs`, image storage, asset storage, and the existing admin pages. Front-end product entry points are split by domain using the same Next.js project: `haotushow.com` for the brand homepage, `canvas.haotushow.com` for canvas, and `workbench.haotushow.com` for the figo-style workbench. The first release migrates the homepage and three representative workbench scenes, then uses the same extension points for the remaining scenes.

**Tech Stack:** Next.js App Router, React, TypeScript, Tailwind CSS, Ant Design where already used by canvas infrastructure, Zustand, localforage, Go, Gin, GORM.

## Global Constraints

- `infinite-canvas` is the main site and the only runtime backend.
- Do not call, import, or preserve any `figo-generate-images-website` backend API.
- `/workbench` must use remote cloud channels only; ignore local direct channel mode, Base URL, API Key, and local model fetch.
- `/workbench` visual style should copy the current `figo` workbench style as closely as possible.
- Reuse `infinite-canvas` services first; add new backend fields or endpoints only when existing APIs cannot represent required history, billing, or source metadata.
- Workbench generation history must appear in canvas image history.
- Workbench billing must appear in canvas user/admin deduction logs.
- Workbench generated images and saved outputs must use canvas image storage and asset systems.
- `haotushow.com` homepage must also live in this Next.js project so the public brand entry, workbench entry, and canvas entry are complete.
- First implementation scope: `门头招牌` (`STOREFRONT`), `海报设计` (`POSTER`), and `菜单设计` (`MENU`).
- Keep implementation scoped; do not build a new scene-management admin in the first release.
- Repository instruction: do not execute build, lint, or test commands unless the user explicitly asks. Keep verification commands in this plan as handoff commands, and record whether they were run or left for the user.
- Use one Next.js project with multiple domain entry points; do not create a separate workbench frontend repository in the first release.
- Do not create a separate homepage frontend repository in the first release.
- `haotushow.com` should map to the migrated figo-style marketing homepage.
- `workbench.haotushow.com` should map to the `/workbench` experience while still using the same unified canvas API and account backend.

---

## Current Decisions

- Main repository: `/Users/points/Documents/gitee1/infinite-canvas`.
- Reference repository: `/Users/points/Documents/gitee1/figo-generate-images-website`.
- Visual source files:
  - `figo-generate-images-website/codes/h5/src/App.tsx`
  - `figo-generate-images-website/codes/h5/src/index.css`
  - `figo-generate-images-website/codes/h5/src/data.ts`
  - `figo-generate-images-website/codes/h5/src/generation.ts`
  - `figo-generate-images-website/codes/h5/src/types.ts`
- Canvas model source:
  - `web/src/stores/use-config-store.ts`
  - `service/settings.go`
  - `model/setting.go`
- Canvas generation APIs:
  - `web/src/services/api/image.ts`
  - `handler/ai.go`
  - `service/auth.go`
  - `repository/user.go`
- Canvas history and billing:
  - `web/src/services/api/image-tasks.ts`
  - `web/src/components/image-tasks/image-task-history.tsx`
  - `web/src/services/api/deduction-logs.ts`
  - `web/src/app/(user)/deduction-logs/page.tsx`
  - `web/src/app/(admin)/admin/deduction-logs/page.tsx`

## Progress Log

- 2026-06-28: Confirmed `infinite-canvas` is the main site.
- 2026-06-28: Confirmed no `figo` backend should be migrated or used.
- 2026-06-28: Confirmed `/workbench` should visually copy the `figo` workbench.
- 2026-06-28: Confirmed workbench history, billing, assets, model config, and admin visibility must be unified with canvas.
- 2026-06-28: Confirmed workbench uses remote cloud channels only.
- 2026-06-28: Created this restartable implementation document.
- 2026-06-28: Re-reviewed the plan for omissions; added real front-end test command shape, database docs, WebDAV snapshot sync, login/remote-only assumptions, and task ID propagation.
- 2026-06-28: Confirmed deployment direction: same Next.js project, multiple domain entry points; `workbench.haotushow.com` should serve the workbench frontend while keeping the same backend.
- 2026-06-28: Confirmed `haotushow.com` marketing homepage should also migrate into this same Next.js project for a complete multi-domain product surface.

## File Structure

- Create `web/src/app/(user)/workbench/page.tsx`
  - Route entry for the figo-style workbench.
- Create `web/src/app/(user)/site/page.tsx`
  - Internal route for the migrated `haotushow.com` brand homepage. Keep the current canvas `/` page intact for `canvas.haotushow.com`; host routing decides which homepage a domain sees.
- Create `web/src/features/site/`
  - Figo-style marketing homepage sections, CTA routing, gallery, pricing/credits copy, footer, and brand assets.
- Create `web/src/features/workbench/types.ts`
  - Scene IDs, scene metadata, prompt field types, reference slot types, workbench generation log types.
- Create `web/src/features/workbench/scenes.ts`
  - Migrated first-scope scenes and presets from `figo`.
- Create `web/src/features/workbench/reference-slots.ts`
  - Reference image slot labels and prompt roles for first-scope scenes.
- Create `web/src/features/workbench/prompt-builder.ts`
  - Canvas-compatible prompt builder based on `figo` `buildGenerationPayload`.
- Create `web/src/features/workbench/remote-config.ts`
  - Helpers that force workbench generation into remote channel mode and select image models from canvas public settings.
- Create `web/src/features/workbench/workbench-log-store.ts`
  - Local form snapshot store keyed by canvas `AIImageTask.taskId`, so workbench history can restore form state while cloud history remains authoritative.
- Create `web/src/features/workbench/components/`
  - Figo-style presentational components for scene rail, form panels, reference slots, model/parameter area, results, and history.
- Create `web/src/features/workbench/styles.css`
  - Scoped figo-style workbench CSS. Avoid changing global canvas styles except the route import.
- Modify `web/src/app/(user)/layout.tsx` or current navigation component if needed
  - Add a visible `/workbench` entry.
- Modify `model/user.go`
  - Add source metadata fields to `AIImageTask`, only if the existing schema cannot preserve source data cleanly.
- Modify `handler/ai.go`, `service/auth.go`, `repository/user.go`
  - Read optional source metadata from image requests and copy it into `AIImageTask` and `CreditLog.Extra`.
- Modify `web/src/services/api/image.ts`
  - Add optional request metadata for `source`, `sceneId`, `sceneName`, and `templateName`; expose the canvas image task ID to callers that need history linkage.
- Modify `web/src/services/api/image-tasks.ts`
  - Add matching TypeScript fields if backend metadata is added.
- Modify `web/src/components/image-tasks/image-task-history.tsx`
  - Display source/scene metadata when present.
- Modify deduction log pages only if existing parsed `extra` display needs source/scene labels.
- Modify `docs/content/docs/backend/backend-database.mdx`
  - Document new `AIImageTask` source metadata fields if database fields are added.
- Modify `web/src/services/app-sync.ts`
  - Include workbench form snapshots in the existing `image-workbench` WebDAV sync domain, or explicitly document why they are local-only.
- Modify `web/package.json`
  - Add a real front-end test script if new TypeScript tests are added; the current package has no `npm test` script.
- Create or modify `web/src/middleware.ts` if host-based routing is implemented in app code
  - Rewrite `haotushow.com` requests to `/site` and `workbench.haotushow.com` requests to `/workbench` while leaving API/static assets untouched.
- Modify deployment config only if host-based routing is handled outside Next.js
  - Map `haotushow.com` and `workbench.haotushow.com` to the same Next.js app and route them to `/site` and `/workbench`.

## Domain Entry Contract

Use one Next.js application and one backend:

```text
haotushow.com             -> migrated figo-style brand/marketing homepage in this Next.js project
canvas.haotushow.com      -> canvas product entry
workbench.haotushow.com   -> /workbench figo-style product entry
```

First-release implementation should keep dedicated internal app routes and add host routing at the deployment layer or in Next.js middleware:

```text
haotushow.com/            -> internal brand homepage route `/site`
canvas.haotushow.com/     -> existing canvas homepage route `/`
workbench.haotushow.com/  -> `/workbench`
```

If host routing is added inside Next.js, preserve these rules:

- Do not rewrite `/api/*`, `/_next/*`, `/image-proxy`, `/webdav-proxy`, static assets, or uploaded/media routes.
- Requests for `haotushow.com/` should render the migrated brand homepage, not the existing canvas homepage.
- Requests for `canvas.haotushow.com/` should keep rendering the existing canvas homepage.
- Requests for `workbench.haotushow.com/` should render `/workbench`.
- Deep links under `workbench.haotushow.com` should remain inside the workbench product unless explicitly linking to canvas, admin, login, image history, deduction logs, or assets.
- Auth cookies should be configured for a shared parent domain such as `.haotushow.com` when cross-subdomain login sharing is required.
- The workbench UI should not show canvas-first navigation as its primary frame; cross-links to canvas/history/assets can exist as secondary actions.
- The marketing homepage should route primary CTA buttons to `workbench.haotushow.com` or `/workbench` and secondary creator/designer CTAs to `canvas.haotushow.com` or `/canvas`.

## Homepage Migration Scope

Migrate the `figo` homepage as a brand/product page, not as a new backend app. Source sections currently live inside `figo-generate-images-website/codes/h5/src/App.tsx` around the `home`, `portfolio`, and `credit` tab rendering.

First-release homepage should include:

- Header with 好图秀 logo and CTAs.
- Hero section for "好图秀 AI 商业设计图一键生成平台".
- Scene/category cards that route into `/workbench` with a selected scene when possible.
- Product strengths / core technology section.
- Gallery section backed by canvas featured image tasks when practical, or static figo visual cards for first release if live data is too much scope.
- Credits/pricing explanation that reflects canvas real recharge plans if exposed, otherwise keep it as marketing copy and route recharge actions through the unified canvas account flow.
- Footer with ICP and public security filing links currently present in canvas footer.

Do not migrate figo homepage mock recharge behavior. Any pricing CTA must route to the unified canvas recharge/account flow.

## Source Metadata Contract

Use this shape for workbench generation metadata:

```ts
export type WorkbenchGenerationMetadata = {
    source: "workbench";
    sceneId: "STOREFRONT" | "POSTER" | "MENU";
    sceneName: "门头招牌" | "海报设计" | "菜单设计";
    templateName?: string;
};
```

Preferred backend persistence:

```go
type AIImageTask struct {
    // existing fields...
    Source       string `json:"source" gorm:"index"`
    SceneID      string `json:"sceneId" gorm:"index"`
    SceneName    string `json:"sceneName"`
    TemplateName string `json:"templateName"`
}
```

Credit log `extra` should include these fields alongside the existing model, path, prompt, imageUrl, taskId, and frozenCredits fields:

```json
{
  "source": "workbench",
  "sceneId": "STOREFRONT",
  "sceneName": "门头招牌",
  "templateName": "现代极简"
}
```

## Workbench Runtime Requirements

- Workbench generation requires a logged-in canvas user because remote `/api/v1/images/*` endpoints require `UserAuth`.
- If the user is not logged in, `/workbench` should show the existing canvas login entry or open the same login route/modal pattern used by the rest of the user app.
- Workbench must ignore local direct channel mode even if the global config is set to `local`; force `channelMode: "remote"` in the adapter.
- Workbench model selection should use `useEffectiveConfig()` and canvas public settings, with `effectiveConfig.imageModel` as the default.
- Workbench credit preview must use the same rule as canvas remote image generation: model base credits, supported 4K `+3`, first reference image free, second and later reference images `+1` each, multiplied by generation count.
- Workbench 4K requests must preserve the canvas business marker (`4k:`) when needed so backend billing can identify 4K correctly.
- Workbench reference image compression must use canvas public image setting `publicSettings.image.referenceCompressionQuality` through existing `requestEdit` behavior.
- Workbench generation wrappers must return both generated image data and the canvas/upstream task ID when available; form snapshots and history restore depend on this link.

## Task List

### Task 1: Backend Source Metadata

**Files:**
- Modify: `model/user.go`
- Modify: `handler/ai.go`
- Modify: `service/auth.go`
- Modify: `repository/user.go`
- Modify: `repository/user_test.go`
- Modify: `web/src/services/api/image-tasks.ts`
- Modify: `docs/content/docs/backend/backend-database.mdx`

**Interfaces:**
- Consumes: optional request fields `source`, `sceneId`, `sceneName`, `templateName`.
- Produces: `AIImageTask.source`, `AIImageTask.sceneId`, `AIImageTask.sceneName`, `AIImageTask.templateName`; matching fields in credit log `extra`.

- [ ] **Step 1: Add a failing repository test for source metadata**

Add a test in `repository/user_test.go` that freezes and completes a workbench image task, then asserts the task and consume log preserve source fields.

Run: `go test ./repository -run TestCompleteAIImageTaskSuccessPreservesWorkbenchMetadata -v`

Expected: FAIL before fields exist.

- [ ] **Step 2: Add fields to `model.AIImageTask`**

Add `Source`, `SceneID`, `SceneName`, and `TemplateName` fields to `model/user.go`.

- [ ] **Step 3: Thread metadata through service and repository**

Extend `FreezeAIImageCredits` to accept source metadata or an options struct. Keep existing call sites readable.

- [ ] **Step 4: Parse metadata from JSON and multipart image requests**

In `handler/ai.go`, read optional fields from both `/images/generations` JSON and `/images/edits` multipart requests.

- [ ] **Step 5: Add metadata to credit log extra**

Update `aiImageCreditLogExtra` so freeze, consume, and release logs can identify workbench source and scene.

- [ ] **Step 6: Update TypeScript task type**

Add optional `source`, `sceneId`, `sceneName`, and `templateName` fields to `AIImageTask` in `web/src/services/api/image-tasks.ts`.

- [ ] **Step 7: Update database documentation**

Add the new `ai_image_tasks` metadata fields to `docs/content/docs/backend/backend-database.mdx`.

- [ ] **Step 8: Run backend tests**

Run: `go test ./...`

Expected: PASS.

### Task 2: Workbench Scene Core

**Files:**
- Create: `web/src/features/workbench/types.ts`
- Create: `web/src/features/workbench/scenes.ts`
- Create: `web/src/features/workbench/reference-slots.ts`
- Create: `web/src/features/workbench/prompt-builder.ts`
- Create: `web/test/workbench-prompt-builder.test.ts`
- Modify: `web/package.json` if no test script exists yet.

**Interfaces:**
- Produces: `WORKBENCH_SCENES`, `WORKBENCH_PRESETS`, `REFERENCE_SLOT_INSTRUCTIONS`, `buildWorkbenchPrompt(input)`.

- [ ] **Step 1: Write prompt-builder tests**

Cover `STOREFRONT`, `POSTER`, and `MENU`. Assert scene fields, reference roles, additional prompt, negative prompt, and unified text requirements are present.

Run: `cd web && node --test test/workbench-prompt-builder.test.ts`

Expected: FAIL before implementation.

- [ ] **Step 2: Port first three scene definitions**

Move only `STOREFRONT`, `POSTER`, and `MENU` from `figo` into `scenes.ts`.

- [ ] **Step 3: Port first three reference slot definitions**

Move the corresponding reference slot labels and roles into `reference-slots.ts`.

- [ ] **Step 4: Implement `buildWorkbenchPrompt`**

Keep output canvas-compatible: final result is a plain prompt string plus metadata, not a figo backend payload.

- [ ] **Step 5: Add or reuse a front-end test script**

If `web/package.json` still has no test script, add `"test": "node --test"` so future agents can run targeted tests consistently.

- [ ] **Step 6: Run tests**

Run: `cd web && node --test test/workbench-prompt-builder.test.ts`

Expected: PASS.

### Task 3: Remote-Only Workbench Generation Adapter

**Files:**
- Create: `web/src/features/workbench/remote-config.ts`
- Modify: `web/src/services/api/image.ts`
- Create: `web/src/features/workbench/workbench-generation.ts`
- Create: `web/test/workbench-generation.test.ts`
- Modify: `web/package.json` if no test script exists yet.

**Interfaces:**
- Consumes: canvas `AiConfig`, prompt, references, workbench metadata.
- Produces: generation/edit calls that always use `channelMode: "remote"` and include source metadata; returns generated images plus `taskId` when available.

- [ ] **Step 1: Write tests for remote-only config**

Assert local/global mode is ignored and output config always has `channelMode: "remote"` with the selected image model.

- [ ] **Step 2: Add optional metadata to `requestGeneration` and `requestEdit`**

The request body/form should include `source`, `sceneId`, `sceneName`, and `templateName` when provided.

- [ ] **Step 3: Preserve task IDs from image responses**

Extend the image API helpers so callers can opt into a richer result shape containing generated images and `taskId`. Do not break existing call sites that expect only image arrays.

- [ ] **Step 4: Implement workbench generation wrapper**

Choose `requestEdit` when references exist, otherwise `requestGeneration`.

- [ ] **Step 5: Refresh remote user after generation**

Reuse existing `requestGeneration` / `requestEdit` behavior so balance updates through `hydrateUser`.

- [ ] **Step 6: Record front-end test command**

Run: `cd web && node --test test/workbench-generation.test.ts`

Expected when run: PASS.

### Task 4: Workbench Local Snapshot Store

**Files:**
- Create: `web/src/features/workbench/workbench-log-store.ts`
- Create: `web/test/workbench-log-store.test.ts`
- Modify: `web/src/services/app-sync.ts`

**Interfaces:**
- Consumes: workbench form state and canvas `taskId`.
- Produces: local snapshots used to restore figo-style form state from a history item.

- [ ] **Step 1: Write local snapshot tests**

Assert snapshots are saved by task ID, restored by task ID, and serialized without large inline image blobs when storage keys exist.

- [ ] **Step 2: Implement localforage store**

Use a store name under the existing app namespace, for example `workbench_generation_snapshots`.

- [ ] **Step 3: Include snapshots in WebDAV sync**

Extend the existing `image-workbench` domain in `web/src/services/app-sync.ts` so workbench form snapshots can sync with image workbench records, unless product direction changes to local-only snapshots. Keep existing image-generation logs compatible.

- [ ] **Step 4: Add cleanup helper**

Provide a helper to delete snapshots for removed history items if the UI supports deletion.

- [ ] **Step 5: Run tests**

Run: `cd web && node --test test/workbench-log-store.test.ts`

Expected: PASS.

### Task 5: Figo-Style Marketing Homepage

**Files:**
- Create: `web/src/app/(user)/site/page.tsx`
- Create: `web/src/features/site/components/site-shell.tsx`
- Create: `web/src/features/site/components/site-header.tsx`
- Create: `web/src/features/site/components/site-hero.tsx`
- Create: `web/src/features/site/components/site-scene-showcase.tsx`
- Create: `web/src/features/site/components/site-strengths.tsx`
- Create: `web/src/features/site/components/site-gallery.tsx`
- Create: `web/src/features/site/components/site-pricing.tsx`
- Create: `web/src/features/site/components/site-footer.tsx`
- Create: `web/src/features/site/styles.css`

**Interfaces:**
- Consumes: figo homepage visual/content reference, canvas featured image tasks when practical, canvas public routes.
- Produces: `/site` internal route for `haotushow.com`.

- [ ] **Step 1: Create homepage route shell**

Create `/site` as the internal route for `haotushow.com`. Do not replace the existing canvas `/` route.

- [ ] **Step 2: Port figo homepage visual language**

Migrate the figo homepage look and copy structure: hero, scene cards, product strengths, gallery, pricing explanation, and footer.

- [ ] **Step 3: Replace figo tab actions with real routes**

Route primary CTA actions to `/workbench`; route designer/canvas actions to `/canvas`; route account/recharge actions to the existing canvas account/recharge flow.

- [ ] **Step 4: Remove figo mock-only behavior**

Do not port mock recharge, local credit mutation, or old figo auth modal behavior. Use canvas account entry points only.

- [ ] **Step 5: Decide gallery data source**

Prefer canvas featured image tasks through `fetchFeaturedImageTasks`. If this introduces too much first-release coupling, use static figo-style visual cards and record a follow-up to switch to live featured records.

- [ ] **Step 6: Manual responsive check**

Verify mobile and desktop hero, CTA, scene cards, gallery, pricing, and footer do not overflow.

### Task 6: Figo-Style Workbench Page

**Files:**
- Create: `web/src/app/(user)/workbench/page.tsx`
- Create: `web/src/features/workbench/components/workbench-shell.tsx`
- Create: `web/src/features/workbench/components/scene-sidebar.tsx`
- Create: `web/src/features/workbench/components/scene-form.tsx`
- Create: `web/src/features/workbench/components/reference-slot-grid.tsx`
- Create: `web/src/features/workbench/components/generation-panel.tsx`
- Create: `web/src/features/workbench/components/result-gallery.tsx`
- Create: `web/src/features/workbench/components/workbench-history.tsx`
- Create: `web/src/features/workbench/styles.css`

**Interfaces:**
- Consumes: scene core, remote generation adapter, canvas image storage, canvas user/config stores.
- Produces: `/workbench` route with figo-style visual layout.

- [ ] **Step 1: Create route shell**

Render the workbench as the first screen, not a marketing landing page.

- [ ] **Step 2: Port figo visual style**

Use `figo` workbench layout, palette, cards, dark/light theme toggle, scene rail, form panel, result canvas, and history panel as visual reference.

- [ ] **Step 3: Add first three scene forms**

Support `STOREFRONT`, `POSTER`, and `MENU` with their required fields and reference slots.

- [ ] **Step 4: Add remote model selector and image parameters**

Use canvas remote image models and model cost data; do not show local channel controls.

- [ ] **Step 5: Add login and balance handling**

If no canvas user token exists, guide the user to log in before generation. Show current credits and frozen credits using the existing user store, and rely on backend errors for final balance enforcement.

- [ ] **Step 6: Add generation flow**

Upload/resolve references through canvas image storage, build prompt, call workbench generation wrapper, display pending state, then show results.

- [ ] **Step 7: Save result actions**

Support download, use-as-reference, and save-to-assets through canvas utilities.

- [ ] **Step 8: Restore from history**

When a workbench snapshot exists for a task, restore scene, fields, references, model, size, quality, and result preview.

- [ ] **Step 9: Manual responsive check**

Verify desktop and mobile layouts for text overflow, control sizing, and result display.

### Task 7: Unified History and Deduction Log Display

**Files:**
- Modify: `web/src/components/image-tasks/image-task-history.tsx`
- Modify: `web/src/app/(user)/deduction-logs/page.tsx`
- Modify: `web/src/app/(admin)/admin/deduction-logs/page.tsx`
- Modify: `web/src/app/(admin)/admin/image-history/page.tsx` only if filters need source/scene.
- Modify: `web/src/services/app-sync.ts` if workbench snapshots are shown from history.

**Interfaces:**
- Consumes: `AIImageTask` source metadata and `CreditLog.Extra` metadata.
- Produces: user/admin pages that visibly identify workbench source and scene.

- [ ] **Step 1: Add source/scene display to image history**

Show a compact label such as `工作台 · 门头招牌` when metadata exists.

- [ ] **Step 2: Add source/scene display to deduction logs**

Parse `extra.source`, `extra.sceneName`, and `extra.templateName` and show them without breaking existing rows.

- [ ] **Step 3: Add optional filtering only if cheap**

If existing query filtering can use keyword search, do not add new filters in first release.

- [ ] **Step 4: Verify WebDAV sync still covers image history**

If workbench snapshots are synced through `image-workbench`, run through sync mentally or manually with one saved workbench snapshot and confirm it does not break existing image generation logs.

- [ ] **Step 5: Verify pages**

Open `/image-history`, `/deduction-logs`, `/admin/image-history`, and `/admin/deduction-logs` after generating from workbench.

### Task 8: Navigation, Domain Entries, and Release Verification

**Files:**
- Modify: `web/src/components/layout/app-top-nav.tsx` or the current user navigation entry point.
- Create or Modify: `web/src/middleware.ts` if host-based routing is done in Next.js.
- Modify: deployment config if host-based routing is handled by the platform instead of Next.js.
- Modify: `docs/superpowers/plans/2026-06-28-figo-workbench-migration.md`

**Interfaces:**
- Consumes: completed `/workbench` route.
- Produces: visible navigation entry and updated progress log.

- [ ] **Step 1: Add navigation entry**

Add `工作台` linking to `/workbench`.

- [ ] **Step 2: Add domain entry handling**

Either configure deployment routing so `haotushow.com` serves `/site` and `workbench.haotushow.com` serves `/workbench`, or add Next.js middleware that rewrites only host page requests and leaves API/static/proxy routes untouched.

- [ ] **Step 3: Check shared-auth deployment settings**

Confirm whether production auth cookies need `.haotushow.com` domain support so `canvas.haotushow.com` and `workbench.haotushow.com` share login state.

- [ ] **Step 4: Run full verification**

Record these handoff commands. Run them only if the user explicitly asks for verification in the current session:

```bash
go test ./...
cd web && npm run format:check
cd web && npm run build
```

Expected when run: all commands pass.

- [ ] **Step 5: Browser verification**

Start the dev server only if the user asks for verification. Open `/workbench`, generate one image for each first-scope scene, then verify history, deduction logs, and user balance.

- [ ] **Step 6: Domain verification**

After deployment routing exists, open `haotushow.com`, `canvas.haotushow.com`, and `workbench.haotushow.com`; confirm each lands in the correct frontend experience, confirm API requests still hit the unified backend, and confirm cross-links still work.

- [ ] **Step 7: Update this document**

Mark completed tasks, add the exact verification commands and results to the Progress Log, and record any follow-up tasks.

## Restart Checklist

When resuming after a fresh session:

1. Read this file first.
2. Run `git status --short` in `/Users/points/Documents/gitee1/infinite-canvas`.
3. Check the latest entries in `Progress Log`.
4. Continue from the first unchecked task.
5. Before editing files, inspect current versions because the user may have changed them.
6. After each completed task, update this file immediately with checkbox changes and a progress log entry.

## Open Questions

- Whether to add database fields for source metadata or store all source metadata only in `AIImageTask.Extra`. Current recommendation: add explicit fields for `source`, `sceneId`, `sceneName`, and `templateName`.
- Whether first release should include all figo scenes after the first three are stable. Current recommendation: no; ship first three, then extend.
- Whether workbench history should support deleting cloud history from inside `/workbench`. Current recommendation: no first-release deletion; rely on existing history pages.

## Handoff

Plan created on 2026-06-28. Start with Task 1 unless the user explicitly asks for a front-end-only prototype first.
