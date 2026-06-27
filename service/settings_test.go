package service

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/basketikun/infinite-canvas/model"
	"github.com/basketikun/infinite-canvas/repository"
)

func TestFetchAdminChannelModelsParsesOpenAIModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"z-model"},{"id":"a-model"},{"id":""}]}`))
	}))
	defer server.Close()

	models, err := fetchAdminChannelModels(model.ModelChannel{
		BaseURL: server.URL,
		APIKey:  "test-key",
	})
	if err != nil {
		t.Fatalf("fetchAdminChannelModels returned error: %v", err)
	}
	if want := []string{"a-model", "z-model"}; !reflect.DeepEqual(models, want) {
		t.Fatalf("models = %#v, want %#v", models, want)
	}
}

func TestFetchAdminChannelModelsReportsArkPlanModelsUnsupported(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/plan/v3/models" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	_, err := fetchAdminChannelModels(model.ModelChannel{
		BaseURL: server.URL + "/api/plan/v3/contents/generations/tasks",
		APIKey:  "test-key",
	})
	if err == nil {
		t.Fatal("expected unsupported /models error")
	}
	if !strings.Contains(err.Error(), "Agent Plan 未提供 OpenAI /models") {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestBuildModelChannelURLNormalizesArkPlanTaskPath(t *testing.T) {
	got := BuildModelChannelURL(model.ModelChannel{BaseURL: "https://ark.cn-beijing.volces.com/api/plan/v3/contents/generations/tasks?debug=1"}, "/models")
	want := "https://ark.cn-beijing.volces.com/api/plan/v3/models"
	if got != want {
		t.Fatalf("BuildModelChannelURL = %q, want %q", got, want)
	}
}

func TestNormalizeSettingsPublishesEnabledChannelModelsAndRepairsDefaults(t *testing.T) {
	settings := normalizeSettings(model.Settings{
		Public: model.PublicSetting{
			ModelChannel: model.PublicModelChannelSetting{
				AvailableModels:   []string{"grok-imagine-video", "disabled-model"},
				DefaultModel:      "grok-imagine-video",
				DefaultTextModel:  "missing-text",
				DefaultImageModel: "missing-image",
				DefaultVideoModel: "missing-video",
			},
		},
		Private: model.PrivateSetting{
			Channels: []model.ModelChannel{
				{Enabled: true, Models: []string{"gpt-5.5", "doubao-seedream-5.0-lite", "doubao-seedance-2.0-fast", "gpt-5.5"}},
				{Enabled: false, Models: []string{"disabled-model"}},
			},
		},
	})

	channel := settings.Public.ModelChannel
	wantModels := []string{"gpt-5.5", "doubao-seedream-5.0-lite", "doubao-seedance-2.0-fast"}
	if !reflect.DeepEqual(channel.AvailableModels, wantModels) {
		t.Fatalf("available models = %#v, want %#v", channel.AvailableModels, wantModels)
	}
	if channel.DefaultModel != "gpt-5.5" {
		t.Fatalf("default model = %q, want text model", channel.DefaultModel)
	}
	if channel.DefaultTextModel != "gpt-5.5" {
		t.Fatalf("default text model = %q, want text model", channel.DefaultTextModel)
	}
	if channel.DefaultImageModel != "doubao-seedream-5.0-lite" {
		t.Fatalf("default image model = %q, want seedream", channel.DefaultImageModel)
	}
	if channel.DefaultVideoModel != "doubao-seedance-2.0-fast" {
		t.Fatalf("default video model = %q, want seedance", channel.DefaultVideoModel)
	}
	if settings.Public.Auth.InviteRewardCredits == nil || *settings.Public.Auth.InviteRewardCredits != 20 {
		t.Fatalf("invite reward credits = %v, want default 20", settings.Public.Auth.InviteRewardCredits)
	}
	if settings.Public.Image.ReferenceCompressionQuality == nil || *settings.Public.Image.ReferenceCompressionQuality != 0.8 {
		t.Fatalf("reference compression quality = %v, want default 0.8", settings.Public.Image.ReferenceCompressionQuality)
	}
}

func TestNormalizeSettingsPreservesZeroInviteRewardCredits(t *testing.T) {
	zero := 0
	settings := normalizeSettings(model.Settings{
		Public: model.PublicSetting{
			Auth: model.PublicAuthSetting{InviteRewardCredits: &zero},
		},
	})
	if settings.Public.Auth.InviteRewardCredits == nil || *settings.Public.Auth.InviteRewardCredits != 0 {
		t.Fatalf("invite reward credits = %v, want configured 0", settings.Public.Auth.InviteRewardCredits)
	}
}

func TestNormalizeSettingsClampsReferenceCompressionQuality(t *testing.T) {
	tooHigh := 1.2
	settings := normalizeSettings(model.Settings{
		Public: model.PublicSetting{
			Image: model.PublicImageSetting{ReferenceCompressionQuality: &tooHigh},
		},
	})
	if settings.Public.Image.ReferenceCompressionQuality == nil || *settings.Public.Image.ReferenceCompressionQuality != 1 {
		t.Fatalf("reference compression quality = %v, want capped 1", settings.Public.Image.ReferenceCompressionQuality)
	}

	tooLow := -0.1
	settings = normalizeSettings(model.Settings{
		Public: model.PublicSetting{
			Image: model.PublicImageSetting{ReferenceCompressionQuality: &tooLow},
		},
	})
	if settings.Public.Image.ReferenceCompressionQuality == nil || *settings.Public.Image.ReferenceCompressionQuality != 0.1 {
		t.Fatalf("reference compression quality = %v, want floored 0.1", settings.Public.Image.ReferenceCompressionQuality)
	}
}

func TestNormalizeSettingsClampsCanvasToolCosts(t *testing.T) {
	settings := normalizeSettings(model.Settings{
		Public: model.PublicSetting{
			Canvas: model.PublicCanvasSetting{
				ToolCosts: []model.CanvasToolCost{
					{Tool: "superResolve", Credits: 3},
					{Tool: "crop", Credits: -2},
					{Tool: "", Credits: 9},
				},
			},
		},
	})
	want := []model.CanvasToolCost{
		{Tool: "superResolve", Credits: 3},
		{Tool: "crop", Credits: 0},
	}
	if !reflect.DeepEqual(settings.Public.Canvas.ToolCosts, want) {
		t.Fatalf("tool costs = %#v, want %#v", settings.Public.Canvas.ToolCosts, want)
	}
}

func TestConsumeCanvasToolCreditsUsesConfiguredToolCost(t *testing.T) {
	userID := "user_canvas_tool_cost_001"
	if _, err := repository.SaveUser(model.User{ID: userID, Username: userID, Role: model.UserRoleUser, Status: model.UserStatusActive, Credits: 5, AffCode: "TOOLCOST001"}); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.SaveSettings(model.Settings{
		Public: model.PublicSetting{
			Canvas: model.PublicCanvasSetting{ToolCosts: []model.CanvasToolCost{{Tool: "crop", Credits: 2}}},
		},
	}, "now"); err != nil {
		t.Fatal(err)
	}

	user, err := ConsumeCanvasToolCredits(userID, "crop")
	if err != nil {
		t.Fatal(err)
	}
	if user.Credits != 3 {
		t.Fatalf("credits = %d, want 3", user.Credits)
	}
	list, err := ListCreditLogs(model.Query{Keyword: userID, Page: 1, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if list.Total == 0 || list.Items[0].Type != model.CreditLogTypeCanvasToolConsume || list.Items[0].Amount != -2 {
		t.Fatalf("logs=%#v total=%d, want canvas tool consume -2", list.Items, list.Total)
	}
}
