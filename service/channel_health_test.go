package service

import (
	"testing"
	"time"

	"github.com/basketikun/infinite-canvas/model"
)

func TestNormalizeModelChannelSetsCircuitBreakerDefaults(t *testing.T) {
	channel := normalizeModelChannel(model.ModelChannel{})

	if channel.FailureThreshold != defaultChannelFailureThreshold {
		t.Fatalf("failure threshold = %d, want %d", channel.FailureThreshold, defaultChannelFailureThreshold)
	}
	if channel.CooldownSeconds != defaultChannelCooldownSeconds {
		t.Fatalf("cooldown seconds = %d, want %d", channel.CooldownSeconds, defaultChannelCooldownSeconds)
	}
}

func TestRecordModelChannelFailureStartsCooldownAtThreshold(t *testing.T) {
	resetModelChannelHealthForTest()
	now := time.Date(2026, 6, 23, 12, 0, 0, 0, time.UTC)
	channel := normalizeModelChannel(model.ModelChannel{
		Name:             "primary",
		BaseURL:          "https://primary.example.com",
		FailureThreshold: 2,
		CooldownSeconds:  30,
	})

	recordModelChannelFailure(channel, now)
	if isModelChannelCoolingDown(channel, now) {
		t.Fatal("channel cooled down before reaching threshold")
	}

	recordModelChannelFailure(channel, now.Add(time.Second))
	if !isModelChannelCoolingDown(channel, now.Add(2*time.Second)) {
		t.Fatal("channel did not cool down after reaching threshold")
	}
	if isModelChannelCoolingDown(channel, now.Add(31*time.Second)) {
		t.Fatal("channel stayed cooled down after cooldown expired")
	}
}

func TestRecordModelChannelFailureWithCooldownOverridesChannelCooldown(t *testing.T) {
	resetModelChannelHealthForTest()
	now := time.Date(2026, 6, 23, 12, 0, 0, 0, time.UTC)
	channel := normalizeModelChannel(model.ModelChannel{
		Name:             "primary",
		BaseURL:          "https://primary.example.com",
		FailureThreshold: 1,
		CooldownSeconds:  30,
	})

	recordModelChannelFailureWithCooldown(channel, 180, now)

	if !isModelChannelCoolingDown(channel, now.Add(179*time.Second)) {
		t.Fatal("channel should still be cooling before custom cooldown expires")
	}
	if isModelChannelCoolingDown(channel, now.Add(181*time.Second)) {
		t.Fatal("channel should stop cooling after custom cooldown expires")
	}
}

func TestAvailableModelChannelsSkipCoolingChannelWhenBackupExists(t *testing.T) {
	resetModelChannelHealthForTest()
	now := time.Date(2026, 6, 23, 12, 0, 0, 0, time.UTC)
	primary := normalizeModelChannel(model.ModelChannel{
		Name:             "primary",
		BaseURL:          "https://primary.example.com",
		APIKey:           "primary-key",
		Models:           []string{"gpt-image-2"},
		Enabled:          true,
		FailureThreshold: 1,
		CooldownSeconds:  60,
	})
	backup := normalizeModelChannel(model.ModelChannel{
		Name:    "backup",
		BaseURL: "https://backup.example.com",
		APIKey:  "backup-key",
		Models:  []string{"gpt-image-2"},
		Enabled: true,
	})
	recordModelChannelFailure(primary, now)

	got := availableModelChannels([]model.ModelChannel{primary, backup}, "gpt-image-2", now.Add(time.Second))

	if len(got) != 1 || got[0].Name != "backup" {
		t.Fatalf("available channels = %#v, want only backup", got)
	}
}
