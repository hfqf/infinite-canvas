package repository

import (
	"testing"

	"github.com/basketikun/infinite-canvas/model"
)

func TestPromptCategoriesExcludeUnavailableRemoteSources(t *testing.T) {
	removed := map[string]bool{
		"gpt-image-2-prompts":     true,
		"youmind-gpt-image-2":     true,
		"youmind-nano-banana-pro": true,
	}
	for _, item := range PromptCategories() {
		if removed[item.Category] {
			t.Fatalf("prompt category %q should be removed because its cover images are unavailable", item.Category)
		}
	}
}

func TestPromptCategoriesKeepAvailableRemoteSources(t *testing.T) {
	categories := map[string]bool{}
	for _, item := range PromptCategories() {
		categories[item.Category] = true
	}
	for _, category := range []string{"system", "awesome-gpt-image", "awesome-gpt4o-image-prompts", "davidwu-gpt-image2-prompts"} {
		if !categories[category] {
			t.Fatalf("prompt category %q should remain available", category)
		}
	}
}

func TestUserPromptsAreScopedByUserAndSorted(t *testing.T) {
	resetDBForTest(t)
	firstUser := "user_prompt_owner"
	secondUser := "user_prompt_other"
	_, err := SaveUserPrompt(model.UserPrompt{ID: "up_1", UserID: firstUser, Title: "Second", Prompt: "second prompt", Category: "海报", SortOrder: 20, CreatedAt: "1", UpdatedAt: "1"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = SaveUserPrompt(model.UserPrompt{ID: "up_2", UserID: firstUser, Title: "First", Prompt: "first prompt", Category: "海报", SortOrder: 10, Tags: []string{"常用"}, CreatedAt: "2", UpdatedAt: "2"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = SaveUserPrompt(model.UserPrompt{ID: "up_3", UserID: secondUser, Title: "Other", Prompt: "other prompt", Category: "菜单", SortOrder: 1, CreatedAt: "3", UpdatedAt: "3"})
	if err != nil {
		t.Fatal(err)
	}

	items, total, err := ListUserPrompts(firstUser, model.Query{Category: "海报", Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(items) != 2 || items[0].ID != "up_2" || items[1].ID != "up_1" {
		t.Fatalf("items=%#v total=%d, want first user's sorted prompts", items, total)
	}

	if err := ReorderUserPrompts(firstUser, map[string]int{"up_1": 1, "up_2": 2, "up_3": 0}, "reordered"); err != nil {
		t.Fatal(err)
	}
	items, _, err = ListUserPrompts(firstUser, model.Query{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if items[0].ID != "up_1" || items[1].ID != "up_2" {
		t.Fatalf("items after reorder=%#v, want up_1 then up_2", items)
	}
}
