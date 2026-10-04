package main

import (
	"errors"
	"testing"
)

func TestLayaRouterTargetQualityUsesTaskSignals(t *testing.T) {
	cases := []struct {
		name     string
		analysis layaRouteAnalysis
		wantMin  int
	}{
		{name: "trivial", analysis: layaRouteAnalysis{Difficulty: 0.2, Domain: "factual_lookup"}, wantMin: 2},
		{name: "moderate code", analysis: layaRouteAnalysis{Difficulty: 1.8, Domain: "code"}, wantMin: 5},
		{name: "sensitive", analysis: layaRouteAnalysis{Difficulty: 0.5, Domain: "writing", Sensitive: 0.8}, wantMin: 4},
		{name: "tools", analysis: layaRouteAnalysis{Difficulty: 0.4, Domain: "factual_lookup", NeedsTools: 0.9}, wantMin: 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := layaTargetQuality(tc.analysis); got < tc.wantMin {
				t.Fatalf("target quality %d is below expected floor %d for %#v", got, tc.wantMin, tc.analysis)
			}
		})
	}
}

func TestLayaRouterFreeProfileNeverFallsBackToPaid(t *testing.T) {
	_, _, err := chooseLayaCandidate("free", layaRouteAnalysis{Difficulty: 0.2}, []layaRouterCandidate{
		{ProviderID: "paid", ModelID: "fast", Connected: true, Enabled: true, Group: "budget", Quality: 4, Speed: 5},
	})
	if err == nil {
		t.Fatal("free profile must fail when no enabled connected Free model exists")
	}

	selected, _, err := chooseLayaCandidate("free", layaRouteAnalysis{Difficulty: 0.2}, []layaRouterCandidate{
		{ProviderID: "paid", ModelID: "fast", Connected: true, Enabled: true, Group: "budget", Quality: 5, Speed: 5},
		{ProviderID: "free", ModelID: "free-good", Connected: true, Enabled: true, Group: "free", Quality: 3, Speed: 3},
	})
	if err != nil {
		t.Fatal(err)
	}
	if selected.ProviderID != "free" || selected.ModelID != "free-good" {
		t.Fatalf("free profile selected a non-free model: %#v", selected)
	}
}

func TestLayaRouterCostProfileUsesCheapestQualifyingGroup(t *testing.T) {
	analysis := layaRouteAnalysis{Difficulty: 1.0, Domain: "writing"}
	selected, _, err := chooseLayaCandidate("cost", analysis, []layaRouterCandidate{
		{ProviderID: "premium", ModelID: "frontier", Connected: true, Enabled: true, Group: "premium", Quality: 5, Speed: 3},
		{ProviderID: "budget", ModelID: "small", Connected: true, Enabled: true, Group: "budget", Quality: 3, Speed: 5},
		{ProviderID: "standard", ModelID: "mid", Connected: true, Enabled: true, Group: "standard", Quality: 4, Speed: 4},
	})
	if err != nil {
		t.Fatal(err)
	}
	if selected.ModelID != "small" {
		t.Fatalf("cost profile should select cheapest qualifying model, got %#v", selected)
	}
}

func TestLayaRouterQualityProfilePrefersStrongestEligibleModel(t *testing.T) {
	selected, _, err := chooseLayaCandidate("quality", layaRouteAnalysis{Difficulty: 1.0}, []layaRouterCandidate{
		{ProviderID: "a", ModelID: "mid", Connected: true, Enabled: true, Group: "budget", Quality: 3, Speed: 5},
		{ProviderID: "b", ModelID: "frontier", Connected: true, Enabled: true, Group: "premium", Quality: 5, Speed: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if selected.ModelID != "frontier" {
		t.Fatalf("quality profile did not choose highest quality model: %#v", selected)
	}
}

func TestLayaRouterIgnoresDisabledAndDisconnectedModels(t *testing.T) {
	selected, _, err := chooseLayaCandidate("balanced", layaRouteAnalysis{Difficulty: 0.3}, []layaRouterCandidate{
		{ProviderID: "a", ModelID: "disabled", Connected: true, Enabled: false, Group: "free", Quality: 5, Speed: 5},
		{ProviderID: "b", ModelID: "offline", Connected: false, Enabled: true, Group: "free", Quality: 5, Speed: 5},
		{ProviderID: "c", ModelID: "ready", Connected: true, Enabled: true, Group: "standard", Quality: 2, Speed: 3},
	})
	if err != nil {
		t.Fatal(err)
	}
	if selected.ModelID != "ready" {
		t.Fatalf("router selected disabled/offline model: %#v", selected)
	}
}

func TestLayaRouterParsesModelRouterPresetOutput(t *testing.T) {
	payload := map[string]any{
		"answers": map[string]any{
			"difficulty": map[string]any{"score": 2.25},
			"domain": map[string]any{"choice": "code"},
			"needs_tools": map[string]any{"noul": 0.81},
			"is_sensitive": map[string]any{"noul": 0.12},
		},
		"routing": map[string]any{"model": "english", "reason": "latin script"},
		"latency_ms": 17.4,
	}
	got, err := layaAnalyzePayload(payload)
	if err != nil {
		t.Fatal(err)
	}
	if got.Difficulty != 2.25 || got.Domain != "code" || got.NeedsTools != 0.81 || got.Sensitive != 0.12 {
		t.Fatalf("unexpected Laya analysis %#v", got)
	}
	if got.Checkpoint != "english" || got.LayaReason != "latin script" || got.LatencyMS != 17.4 {
		t.Fatalf("routing metadata was not preserved: %#v", got)
	}
}

func TestLayaRouterDefaultTraitsRecognizeFreeAndIncludedModels(t *testing.T) {
	group, _, _ := layaDefaultTraits(
		tlProviderDefinition{ID: "openrouter", Protocol: "openai-compatible"},
		tlProviderModel{ID: "vendor/model:free", Name: "Model Free"},
	)
	if group != "free" {
		t.Fatalf("expected OpenRouter-style free model classification, got %q", group)
	}

	group, _, _ = layaDefaultTraits(
		tlProviderDefinition{ID: "chatgpt", Protocol: "codex-chatgpt"},
		tlProviderModel{ID: "gpt-6-luna", Name: "GPT-6 Luna"},
	)
	if group != "included" {
		t.Fatalf("account-backed model should default to included quota, got %q", group)
	}
}


func TestLayaToolPayloadPrefersCanonicalTextOverStructuredWrapper(t *testing.T) {
	output := map[string]any{
		"structuredContent": map[string]any{"result": "wrapper metadata"},
		"content": []map[string]any{{
			"text": `{"answers":{"difficulty":{"score":1.5}},"routing":{"model":"english"}}`,
		}},
	}
	payload, err := layaToolPayload(output)
	if err != nil {
		t.Fatal(err)
	}
	answers := mapFromAny(payload["answers"])
	if answers == nil || mapFromAny(answers["difficulty"]) == nil {
		t.Fatalf("canonical Laya content text was not decoded: %#v", payload)
	}
}

func TestLayaToolPayloadUnwrapsStructuredStringResult(t *testing.T) {
	output := map[string]any{
		"structuredContent": map[string]any{
			"result": `{"answers":{"domain":{"choice":"code"}},"latency_ms":3.2}`,
		},
	}
	payload, err := layaToolPayload(output)
	if err != nil {
		t.Fatal(err)
	}
	answers := mapFromAny(payload["answers"])
	if answers == nil || mapFromAny(answers["domain"]) == nil {
		t.Fatalf("structured string result was not unwrapped: %#v", payload)
	}
}


func TestLayaRouteFailureClassification(t *testing.T) {
	cases := []struct {
		name         string
		message      string
		providerWide bool
		ok           bool
	}{
		{name: "rate limit", message: "model request failed with status 429: rate-limited", providerWide: false, ok: true},
		{name: "model unavailable", message: "model_not_available", providerWide: false, ok: true},
		{name: "bridge pairing", message: "Claude Web bridge is not paired", providerWide: true, ok: true},
		{name: "ordinary validation", message: "model returned an empty response", providerWide: false, ok: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			providerWide, cooldown, _, ok := classifyLayaRouteFailure(errors.New(tc.message))
			if ok != tc.ok || providerWide != tc.providerWide {
				t.Fatalf("unexpected classification: providerWide=%v ok=%v", providerWide, ok)
			}
			if ok && cooldown <= 0 {
				t.Fatalf("reroutable failure must receive a cooldown")
			}
		})
	}
}

func TestLayaRouterSkipsCandidateThatIsNotReady(t *testing.T) {
	selected, _, err := chooseLayaCandidate("balanced", layaRouteAnalysis{Difficulty: 1.0}, []layaRouterCandidate{
		{ProviderID: "a", ModelID: "unavailable", Connected: true, Ready: false, Availability: "Rate limited", Enabled: true, Group: "free", Quality: 5, Speed: 5},
		{ProviderID: "b", ModelID: "ready", Connected: true, Ready: true, Availability: "Ready", Enabled: true, Group: "budget", Quality: 3, Speed: 3},
	})
	if err != nil {
		t.Fatal(err)
	}
	if selected.ModelID != "ready" {
		t.Fatalf("router selected an unavailable model: %#v", selected)
	}
}
