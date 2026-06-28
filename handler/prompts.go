package handler

import (
	"encoding/json"
	"net/http"

	"github.com/basketikun/infinite-canvas/model"
	"github.com/basketikun/infinite-canvas/service"
)

func Prompts(w http.ResponseWriter, r *http.Request) {
	result, err := service.ListPrompts(parseQuery(r))
	if err != nil {
		FailError(w, err)
		return
	}
	OK(w, result)
}

func UserPrompts(w http.ResponseWriter, r *http.Request) {
	user, ok := service.UserFromContext(r.Context())
	if !ok {
		Fail(w, "请先登录")
		return
	}
	result, err := service.ListUserPrompts(user.ID, parseQuery(r))
	if err != nil {
		FailError(w, err)
		return
	}
	OK(w, result)
}

func SaveUserPrompt(w http.ResponseWriter, r *http.Request) {
	user, ok := service.UserFromContext(r.Context())
	if !ok {
		Fail(w, "请先登录")
		return
	}
	var item model.UserPrompt
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		Fail(w, "请求参数错误")
		return
	}
	result, err := service.SaveUserPrompt(user.ID, item)
	if err != nil {
		FailError(w, err)
		return
	}
	OK(w, result)
}

func DeleteUserPrompt(w http.ResponseWriter, r *http.Request, id string) {
	user, ok := service.UserFromContext(r.Context())
	if !ok {
		Fail(w, "请先登录")
		return
	}
	if err := service.DeleteUserPrompt(user.ID, id); err != nil {
		FailError(w, err)
		return
	}
	OK(w, map[string]bool{"ok": true})
}

func ReorderUserPrompts(w http.ResponseWriter, r *http.Request) {
	user, ok := service.UserFromContext(r.Context())
	if !ok {
		Fail(w, "请先登录")
		return
	}
	var req struct {
		Items []model.UserPromptReorderItem `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Fail(w, "请求参数错误")
		return
	}
	orders := map[string]int{}
	for _, item := range req.Items {
		if item.ID == "" {
			continue
		}
		orders[item.ID] = item.SortOrder
	}
	if err := service.ReorderUserPrompts(user.ID, orders); err != nil {
		FailError(w, err)
		return
	}
	OK(w, map[string]bool{"ok": true})
}
