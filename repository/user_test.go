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
	_, released, err := ReleaseAIImageTask(task.TaskID, user.ID, "failed", "failed-after-success")
	if err != nil {
		t.Fatal(err)
	}
	if released {
		t.Fatal("release after charged task released=true, want false")
	}
	completed, ok, err = GetAIImageTaskByTaskID(task.TaskID)
	if err != nil || !ok {
		t.Fatalf("load task ok=%v err=%v", ok, err)
	}
	if completed.Status != "succeeded" || completed.ChargedAt != "done" || completed.ReleasedAt != "" {
		t.Fatalf("task after release attempt=%#v, want charged succeeded unchanged", completed)
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
	if _, charged, err := CompleteAIImageTaskSuccess(first.TaskID, user.ID, "succeeded", "https://cdn.example.com/late.png", "success-after-release"); err != nil || charged {
		t.Fatalf("complete released task charged=%v err=%v, want false nil", charged, err)
	}
	user, ok, err = GetUserByID(user.ID)
	if err != nil || !ok {
		t.Fatalf("load user ok=%v err=%v", ok, err)
	}
	if user.Credits != 10 || user.FrozenCredits != 0 {
		t.Fatalf("after late success credits=%d frozen=%d, want total 10 frozen 0", user.Credits, user.FrozenCredits)
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

func TestListFrozenAIImageTasks(t *testing.T) {
	resetDBForTest(t)
	user := model.User{ID: "user_frozen_tasks_001", Username: "frozen-task-user", Role: model.UserRoleUser, Status: model.UserStatusActive, Credits: 20}
	if _, err := SaveUser(user); err != nil {
		t.Fatal(err)
	}
	frozen := model.AIImageTask{ID: "task_frozen_001", TaskID: "task_frozen_001", UserID: user.ID, Model: "gpt-image-2", Path: "/images/edits", Prompt: "frozen", Credits: 6}
	if _, ok, err := FreezeAIImageTask(frozen, "freeze-time"); err != nil || !ok {
		t.Fatalf("freeze task ok=%v err=%v", ok, err)
	}
	if _, err := SaveAIImageTask(model.AIImageTask{ID: "task_done_001", TaskID: "task_done_001", UserID: user.ID, Model: "gpt-image-2", FrozenAt: "freeze-time", ChargedAt: "charged-time"}); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveAIImageTask(model.AIImageTask{ID: "task_released_001", TaskID: "task_released_001", UserID: user.ID, Model: "gpt-image-2", FrozenAt: "freeze-time", ReleasedAt: "released-time"}); err != nil {
		t.Fatal(err)
	}

	tasks, err := ListFrozenAIImageTasks(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || tasks[0].TaskID != frozen.TaskID {
		t.Fatalf("tasks=%#v, want only frozen task", tasks)
	}
}

func TestListUserAIImageTasksFiltersUserAndKeyword(t *testing.T) {
	resetDBForTest(t)
	user := model.User{ID: "user_image_history_001", Username: "history-user", Role: model.UserRoleUser, Status: model.UserStatusActive, AffCode: "HIST001"}
	if _, err := SaveUser(user); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveUser(model.User{ID: "user_image_history_002", Username: "other-history-user", Role: model.UserRoleUser, Status: model.UserStatusActive, AffCode: "HIST002"}); err != nil {
		t.Fatal(err)
	}
	tasks := []model.AIImageTask{
		{ID: "history_task_001", TaskID: "task_history_001", UserID: user.ID, Model: "gpt-image-2", Path: "/images/generations", Prompt: "blue cat", Status: "succeeded", ImageURL: "https://cdn.example.com/cat.png", Size: "1024x1024", Quality: "low", Count: 1, ReferenceCount: 0, Credits: 2, CreatedAt: "2026-06-24 10:00:00", UpdatedAt: "2026-06-24 10:01:00"},
		{ID: "history_task_002", TaskID: "task_history_002", UserID: user.ID, Model: "gpt-image-2", Path: "/images/edits", Prompt: "red dog", Status: "failed", Size: "2048x2048", Quality: "medium", Count: 1, ReferenceCount: 2, Credits: 4, CreatedAt: "2026-06-24 11:00:00", UpdatedAt: "2026-06-24 11:01:00"},
		{ID: "history_task_003", TaskID: "task_history_003", UserID: "user_image_history_002", Model: "gpt-image-2", Path: "/images/generations", Prompt: "blue cat other", Status: "succeeded", CreatedAt: "2026-06-24 12:00:00", UpdatedAt: "2026-06-24 12:01:00"},
	}
	for _, task := range tasks {
		if _, err := SaveAIImageTask(task); err != nil {
			t.Fatal(err)
		}
	}

	list, total, err := ListUserAIImageTasks(user.ID, model.Query{Keyword: "blue", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(list) != 1 || list[0].TaskID != "task_history_001" {
		t.Fatalf("tasks=%#v total=%d, want only current user's blue task", list, total)
	}
}

func TestListInvitationRecordsFiltersInviterAndKeyword(t *testing.T) {
	resetDBForTest(t)
	inviter := model.User{ID: "user_inviter_001", Username: "inviter-user", DisplayName: "邀请人", Role: model.UserRoleUser, Status: model.UserStatusActive, AffCode: "INV001"}
	otherInviter := model.User{ID: "user_inviter_002", Username: "other-inviter", Role: model.UserRoleUser, Status: model.UserStatusActive, AffCode: "INV002"}
	if _, err := SaveUser(inviter); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveUser(otherInviter); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveUser(model.User{ID: "user_invitee_001", Username: "invitee-blue", Email: "blue@example.com", Role: model.UserRoleUser, Status: model.UserStatusActive, AffCode: "INVITEE1", InviterID: inviter.ID, CreatedAt: "2026-06-25T10:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveUser(model.User{ID: "user_invitee_002", Username: "invitee-red", Email: "red@example.com", Role: model.UserRoleUser, Status: model.UserStatusActive, AffCode: "INVITEE2", InviterID: otherInviter.ID, CreatedAt: "2026-06-25T11:00:00Z"}); err != nil {
		t.Fatal(err)
	}

	records, total, err := ListInvitationRecords(inviter.ID, model.Query{Keyword: "blue", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(records) != 1 {
		t.Fatalf("records=%#v total=%d, want one inviter record", records, total)
	}
	if records[0].InviterID != inviter.ID || records[0].InviteeUsername != "invitee-blue" || records[0].InviterUsername != "inviter-user" {
		t.Fatalf("record=%#v, want inviter and invitee info", records[0])
	}
}

func TestConsumeUserCreditsUsesAvailableCredits(t *testing.T) {
	resetDBForTest(t)
	user, err := SaveUser(model.User{ID: "user_available_credits_001", Username: "available-user", Role: model.UserRoleUser, Status: model.UserStatusActive, Credits: 10, FrozenCredits: 6})
	if err != nil {
		t.Fatal(err)
	}

	if _, ok, err := ConsumeUserCredits(user.ID, 5, "consume-too-much"); err != nil || ok {
		t.Fatalf("consume frozen credits ok=%v err=%v, want false nil", ok, err)
	}
	user, ok, err := GetUserByID(user.ID)
	if err != nil || !ok {
		t.Fatalf("load user ok=%v err=%v", ok, err)
	}
	if user.Credits != 10 || user.FrozenCredits != 6 {
		t.Fatalf("after failed consume credits=%d frozen=%d, want total 10 frozen 6", user.Credits, user.FrozenCredits)
	}

	if _, ok, err = ConsumeUserCredits(user.ID, 4, "consume-available"); err != nil || !ok {
		t.Fatalf("consume available credits ok=%v err=%v, want true nil", ok, err)
	}
	user, ok, err = GetUserByID(user.ID)
	if err != nil || !ok {
		t.Fatalf("load user ok=%v err=%v", ok, err)
	}
	if user.Credits != 6 || user.FrozenCredits != 6 {
		t.Fatalf("after consume credits=%d frozen=%d, want total 6 frozen 6", user.Credits, user.FrozenCredits)
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
