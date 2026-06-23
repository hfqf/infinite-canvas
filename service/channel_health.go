package service

import (
	"strings"
	"sync"
	"time"

	"github.com/basketikun/infinite-canvas/model"
)

const (
	defaultChannelFailureThreshold = 3
	defaultChannelCooldownSeconds  = 120
)

type modelChannelHealth struct {
	ConsecutiveFailures int
	CooldownUntil       time.Time
}

var (
	modelChannelHealthMu    sync.Mutex
	modelChannelHealthByKey = map[string]modelChannelHealth{}
)

func availableModelChannels(channels []model.ModelChannel, modelName string, now time.Time) []model.ModelChannel {
	result := []model.ModelChannel{}
	for _, channel := range modelChannelsForModel(channels, modelName) {
		if !isModelChannelCoolingDown(channel, now) {
			result = append(result, channel)
		}
	}
	return result
}

func RecordModelChannelFailure(channel model.ModelChannel) {
	recordModelChannelFailure(channel, time.Now())
}

func RecordModelChannelFailureWithCooldown(channel model.ModelChannel, cooldownSeconds int) {
	recordModelChannelFailureWithCooldown(channel, cooldownSeconds, time.Now())
}

func RecordModelChannelSuccess(channel model.ModelChannel) {
	modelChannelHealthMu.Lock()
	defer modelChannelHealthMu.Unlock()
	delete(modelChannelHealthByKey, modelChannelHealthKey(channel))
}

func recordModelChannelFailure(channel model.ModelChannel, now time.Time) {
	channel = normalizeModelChannel(channel)
	recordModelChannelFailureWithCooldown(channel, channel.CooldownSeconds, now)
}

func recordModelChannelFailureWithCooldown(channel model.ModelChannel, cooldownSeconds int, now time.Time) {
	channel = normalizeModelChannel(channel)
	if cooldownSeconds <= 0 {
		cooldownSeconds = channel.CooldownSeconds
	}
	key := modelChannelHealthKey(channel)
	modelChannelHealthMu.Lock()
	defer modelChannelHealthMu.Unlock()
	state := modelChannelHealthByKey[key]
	state.ConsecutiveFailures++
	if state.ConsecutiveFailures >= channel.FailureThreshold {
		state.CooldownUntil = now.Add(time.Duration(cooldownSeconds) * time.Second)
	}
	modelChannelHealthByKey[key] = state
}

func isModelChannelCoolingDown(channel model.ModelChannel, now time.Time) bool {
	modelChannelHealthMu.Lock()
	defer modelChannelHealthMu.Unlock()
	state := modelChannelHealthByKey[modelChannelHealthKey(channel)]
	return !state.CooldownUntil.IsZero() && now.Before(state.CooldownUntil)
}

func modelChannelHealthKey(channel model.ModelChannel) string {
	return strings.TrimSpace(channel.Name) + "|" + normalizeModelChannelBaseURL(channel.BaseURL)
}

func resetModelChannelHealthForTest() {
	modelChannelHealthMu.Lock()
	defer modelChannelHealthMu.Unlock()
	modelChannelHealthByKey = map[string]modelChannelHealth{}
}
