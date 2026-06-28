# User Prompt Library Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add account-scoped "我的提示词库" beside the existing system prompt library, with save, edit, delete, categorize, sort, and cross-page selection.

**Architecture:** Keep existing `prompts` as the system/public library. Add a separate `user_prompts` table scoped by `user_id`, expose logged-in `/api/v1/user-prompts` APIs, and upgrade the shared prompt picker to switch between personal and system sources.

**Tech Stack:** Go, Gin, GORM, SQLite/MySQL/Postgres, Next.js App Router, React, TypeScript, Ant Design, TanStack Query, Zustand.

## Global Constraints

- Do not mix personal prompt rows into the existing `prompts` table.
- Existing system prompt list, remote sync, and admin prompt management must keep working.
- All personal prompt APIs require login and must scope reads/writes by current `user_id`.
- First release uses simple string categories and simple numeric/up-down sorting.
- Do not run build/test/lint commands unless explicitly requested by the user.
- Update `docs/content/docs/progress/pending-test.mdx` when implementation is complete.

---

## File Structure

- Modify `model/prompt.go`: add `UserPrompt`, `UserPromptList`, and reorder request model.
- Modify `repository/db.go`: include `UserPrompt` in migrations and promote long text columns on MySQL.
- Modify `repository/prompt.go`: add user prompt list/save/delete/reorder repository functions.
- Modify `repository/prompt_test.go`: cover user scoping and sort order.
- Modify `service/prompts.go`: add validation/defaults and user-scoped prompt service functions.
- Modify `handler/prompts.go`: add HTTP handlers for current user's prompt library.
- Modify `router/router.go`: mount `/api/v1/user-prompts` routes.
- Modify `web/src/services/api/prompts.ts`: add user prompt API types and methods.
- Modify `web/src/components/prompts/use-prompt-list.ts`: add personal prompt list hook.
- Modify `web/src/components/prompts/prompt-select-dialog.tsx`: add source segmented control.
- Create `web/src/components/prompts/save-user-prompt-dialog.tsx`: reusable save dialog.
- Create `web/src/components/prompts/save-user-prompt-button.tsx`: small reusable save action.
- Modify `/prompts`, `/image`, `/video`, canvas prompt components, image toolbar, and image history to expose save/select flows.
- Modify `docs/content/docs/progress/pending-test.mdx`: record testable changes.

## Task 1: Backend User Prompt Model And Repository

**Files:**
- Modify: `model/prompt.go`
- Modify: `repository/db.go`
- Modify: `repository/prompt.go`
- Modify: `repository/prompt_test.go`

**Interfaces:**
- Produces: `model.UserPrompt`, `model.UserPromptList`
- Produces: `repository.ListUserPrompts(userID string, q model.Query) ([]model.UserPrompt, int64, error)`
- Produces: `repository.SaveUserPrompt(item model.UserPrompt) (model.UserPrompt, error)`
- Produces: `repository.DeleteUserPrompt(userID string, id string) error`
- Produces: `repository.ReorderUserPrompts(userID string, orders map[string]int, now string) error`

- [x] **Step 1: Write repository tests**

Add tests in `repository/prompt_test.go`:

```go
func TestUserPromptsAreScopedByUserAndSorted(t *testing.T) {
    resetDBForTest(t)
    firstUser := "user_prompt_owner"
    secondUser := "user_prompt_other"
    _, err := SaveUserPrompt(model.UserPrompt{ID: "up_1", UserID: firstUser, Title: "Second", Prompt: "second prompt", Category: "海报", SortOrder: 20, CreatedAt: "1", UpdatedAt: "1"})
    if err != nil { t.Fatal(err) }
    _, err = SaveUserPrompt(model.UserPrompt{ID: "up_2", UserID: firstUser, Title: "First", Prompt: "first prompt", Category: "海报", SortOrder: 10, Tags: []string{"常用"}, CreatedAt: "2", UpdatedAt: "2"})
    if err != nil { t.Fatal(err) }
    _, err = SaveUserPrompt(model.UserPrompt{ID: "up_3", UserID: secondUser, Title: "Other", Prompt: "other prompt", Category: "菜单", SortOrder: 1, CreatedAt: "3", UpdatedAt: "3"})
    if err != nil { t.Fatal(err) }

    items, total, err := ListUserPrompts(firstUser, model.Query{Category: "海报", Page: 1, PageSize: 20})
    if err != nil { t.Fatal(err) }
    if total != 2 || len(items) != 2 || items[0].ID != "up_2" || items[1].ID != "up_1" {
        t.Fatalf("items=%#v total=%d, want first user's sorted prompts", items, total)
    }

    if err := ReorderUserPrompts(firstUser, map[string]int{"up_1": 1, "up_2": 2, "up_3": 0}, "reordered"); err != nil {
        t.Fatal(err)
    }
    items, _, err = ListUserPrompts(firstUser, model.Query{Page: 1, PageSize: 20})
    if err != nil { t.Fatal(err) }
    if items[0].ID != "up_1" || items[1].ID != "up_2" {
        t.Fatalf("items after reorder=%#v, want up_1 then up_2", items)
    }
}
```

- [x] **Step 2: Add model and migration**

Add `UserPrompt` and `UserPromptList` to `model/prompt.go`, add `&model.UserPrompt{}` to `repository/db.go` migrations, and add MySQL long text entries for `user_prompts.prompt`, `title`, `tags`, and `source`.

- [x] **Step 3: Implement repository functions**

In `repository/prompt.go`, add list, tags/categories collection, save, delete, max sort, and reorder functions, mirroring existing prompt filter patterns but always filtering by `user_id`.

- [x] **Step 4: Verification command**

Suggested command if tests are explicitly requested:

```bash
go test ./repository -run TestUserPromptsAreScopedByUserAndSorted -v
```

## Task 2: Backend Service, Handlers, And Routes

**Files:**
- Modify: `service/prompts.go`
- Modify: `handler/prompts.go`
- Modify: `router/router.go`

**Interfaces:**
- Consumes Task 1 repository functions.
- Produces `GET/POST/DELETE/POST reorder` under `/api/v1/user-prompts`.

- [x] **Step 1: Add service validation**

Add service functions:

```go
func ListUserPrompts(userID string, q model.Query) (model.UserPromptList, error)
func SaveUserPrompt(userID string, item model.UserPrompt) (model.UserPrompt, error)
func DeleteUserPrompt(userID string, id string) error
func ReorderUserPrompts(userID string, orders map[string]int) error
```

Rules: trim title/prompt/category/source, require non-empty prompt, default title to first 24 runes of prompt, default category to `默认`, assign IDs and timestamps in service.

- [x] **Step 2: Add handlers**

Add handlers in `handler/prompts.go`:

```go
func UserPrompts(w http.ResponseWriter, r *http.Request)
func SaveUserPrompt(w http.ResponseWriter, r *http.Request)
func DeleteUserPrompt(w http.ResponseWriter, r *http.Request, id string)
func ReorderUserPrompts(w http.ResponseWriter, r *http.Request)
```

Use `service.UserFromContext(r.Context())` and return `Fail(w, "请先登录")` if missing.

- [x] **Step 3: Mount routes**

In `router/router.go`, add to `v1` group:

```go
v1.GET("/user-prompts", gin.WrapF(handler.UserPrompts))
v1.POST("/user-prompts", gin.WrapF(handler.SaveUserPrompt))
v1.POST("/user-prompts/reorder", gin.WrapF(handler.ReorderUserPrompts))
v1.DELETE("/user-prompts/:id", func(c *gin.Context) {
    handler.DeleteUserPrompt(c.Writer, c.Request, c.Param("id"))
})
```

## Task 3: Frontend User Prompt API And Shared Save Dialog

**Files:**
- Modify: `web/src/services/api/prompts.ts`
- Modify: `web/src/components/prompts/use-prompt-list.ts`
- Create: `web/src/components/prompts/save-user-prompt-dialog.tsx`
- Create: `web/src/components/prompts/save-user-prompt-button.tsx`

**Interfaces:**
- Produces `fetchUserPrompts`, `saveUserPrompt`, `deleteUserPrompt`, `reorderUserPrompts`.
- Produces `SaveUserPromptButton`.

- [x] **Step 1: Add API methods**

Extend `prompts.ts` with `UserPrompt`, `UserPromptListResponse`, `fetchUserPrompts(token, query)`, `saveUserPrompt(token, prompt)`, `deleteUserPrompt(token, id)`, and `reorderUserPrompts(token, orders)`.

- [x] **Step 2: Add hooks**

Add `useUserPromptList({ keyword, tags, category, enabled })` mirroring `usePromptList`, using token from `useUserStore`.

- [x] **Step 3: Add save dialog/button**

Create a dialog with fields `title`, `category`, `tagText`, `prompt`. `SaveUserPromptButton` opens it with initial prompt/source, checks login token, and invalidates `["user-prompts"]` query keys after save.

## Task 4: Prompt Library Page And Prompt Picker

**Files:**
- Modify: `web/src/app/(user)/prompts/page.tsx`
- Modify: `web/src/components/prompts/prompt-select-dialog.tsx`

**Interfaces:**
- Consumes Task 3 hooks and save button.
- Produces system/my library switching on page and picker.

- [x] **Step 1: Add library segmented control to `/prompts`**

Use Ant Design `Segmented` with `系统提示词库` and `我的提示词库`. System tab keeps current UI. My tab lists personal prompts with edit/delete/save form and sort controls.

- [x] **Step 2: Add source segmented control to picker**

`PromptSelectDialog` gets `我的提示词` / `系统提示词`. Logged-in users default to personal prompts; guests default to system prompts and see a login hint for personal prompts.

## Task 5: Save Prompt Entry Points

**Files:**
- Modify: `web/src/app/(user)/image/page.tsx`
- Modify: `web/src/app/(user)/video/page.tsx`
- Modify: `web/src/app/(user)/canvas/components/canvas-node-prompt-panel.tsx`
- Modify: `web/src/app/(user)/canvas/components/canvas-config-composer.tsx`
- Modify: `web/src/app/(user)/canvas/components/canvas-node-hover-toolbar.tsx`
- Modify: `web/src/components/image-tasks/image-task-history.tsx`
- Modify: `web/src/components/prompts/prompt-card.tsx`
- Modify: `web/src/components/prompts/prompt-detail-dialog.tsx`

**Interfaces:**
- Consumes `SaveUserPromptButton`.

- [x] **Step 1: Add save buttons near prompt inputs**

Add compact `SaveUserPromptButton` next to prompt library buttons in image/video/canvas prompt panels.

- [x] **Step 2: Add save from prompt metadata**

In canvas image toolbar and image history detail, show save action only when a prompt exists.

- [x] **Step 3: Add save from system cards**

System prompt cards/details get "保存到我的提示词".

## Task 6: Docs And Handoff

**Files:**
- Modify: `docs/content/docs/progress/pending-test.mdx`
- Modify: `docs/content/docs/backend/backend-database.mdx`

**Steps:**

- [x] Document `user_prompts` table.
- [x] Add pending-test entry covering create/edit/delete/filter/reorder, selecting personal prompts, and saving from prompt surfaces.
- [x] Run `git diff --check`.
