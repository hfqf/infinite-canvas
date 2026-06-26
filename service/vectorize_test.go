package service

import (
	"reflect"
	"testing"

	"github.com/basketikun/infinite-canvas/config"
)

func TestPng2SVGCleanArgsUsesDefaultProfile(t *testing.T) {
	original := config.Cfg.Png2SVGCleanProfile
	t.Cleanup(func() { config.Cfg.Png2SVGCleanProfile = original })
	config.Cfg.Png2SVGCleanProfile = ""

	got := png2SVGCleanArgs("input.png", "output.svg")
	want := []string{"bin/png2svg-clean.mjs", "input.png", "output.svg", "--profile", "generic-clean-logo"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("png2SVGCleanArgs()=%#v, want %#v", got, want)
	}
}

func TestPng2SVGCleanArgsUsesConfiguredProfile(t *testing.T) {
	original := config.Cfg.Png2SVGCleanProfile
	t.Cleanup(func() { config.Cfg.Png2SVGCleanProfile = original })
	config.Cfg.Png2SVGCleanProfile = "bls-clean-ribbon"

	got := png2SVGCleanArgs("input.png", "output.svg")
	want := []string{"bin/png2svg-clean.mjs", "input.png", "output.svg", "--profile", "bls-clean-ribbon"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("png2SVGCleanArgs()=%#v, want %#v", got, want)
	}
}

func TestVectorizeEngineUsesPng2SVGCleanNode(t *testing.T) {
	if got := vectorizeEngine("colorMask"); got != "png2svg-clean-node" {
		t.Fatalf("vectorizeEngine()=%q, want png2svg-clean-node", got)
	}
}
