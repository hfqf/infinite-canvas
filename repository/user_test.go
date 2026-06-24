package repository

import (
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/basketikun/infinite-canvas/config"
	"github.com/basketikun/infinite-canvas/model"
)

func TestListUsersSearchesByID(t *testing.T) {
	resetDBForTest(t)
	user := model.User{ID: "user_search_id_001", Username: "credit-log-user", Role: model.UserRoleUser, Status: model.UserStatusActive}
	saved, err := SaveUser(user)
	if err != nil {
		t.Fatal(err)
	}

	users, total, err := ListUsers(model.Query{Keyword: saved.ID, Page: 1, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}

	if total != 1 || len(users) != 1 || users[0].ID != saved.ID {
		t.Fatalf("users=%#v total=%d, want saved user by id", users, total)
	}
}

func TestCompleteAIImageTaskSuccessChargesOnce(t *testing.T) {
	resetDBForTest(t)
	user, err := SaveUser(model.User{ID: "user_ai_task_001", Username: "ai-task-user", Role: model.UserRoleUser, Status: model.UserStatusActive, Credits: 20})
	if err != nil {
		t.Fatal(err)
	}
	task := model.AIImageTask{
		ID:        "ai_image_task_001",
		TaskID:    "task_g_001",
		UserID:    user.ID,
		Model:     "gpt-image-2",
		Path:      "/images/generations",
		Prompt:    "blue cat",
		Credits:   6,
		Status:    "running",
		CreatedAt: "created",
		UpdatedAt: "created",
	}
	if _, err = SaveAIImageTask(task); err != nil {
		t.Fatal(err)
	}

	completed, charged, err := CompleteAIImageTaskSuccess(task.TaskID, user.ID, "succeeded", "https://cdn.example.com/cat.png", "done")
	if err != nil {
		t.Fatal(err)
	}
	if !charged {
		t.Fatal("first completion charged=false, want true")
	}
	if completed.ChargedAt != "done" || completed.ImageURL != "https://cdn.example.com/cat.png" || completed.Status != "succeeded" {
		t.Fatalf("completed task=%#v, want charged task with image url", completed)
	}
	user, ok, err := GetUserByID(user.ID)
	if err != nil || !ok {
		t.Fatalf("load user ok=%v err=%v", ok, err)
	}
	if user.Credits != 14 {
		t.Fatalf("credits=%d, want 14", user.Credits)
	}

	_, charged, err = CompleteAIImageTaskSuccess(task.TaskID, user.ID, "succeeded", "https://cdn.example.com/cat.png", "done-again")
	if err != nil {
		t.Fatal(err)
	}
	if charged {
		t.Fatal("second completion charged=true, want idempotent false")
	}
	user, ok, err = GetUserByID(user.ID)
	if err != nil || !ok {
		t.Fatalf("load user ok=%v err=%v", ok, err)
	}
	if user.Credits != 14 {
		t.Fatalf("credits after second completion=%d, want 14", user.Credits)
	}
	logs, total, err := ListCreditLogs(model.Query{Keyword: task.TaskID, Page: 1, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(logs) != 1 {
		t.Fatalf("logs=%#v total=%d, want one consume log", logs, total)
	}
	if logs[0].Amount != -6 || logs[0].RelatedID != task.TaskID || !strings.Contains(logs[0].Extra, "blue cat") || !strings.Contains(logs[0].Extra, "https://cdn.example.com/cat.png") {
		t.Fatalf("log=%#v, want -6 related task with prompt and image url in extra", logs[0])
	}
}

func TestFreezeAIImageTaskPreventsOverspendingAndCanRelease(t *testing.T) {
	resetDBForTest(t)
	user, err := SaveUser(model.User{ID: "user_ai_freeze_001", Username: "ai-freeze-user", Role: model.UserRoleUser, Status: model.UserStatusActive, Credits: 10})
	if err != nil {
		t.Fatal(err)
	}
	first := model.AIImageTask{ID: "ai_image_task_freeze_001", TaskID: "ai_image_task_freeze_001", UserID: user.ID, Model: "gpt-image-2", Path: "/images/generations", Prompt: "first", Credits: 6}
	if _, ok, err := FreezeAIImageTask(first, "freeze-1"); err != nil || !ok {
		t.Fatalf("first freeze ok=%v err=%v, want true nil", ok, err)
	}
	second := model.AIImageTask{ID: "ai_image_task_freeze_002", TaskID: "ai_image_task_freeze_002", UserID: user.ID, Model: "gpt-image-2", Path: "/images/generations", Prompt: "second", Credits: 6}
	if _, ok, err := FreezeAIImageTask(second, "freeze-2"); err != nil || ok {
		t.Fatalf("second freeze ok=%v err=%v, want false nil", ok, err)
	}
	user, ok, err := GetUserByID(user.ID)
	if err != nil || !ok {
		t.Fatalf("load user ok=%v err=%v", ok, err)
	}
	if user.Credits != 10 || user.FrozenCredits != 6 {
		t.Fatalf("user credits=%d frozen=%d, want total 10 frozen 6", user.Credits, user.FrozenCredits)
	}
	if _, released, err := ReleaseAIImageTask(first.TaskID, user.ID, "failed", "release-1"); err != nil || !released {
		t.Fatalf("release released=%v err=%v, want true nil", released, err)
	}
	user, ok, err = GetUserByID(user.ID)
	if err != nil || !ok {
		t.Fatalf("load user ok=%v err=%v", ok, err)
	}
	if user.Credits != 10 || user.FrozenCredits != 0 {
		t.Fatalf("after release credits=%d frozen=%d, want total 10 frozen 0", user.Credits, user.FrozenCredits)
	}
	logs, total, err := ListCreditLogs(model.Query{Keyword: first.TaskID, Page: 1, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(logs) != 2 {
		t.Fatalf("logs=%#v total=%d, want freeze and release logs", logs, total)
	}
	if logs[0].Type != model.CreditLogTypeAIFreezeRelease || logs[1].Type != model.CreditLogTypeAIFreeze {
		t.Fatalf("log types=%s,%s want release,freeze", logs[0].Type, logs[1].Type)
	}
}

func resetDBForTest(t *testing.T) {
	t.Helper()
	previousConfig := config.Cfg
	previousDB := db
	previousOnce := dbOnce
	previousErr := dbErr
	config.Cfg = config.Config{StorageDriver: "sqlite", DatabaseDSN: filepath.Join(t.TempDir(), "test.db")}
	db = nil
	dbErr = nil
	dbOnce = sync.Once{}
	t.Cleanup(func() {
		config.Cfg = previousConfig
		db = previousDB
		dbErr = previousErr
		dbOnce = previousOnce
	})
}
