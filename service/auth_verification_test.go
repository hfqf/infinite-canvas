package service

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/basketikun/infinite-canvas/config"
	"github.com/basketikun/infinite-canvas/model"
	"github.com/basketikun/infinite-canvas/repository"
)

func TestRegisterRequiresEmailVerificationCode(t *testing.T) {
	previousConfig := config.Cfg
	config.Cfg = config.Config{
		StorageDriver:        "sqlite",
		DatabaseDSN:          filepath.Join(t.TempDir(), "test.db"),
		JWTSecret:            "test-secret",
		JWTExpireHours:       168,
		VerificationProvider: "noop",
	}
	t.Cleanup(func() {
		config.Cfg = previousConfig
	})

	email := "Designer.Verify@example.com"
	if _, err := Register("verify-user-missing", "password123", email, ""); err == nil || !strings.Contains(err.Error(), "验证码") {
		t.Fatalf("Register without code error = %v, want verification code error", err)
	}

	issued, err := RequestVerificationCode(email, "register")
	if err != nil {
		t.Fatalf("RequestVerificationCode: %v", err)
	}
	if issued.ExpiresInSeconds <= 0 || issued.DebugCode == "" {
		t.Fatalf("issued = %#v, want ttl and debug code for noop provider", issued)
	}

	session, err := Register("verify-user-ok", "password123", email, issued.DebugCode)
	if err != nil {
		t.Fatalf("Register with code: %v", err)
	}
	if session.User.Email != strings.ToLower(email) {
		t.Fatalf("session email = %q, want normalized email", session.User.Email)
	}

	if _, err := Register("verify-user-reuse", "password123", email, issued.DebugCode); err == nil {
		t.Fatal("Register reused code error = nil, want error")
	}
}

func TestRegisterWithInviteCodeRecordsInviter(t *testing.T) {
	previousConfig := config.Cfg
	config.Cfg = config.Config{
		StorageDriver:        "sqlite",
		DatabaseDSN:          filepath.Join(t.TempDir(), "test.db"),
		JWTSecret:            "test-secret",
		JWTExpireHours:       168,
		VerificationProvider: "noop",
	}
	t.Cleanup(func() {
		config.Cfg = previousConfig
	})

	inviterCodeEmail := "inviter@example.com"
	inviterCode, err := RequestVerificationCode(inviterCodeEmail, "register")
	if err != nil {
		t.Fatal(err)
	}
	inviterSession, err := Register("invite-owner", "password123", inviterCodeEmail, inviterCode.DebugCode, "")
	if err != nil {
		t.Fatal(err)
	}

	inviteeEmail := "invitee@example.com"
	inviteeCode, err := RequestVerificationCode(inviteeEmail, "register")
	if err != nil {
		t.Fatal(err)
	}
	session, err := Register("invitee-user", "password123", inviteeEmail, inviteeCode.DebugCode, inviterSession.User.AffCode)
	if err != nil {
		t.Fatal(err)
	}

	invitee, ok, err := repository.GetUserByID(session.User.ID)
	if err != nil || !ok {
		t.Fatalf("load invitee ok=%v err=%v", ok, err)
	}
	if invitee.InviterID != inviterSession.User.ID {
		t.Fatalf("invitee inviterId=%q, want %q", invitee.InviterID, inviterSession.User.ID)
	}
	inviter, ok, err := repository.GetUserByID(inviterSession.User.ID)
	if err != nil || !ok {
		t.Fatalf("load inviter ok=%v err=%v", ok, err)
	}
	if inviter.AffCount != 1 {
		t.Fatalf("inviter affCount=%d, want 1", inviter.AffCount)
	}
	if invitee.Credits != 33 {
		t.Fatalf("invitee credits=%d, want 33", invitee.Credits)
	}
	list, err := ListUserAIDeductionLogs(invitee.ID, model.Query{Keyword: "邀请注册", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if list.Total != 1 || len(list.Items) != 1 {
		t.Fatalf("logs=%#v total=%d, want one invite bonus log", list.Items, list.Total)
	}
	if list.Items[0].Type != model.CreditLogTypeInviteRegisterBonus || list.Items[0].Amount != 3 || list.Items[0].Balance != 33 {
		t.Fatalf("bonus log=%#v, want +3 balance 33", list.Items[0])
	}
}
