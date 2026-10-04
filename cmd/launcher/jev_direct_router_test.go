package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestJevDirectEvaluateChoosesTypeSafeRoute(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer tsf_test_key" {
			t.Fatalf("authorization = %q", got)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["model"] != "jev-latest" {
			t.Fatalf("model = %#v", body["model"])
		}
		questions, _ := body["questions"].(map[string]any)
		if len(questions) != 5 {
			t.Fatalf("questions = %#v", questions)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"model":"jev-1.13.0",
			"answers":{
				"route":{"type":"choice","choice":"m002","confidence":0.91},
				"difficulty":{"type":"score","score":2},
				"domain":{"type":"choice","choice":"code"},
				"needs_tools":{"type":"noul","noul":0.95},
				"is_sensitive":{"type":"noul","noul":0.10}
			},
			"usage":{"input_tokens":123,"output_tokens":4}
		}`))
	}))
	defer server.Close()

	service := &jevDirectRouterService{
		client: server.Client(),
		endpoint: server.URL,
		model: jevDirectModel,
	}
	candidates := []layaRouterCandidate{
		{ProviderID:"cheap", ProviderName:"Cheap", ModelID:"mini", ModelName:"Mini", Connected:true, Ready:true, Enabled:true, Group:"budget", Quality:3, Speed:5},
		{ProviderID:"strong", ProviderName:"Strong", ModelID:"pro", ModelName:"Pro", Connected:true, Ready:true, Enabled:true, Group:"standard", Quality:5, Speed:3},
	}
	selection, err := service.evaluate(context.Background(), "tsf_test_key", "inspect and fix the repository", candidates)
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatalf("JEV Direct calls = %d, want 1", calls.Load())
	}
	if selection.ProviderID != "strong" || selection.ModelID != "pro" {
		t.Fatalf("unexpected selection %#v", selection)
	}
	if selection.RouterProviderID != jevDirectRouterProviderID || selection.RouterName != "JEV Direct" ||
		selection.RouterSource != "typesafe-system-one" {
		t.Fatalf("missing direct-router metadata %#v", selection)
	}
	if selection.Analysis.Domain != "code" || selection.Analysis.Difficulty != 2 ||
		selection.Analysis.NeedsTools != 0.95 || selection.Analysis.Sensitive != 0.10 {
		t.Fatalf("unexpected analysis %#v", selection.Analysis)
	}
	if selection.Analysis.Checkpoint != "jev-1.13.0" || !strings.Contains(selection.Reason, "confidence 0.91") {
		t.Fatalf("unexpected JEV metadata %#v", selection)
	}
}

func TestJevDirectCatalogIsIndependentFromOpenRouter(t *testing.T) {
	entry, ok := pluginCatalogEntryByID(jevDirectPluginID)
	if !ok {
		t.Fatal("JEV Direct catalog entry missing")
	}
	if entry.Metadata["endpoint"] != jevDirectEndpoint {
		t.Fatalf("endpoint = %q", entry.Metadata["endpoint"])
	}
	encoded, err := json.Marshal(entry)
	if err != nil { t.Fatal(err) }
	if strings.Contains(strings.ToLower(string(encoded)), "openrouter") {
		t.Fatalf("direct JEV catalog must not depend on OpenRouter: %s", encoded)
	}
}
