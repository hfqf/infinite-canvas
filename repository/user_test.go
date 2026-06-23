package repository

import (
	"path/filepath"
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
