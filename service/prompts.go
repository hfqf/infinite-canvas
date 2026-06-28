package service

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/basketikun/infinite-canvas/model"
	"github.com/basketikun/infinite-canvas/repository"
)

const defaultUserPromptCategory = "默认"

func ListPrompts(q model.Query) (model.PromptList, error) {
	items, total, err := repository.ListPrompts(q)
	if err != nil {
		return model.PromptList{}, err
	}
	tags, err := repository.ListPromptTags(q)
	if err != nil {
		return model.PromptList{}, err
	}
	categories := promptCategoryCodes(ListPromptCategories())
	return model.PromptList{Items: items, Tags: tags, Categories: categories, Total: int(total)}, nil
}

func ListUserPrompts(userID string, q model.Query) (model.UserPromptList, error) {
	items, total, err := repository.ListUserPrompts(userID, q)
	if err != nil {
		return model.UserPromptList{}, err
	}
	tags, err := repository.ListUserPromptTags(userID, q)
	if err != nil {
		return model.UserPromptList{}, err
	}
	categories, err := repository.ListUserPromptCategories(userID)
	if err != nil {
		return model.UserPromptList{}, err
	}
	return model.UserPromptList{Items: items, Tags: tags, Categories: categories, Total: int(total)}, nil
}

func ListPromptCategories() []model.PromptCategory {
	categories, _ := repository.ListPromptCategories()
	return categories
}

func SavePrompt(item model.Prompt) (model.Prompt, error) {
	now := time.Now().Format(time.RFC3339)
	if item.Category == "" {
		item.Category = repository.PromptCategories()[0].Category
	}
	if item.ID == "" {
		item.ID = newID(item.Category)
		item.CreatedAt = now
	}
	item.UpdatedAt = now
	category, ok := repository.PromptCategoryByCode(item.Category)
	if !ok {
		category = repository.PromptCategories()[0]
		item.Category = category.Category
	}
	item.GithubURL = ""
	return repository.SavePrompt(item)
}

func SaveUserPrompt(userID string, item model.UserPrompt) (model.UserPrompt, error) {
	now := time.Now().Format(time.RFC3339)
	item.UserID = userID
	item.Title = strings.TrimSpace(item.Title)
	item.Prompt = strings.TrimSpace(item.Prompt)
	item.Category = strings.TrimSpace(item.Category)
	item.Source = strings.TrimSpace(item.Source)
	item.Tags = normalizePromptTags(item.Tags)
	if item.Prompt == "" {
		return item, safeMessageError{message: "提示词不能为空"}
	}
	if item.Title == "" {
		item.Title = firstRunes(item.Prompt, 24)
	}
	if item.Category == "" {
		item.Category = defaultUserPromptCategory
	}
	if item.ID == "" {
		item.ID = newID("user-prompt")
		item.CreatedAt = now
	}
	if item.SortOrder <= 0 {
		maxSort, err := repository.MaxUserPromptSortOrder(userID, item.Category)
		if err != nil {
			return item, err
		}
		item.SortOrder = maxSort + 10
	}
	item.UpdatedAt = now
	return repository.SaveUserPrompt(item)
}

func DeleteUserPrompt(userID string, id string) error {
	return repository.DeleteUserPrompt(userID, id)
}

func ReorderUserPrompts(userID string, orders map[string]int) error {
	if len(orders) == 0 {
		return nil
	}
	return repository.ReorderUserPrompts(userID, orders, time.Now().Format(time.RFC3339))
}

func DeletePrompt(id string) error {
	return repository.DeletePrompt(id)
}

func DeletePrompts(ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return repository.DeletePrompts(ids)
}

func normalizePromptTags(tags []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" || seen[tag] {
			continue
		}
		seen[tag] = true
		result = append(result, tag)
	}
	return result
}

func firstRunes(value string, limit int) string {
	if limit <= 0 || utf8.RuneCountInString(value) <= limit {
		return value
	}
	runes := []rune(value)
	return string(runes[:limit])
}

func promptCategoryCodes(items []model.PromptCategory) []string {
	codes := []string{}
	for _, item := range items {
		if item.Category != "" {
			codes = append(codes, item.Category)
		}
	}
	return codes
}
