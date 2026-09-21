package schemas

import "testing"

func TestTraceGetSpanNilSafe(t *testing.T) {
	var nilTrace *Trace
	if span := nilTrace.GetSpan("span"); span != nil {
		t.Fatalf("nil trace GetSpan returned %v, want nil", span)
	}

	trace := &Trace{Spans: []*Span{nil, &Span{SpanID: "target"}}}
	if span := trace.GetSpan(""); span != nil {
		t.Fatalf("empty span ID = %v, want nil", span)
	}
	if span := trace.GetSpan("missing"); span != nil {
		t.Fatalf("missing span = %v, want nil", span)
	}
	if span := trace.GetSpan("target"); span == nil || span.SpanID != "target" {
		t.Fatalf("target span = %v, want target", span)
	}
}

func TestTraceAndSpanNilMutatorsNoop(t *testing.T) {
	trace := &Trace{}
	trace.AddSpan(nil)
	if len(trace.Spans) != 0 {
		t.Fatalf("nil span was appended: %v", trace.Spans)
	}

	var nilTrace *Trace
	nilTrace.AddSpan(&Span{SpanID: "ignored"})

	var nilSpan *Span
	nilSpan.SetAttribute("key", "value")
	nilSpan.AddEvent(SpanEvent{Name: "event"})
	nilSpan.End(SpanStatusOk, "")
}

func sptr(s string) *string { return &s }

// TestAttachmentSummaryBoundsInlinePayloads verifies that a data: URL becomes a
// sized reference by default and is only copied when the caller opts in.
func TestAttachmentSummaryBoundsInlinePayloads(t *testing.T) {
	msg := ChatMessage{
		Role: ChatMessageRoleUser,
		Content: &ChatMessageContent{ContentBlocks: []ChatContentBlock{
			{Type: ChatContentBlockTypeText, Text: sptr("what is this")},
			{Type: ChatContentBlockTypeImage, ImageURLStruct: &ChatInputImage{
				URL: "data:image/png;base64,aGVsbG93b3JsZA==", Detail: sptr("high"),
			}},
			{Type: ChatContentBlockTypeImage, ImageURLStruct: &ChatInputImage{
				URL: "https://example.com/cat.png",
			}},
		}},
	}

	got := ExtractMessageSummary(&msg, AttachmentOptions{})
	if got.Content != "what is this" {
		t.Errorf("Content = %q, want text blocks only", got.Content)
	}
	if len(got.Attachments) != 2 {
		t.Fatalf("Attachments = %d, want 2", len(got.Attachments))
	}

	inline := got.Attachments[0]
	if !inline.Inline || inline.Data != "" {
		t.Errorf("inline attachment Data = %q, want empty by default", inline.Data)
	}
	if inline.MediaType != "image/png" || inline.ByteSize == 0 || inline.Detail != "high" {
		t.Errorf("inline attachment = %#v, want media type, size and detail", inline)
	}

	ref := got.Attachments[1]
	if ref.Inline || ref.URL != "https://example.com/cat.png" {
		t.Errorf("reference attachment = %#v, want the URL carried verbatim", ref)
	}

	opted := ExtractMessageSummary(&msg, AttachmentOptions{Inline: true})
	if opted.Attachments[0].Data == "" {
		t.Error("opted-in attachment Data is empty, want the payload copied")
	}

	capped := ExtractMessageSummary(&msg, AttachmentOptions{Inline: true, Cap: 4})
	if capped.Attachments[0].Data != "" {
		t.Error("attachment over cap was copied, want size only")
	}
}

func TestTraceSessionIDAccessor(t *testing.T) {
	if got := TraceSessionID(nil); got != "" {
		t.Errorf("TraceSessionID(nil) = %q, want empty", got)
	}
	if got := TraceSessionID(map[string]any{TraceAttrSessionID: "sess-1"}); got != "sess-1" {
		t.Errorf("TraceSessionID = %q, want sess-1", got)
	}
	// Wrong type must not panic.
	if got := TraceSessionID(map[string]any{TraceAttrSessionID: 42}); got != "" {
		t.Errorf("TraceSessionID(int) = %q, want empty", got)
	}
}

func TestTraceDimensionsAccessor(t *testing.T) {
	if got := TraceDimensions(nil); got != nil {
		t.Errorf("TraceDimensions(nil) = %v, want nil", got)
	}
	typed := TraceDimensions(map[string]any{
		TraceAttrDimensions: map[string]string{"team": "platform"},
	})
	if typed["team"] != "platform" {
		t.Errorf("typed = %v, want team=platform", typed)
	}
	// Decoded from JSON: map[string]any.
	decoded := TraceDimensions(map[string]any{
		TraceAttrDimensions: map[string]any{"team": "platform", "n": 1},
	})
	if decoded["team"] != "platform" {
		t.Errorf("decoded = %v, want team=platform", decoded)
	}
	if _, ok := decoded["n"]; ok {
		t.Errorf("non-string value should be skipped, got %v", decoded)
	}
	// Wrong type must not panic.
	if got := TraceDimensions(map[string]any{TraceAttrDimensions: "nope"}); got != nil {
		t.Errorf("TraceDimensions(string) = %v, want nil", got)
	}
}
