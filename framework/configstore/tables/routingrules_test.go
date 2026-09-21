package tables

import (
	"testing"

	"github.com/bytedance/sonic"
)

// TestRoutingFallback_UnpinnedRoundTripsByteIdentically guards GenerateRoutingRuleHash: a changed byte shape rewrites every rule on the next boot.
func TestRoutingFallback_UnpinnedRoundTripsByteIdentically(t *testing.T) {
	cases := []string{
		`["openai/gpt-4o"]`,
		`["azure/"]`,
		`["anthropic"]`,
		`["openai/ft:gpt-4o:org::abc/v2"]`,
		`["meta-llama/Llama-3.1-8B"]`,
		`[]`,
		`["openai/gpt-4o","azure/","vertex/gemini-2.5-pro"]`,
	}
	for _, input := range cases {
		t.Run(input, func(t *testing.T) {
			var decoded []RoutingFallback
			if err := sonic.Unmarshal([]byte(input), &decoded); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			out, err := sonic.Marshal(decoded)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if string(out) != input {
				t.Fatalf("round-trip changed bytes: got %s, want %s", out, input)
			}
		})
	}
}

// TestRoutingFallback_SplitPreservesParserSemantics keeps Split in step with the request body's own model parsing.
func TestRoutingFallback_SplitPreservesParserSemantics(t *testing.T) {
	cases := []struct {
		input    string
		provider string
		model    string
	}{
		{`"openai/gpt-4o"`, "openai", "gpt-4o"},
		{`"azure/"`, "azure", ""},
		{`"anthropic"`, "", "anthropic"},
		{`"meta-llama/Llama-3.1-8B"`, "", "meta-llama/Llama-3.1-8B"},
		{`{"model":"azure/gpt-4o","key_id":"k1"}`, "azure", "gpt-4o"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			var fb RoutingFallback
			if err := sonic.Unmarshal([]byte(tc.input), &fb); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			provider, model := fb.Split()
			if string(provider) != tc.provider || model != tc.model {
				t.Fatalf("got provider=%q model=%q, want provider=%q model=%q", provider, model, tc.provider, tc.model)
			}
		})
	}
}

// TestRoutingFallback_ObjectFormKeepsKeyID preserves mixed pinned and legacy fallback chains.
func TestRoutingFallback_ObjectFormKeepsKeyID(t *testing.T) {
	input := `["openai/gpt-4o",{"model":"azure/gpt-4o","key_id":"k1"}]`
	var decoded []RoutingFallback
	if err := sonic.Unmarshal([]byte(input), &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(decoded) != 2 {
		t.Fatalf("got %d entries, want 2", len(decoded))
	}
	if decoded[0].IsPinned() {
		t.Fatal("first entry must not be pinned")
	}
	if decoded[1].KeyID != "k1" || !decoded[1].IsPinned() {
		t.Fatalf("second entry lost its key: %+v", decoded[1])
	}
	out, err := sonic.Marshal(decoded)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(out) != input {
		t.Fatalf("mixed round-trip: got %s, want %s", out, input)
	}
}

// TestRoutingFallback_ProgrammaticMarshalsAsString covers entries built in code rather than decoded from the wire.
func TestRoutingFallback_ProgrammaticMarshalsAsString(t *testing.T) {
	out, err := sonic.Marshal([]RoutingFallback{{Model: "openai/gpt-4o"}, {Model: "azure/"}})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(out) != `["openai/gpt-4o","azure/"]` {
		t.Fatalf("got %s", out)
	}
}

// TestRoutingFallback_IncomingModelObjectSurvivesPersistence keeps a pinned incoming-model fallback valid after a save/load cycle.
func TestRoutingFallback_IncomingModelObjectSurvivesPersistence(t *testing.T) {
	var fallback RoutingFallback
	if err := sonic.Unmarshal([]byte(`{"model":"azure/","key_id":"k1"}`), &fallback); err != nil {
		t.Fatal(err)
	}
	encoded, err := sonic.Marshal(fallback)
	if err != nil {
		t.Fatal(err)
	}
	var restored RoutingFallback
	if err := sonic.Unmarshal(encoded, &restored); err != nil {
		t.Fatal(err)
	}
	provider, model := restored.Split()
	if provider != "azure" || model != "" || restored.KeyID != "k1" {
		t.Fatalf("pinned incoming-model fallback changed after persistence: %+v (wire %s)", restored, encoded)
	}
}
