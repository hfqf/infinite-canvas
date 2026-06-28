package repository

import "testing"

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
