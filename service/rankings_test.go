package service

import "testing"

func TestModelMetaFallsBackToDefaultVendor(t *testing.T) {
	meta := modelMeta("gpt-5.6-sol", map[string]rankingModelMeta{})
	if meta.vendor != "OpenAI" {
		t.Fatalf("expected OpenAI vendor, got %q", meta.vendor)
	}
}

func TestModelMetaPrefersStoredVendor(t *testing.T) {
	meta := modelMeta("gpt-5.6-sol", map[string]rankingModelMeta{
		"gpt-5.6-sol": {vendor: "Internal Relay", vendorIcon: "relay"},
	})
	if meta.vendor != "Internal Relay" || meta.vendorIcon != "relay" {
		t.Fatalf("expected stored vendor metadata, got %#v", meta)
	}
}
