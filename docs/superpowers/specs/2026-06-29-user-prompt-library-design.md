# User Prompt Library Upgrade Design

## Goal

Upgrade the prompt library into two clearly separated libraries:

- System prompt library: existing public/admin managed prompts and remote synced prompt sources.
- My prompt library: per-user cloud saved prompts that can be created from any prompt-bearing workflow, categorized, sorted, and selected from canvas/image/video generation pages.

The first release should keep the current system prompt behavior intact while adding account-scoped personal prompts.

## Current Context

- Existing system prompts use `model.Prompt` and the `prompts` table.
- `/prompts` shows the public prompt center.
- `/admin/prompts` manages system prompts and remote sync.
- `PromptSelectDialog` is already reused by canvas, image, and video pages, but it only reads public prompts.
- Prompt-bearing surfaces include image workbench, video workbench, canvas prompt panels, canvas config composer, image node metadata prompts, and image history details.

## Data Model

Add a new account-scoped table instead of mixing personal prompts into `prompts`:

```go
type UserPrompt struct {
    ID        string   `json:"id" gorm:"primaryKey"`
    UserID    string   `json:"userId" gorm:"index"`
    Title     string   `json:"title"`
    Prompt    string   `json:"prompt" gorm:"type:text"`
    Category  string   `json:"category" gorm:"index"`
    Tags      []string `json:"tags" gorm:"serializer:json"`
    SortOrder int      `json:"sortOrder" gorm:"index"`
    Source    string   `json:"source"`
    CreatedAt string   `json:"createdAt"`
    UpdatedAt string   `json:"updatedAt"`
}
```

Categories can be simple strings in the first release. A separate category table is not required unless category metadata becomes necessary later.

## Backend API

Keep existing system prompt endpoints unchanged.

Add logged-in user APIs:

- `GET /api/v1/user-prompts`
  - Query: `keyword`, `category`, `tag`, `page`, `pageSize`.
  - Response: items, tags, categories, total.
- `POST /api/v1/user-prompts`
  - Create or update one personal prompt.
- `DELETE /api/v1/user-prompts/:id`
  - Delete one personal prompt owned by the current user.
- `POST /api/v1/user-prompts/reorder`
  - Update `sortOrder` for a batch of prompt IDs owned by the current user.

Ordering:

- Default order: `sort_order asc, updated_at desc`.
- New prompts get a `sortOrder` after the current max for the user/category.

Authorization:

- All personal prompt APIs require login.
- Every repository query is scoped by `user_id`.

## Frontend Library Page

Update `/prompts` with a top segmented control:

- 系统提示词库
- 我的提示词库

System tab keeps the existing public prompt list and filters.

My tab adds:

- Search by title/prompt.
- Category filter.
- Tag filter.
- New prompt button.
- Edit/delete actions.
- Save from system prompt into my library.
- Sort controls in first release as simple move up/down or numeric order, not drag-and-drop.

My prompt form fields:

- Title
- Prompt
- Category
- Tags
- Sort order

## Save From Any Prompt Surface

Add a reusable `SaveUserPromptButton` / `SaveUserPromptDialog` pair.

Inputs:

```ts
{
  title?: string;
  prompt: string;
  source: string;
}
```

Behavior:

- If not logged in, show the existing login flow/message.
- Pre-fill title from prompt source or prompt first line.
- Let user choose or type category.
- Let user edit tags.
- Save to `POST /api/v1/user-prompts`.

Add the save action in these first-release places:

- `/image` workbench prompt area.
- `/video` workbench prompt area.
- Canvas node prompt panel.
- Canvas config/composer prompt area.
- Canvas image node hover toolbar when `metadata.prompt` exists.
- Image history detail prompt section.
- System prompt card/detail actions: "保存到我的提示词".

## Prompt Select Dialog

Upgrade `PromptSelectDialog` with a source segmented control:

- 我的提示词
- 系统提示词

Default source:

- If user is logged in: default to 我的提示词.
- If user is not logged in: default to 系统提示词 and hide or disable the personal tab with login hint.

Selection behavior remains unchanged:

- Selecting a prompt returns the prompt text to the caller.
- Canvas, image workbench, and video workbench keep using the same dialog component.

## Error Handling

- Personal prompt save requires non-empty prompt content.
- Empty title defaults to the first 24 characters of the prompt.
- Duplicate prompts are allowed in first release to keep the flow simple.
- Delete and reorder operations only affect the current user's rows.

## Testing And Verification

Recommended focused checks:

- Logged-in user creates, edits, deletes, filters, and reorders personal prompts.
- Another user cannot see or mutate those prompts.
- System prompt list remains unchanged.
- System prompt can be saved into my prompts.
- Image workbench, video workbench, and canvas prompt library can select personal prompts.
- Prompt-bearing surfaces can save current prompt into my prompts.
- Not logged-in state gracefully shows system prompts and prompts login for personal save.

## Non-Goals

- No separate admin management for personal prompts in first release.
- No drag-and-drop sorting in first release unless the existing UI can support it cheaply.
- No local-only personal prompt storage.
- No migration of existing system prompts into personal prompts.
