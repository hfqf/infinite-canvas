package service

import (
	"reflect"
	"testing"

	"github.com/basketikun/infinite-canvas/config"
)

func TestPng2SVGCleanArgsUsesDefaultProfile(t *testing.T) {
	originalProfile := config.Cfg.Png2SVGCleanProfile
	originalBin := config.Cfg.Png2SVGCleanBin
	t.Cleanup(func() {
		config.Cfg.Png2SVGCleanProfile = originalProfile
		config.Cfg.Png2SVGCleanBin = originalBin
	})
	config.Cfg.Png2SVGCleanProfile = ""
	config.Cfg.Png2SVGCleanBin = ""

	got := png2SVGCleanArgs("input.png", "output.svg")
	want := []string{"bin/png2svg-generic-85.mjs", "input.png", "output.svg", "--profile", "generic-85"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("png2SVGCleanArgs()=%#v, want %#v", got, want)
	}
}

func TestPng2SVGCleanArgsUsesConfiguredProfile(t *testing.T) {
	originalProfile := config.Cfg.Png2SVGCleanProfile
	originalBin := config.Cfg.Png2SVGCleanBin
	t.Cleanup(func() {
		config.Cfg.Png2SVGCleanProfile = originalProfile
		config.Cfg.Png2SVGCleanBin = originalBin
	})
	config.Cfg.Png2SVGCleanProfile = "custom-profile"
	config.Cfg.Png2SVGCleanBin = "bin/custom.mjs"

	got := png2SVGCleanArgs("input.png", "output.svg")
	want := []string{"bin/custom.mjs", "input.png", "output.svg", "--profile", "custom-profile"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("png2SVGCleanArgs()=%#v, want %#v", got, want)
	}
}

func TestVectorizeEngineUsesPng2SVGCleanNode(t *testing.T) {
	if got := vectorizeEngine("colorMask"); got != "png2svg-clean-node" {
		t.Fatalf("vectorizeEngine()=%q, want png2svg-clean-node", got)
	}
}
