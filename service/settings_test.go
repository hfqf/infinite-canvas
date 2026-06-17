package service

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/basketikun/infinite-canvas/model"
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

func TestNormalizeSettingsPublishesEnabledRoutablePublicModelsAndRepairsDefaults(t *testing.T) {
	settings := normalizeSettings(model.Settings{
		Public: model.PublicSetting{
			ModelChannel: model.PublicModelChannelSetting{
				Models: []model.PublicModelSpec{
					{Model: "gpt-5.5", Capability: "text", Enabled: true},
					{Model: "doubao-seedream-5.0-lite", Capability: "image", Enabled: true},
					{Model: "doubao-seedance-2.0-fast", Capability: "video", Enabled: true},
					{Model: "disabled-model", Capability: "image", Enabled: true},
				},
				DefaultModel:      "missing-default",
				DefaultTextModel:  "missing-text",
				DefaultImageModel: "missing-image",
				DefaultVideoModel: "missing-video",
			},
		},
		Private: model.PrivateSetting{
			Channels: []model.ModelChannel{
				{
					Enabled: true,
					Routes: []model.ModelChannelRoute{
						{Model: "gpt-5.5", Enabled: true},
						{Model: "doubao-seedream-5.0-lite", Enabled: true},
						{Model: "doubao-seedance-2.0-fast", Enabled: true},
					},
				},
				{Enabled: false, Routes: []model.ModelChannelRoute{{Model: "disabled-model", Enabled: true}}},
			},
		},
	})

	channel := settings.Public.ModelChannel
	wantModels := []string{"gpt-5.5", "doubao-seedream-5.0-lite", "doubao-seedance-2.0-fast"}
	gotModels := []string{}
	for _, item := range channel.Models {
		gotModels = append(gotModels, item.Model)
	}
	if !reflect.DeepEqual(gotModels, wantModels) {
		t.Fatalf("public models = %#v, want %#v", gotModels, wantModels)
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
}

func TestSelectModelRouteFromSettingsUsesRouteCreditsAndUpstreamModel(t *testing.T) {
	settings := normalizeSettings(model.Settings{
		Public: model.PublicSetting{
			ModelChannel: model.PublicModelChannelSetting{
				Models: []model.PublicModelSpec{
					{Model: "gpt-image-2-1k", Capability: "image", Enabled: true, DefaultCredits: 10},
				},
				DefaultImageModel: "gpt-image-2-1k",
			},
		},
		Private: model.PrivateSetting{
			Channels: []model.ModelChannel{
				{
					Name:    "服务商 A",
					BaseURL: "https://vendor-a.example.com",
					APIKey:  "sk-a",
					Enabled: true,
					Routes: []model.ModelChannelRoute{
						{Model: "gpt-image-2-1k", UpstreamModel: "gpt-image-2", Credits: 8, Weight: 1, Enabled: true},
					},
				},
			},
		},
	})

	selected, err := selectModelRouteFromSettings(settings, "gpt-image-2-1k")
	if err != nil {
		t.Fatalf("selectModelRouteFromSettings returned error: %v", err)
	}
	if selected.PublicModelName != "gpt-image-2-1k" {
		t.Fatalf("public model = %q, want gpt-image-2-1k", selected.PublicModelName)
	}
	if selected.UpstreamModel != "gpt-image-2" {
		t.Fatalf("upstream model = %q, want gpt-image-2", selected.UpstreamModel)
	}
	if selected.Credits != 8 {
		t.Fatalf("credits = %v, want route price 8", selected.Credits)
	}
}

func TestSelectModelRouteFromSettingsRejectsDisabledPublicModel(t *testing.T) {
	settings := normalizeSettings(model.Settings{
		Public: model.PublicSetting{
			ModelChannel: model.PublicModelChannelSetting{
				Models: []model.PublicModelSpec{
					{Model: "gpt-image-2-0.5k", Capability: "image", Enabled: false, GiftEligible: true, DefaultCredits: 1},
				},
			},
		},
		Private: model.PrivateSetting{
			Channels: []model.ModelChannel{
				{
					Name:    "系统自营",
					BaseURL: "https://relay.example.com",
					APIKey:  "sk-relay",
					Enabled: true,
					Routes: []model.ModelChannelRoute{
						{Model: "gpt-image-2-0.5k", UpstreamModel: "gpt-image-2", Credits: 1, Weight: 1, Enabled: true},
					},
				},
			},
		},
	})

	_, err := selectModelRouteFromSettings(settings, "gpt-image-2-0.5k")
	if err == nil {
		t.Fatal("expected disabled public model to be rejected")
	}
	if !strings.Contains(err.Error(), "模型未启用或不存在") {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestBuildCreditDebitUsesGiftCreditsOnlyForEligibleModel(t *testing.T) {
	user := model.User{Credits: 5, GiftCredits: 10}
	eligible := SelectedModelRoute{PublicModel: model.PublicModelSpec{Model: "gpt-image-2-0.5k", GiftEligible: true}}
	debit, ok := buildCreditDebit(user, eligible, 8)
	if !ok {
		t.Fatal("expected eligible model to use gift credits")
	}
	if debit.GiftCredits != 8 || debit.Credits != 0 {
		t.Fatalf("eligible debit = %#v, want gift 8 and credits 0", debit)
	}

	paidOnly := SelectedModelRoute{PublicModel: model.PublicModelSpec{Model: "gpt-image-2-1k", GiftEligible: false}}
	debit, ok = buildCreditDebit(user, paidOnly, 8)
	if ok {
		t.Fatalf("expected paid-only model to reject when common credits are insufficient, debit = %#v", debit)
	}
	if debit.GiftCredits != 0 || debit.Credits != 8 {
		t.Fatalf("paid-only debit = %#v, want gift 0 and credits 8", debit)
	}
}
