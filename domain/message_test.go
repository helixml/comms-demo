package domain

import (
	"encoding/json"
	"testing"
)

// TestMessage_RoundTrip is the contract test: the JSON envelope captured from a
// real Helix Org outbound POST must decode and re-encode without losing fields.
//
// Update this fixture whenever helix-org/domain/message.go changes shape.
func TestMessage_RoundTrip(t *testing.T) {
	t.Parallel()

	// fixture captured from a real Helix Org outbound POST
	fixture := []byte(`{
  "from": "w-sam",
  "to": ["alice@example.com"],
  "subject": "Re: invoice",
  "body": "Hi Alice, attached is the revised invoice.",
  "body_content_type": "text/plain",
  "thread_id": "thr-1",
  "in_reply_to": "msg-prev",
  "message_id": "msg-this",
  "attachments": [
    {"filename":"invoice.pdf","content_type":"application/pdf","url":"https://example.com/inv.pdf","size_bytes":12345}
  ],
  "extra": {"org_id": "abc"}
}`)

	var m Message
	if err := json.Unmarshal(fixture, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got, want := m.From, "w-sam"; got != want {
		t.Errorf("From = %q, want %q", got, want)
	}
	if got, want := len(m.To), 1; got != want {
		t.Fatalf("To len = %d, want %d", got, want)
	}
	if got, want := m.To[0], "alice@example.com"; got != want {
		t.Errorf("To[0] = %q, want %q", got, want)
	}
	if got, want := m.ThreadID, "thr-1"; got != want {
		t.Errorf("ThreadID = %q, want %q", got, want)
	}
	if got, want := m.InReplyTo, "msg-prev"; got != want {
		t.Errorf("InReplyTo = %q, want %q", got, want)
	}
	if got, want := m.MessageID, "msg-this"; got != want {
		t.Errorf("MessageID = %q, want %q", got, want)
	}
	if got, want := len(m.Attachments), 1; got != want {
		t.Fatalf("Attachments len = %d, want %d", got, want)
	}
	if got, want := m.Attachments[0].SizeBytes, int64(12345); got != want {
		t.Errorf("Attachments[0].SizeBytes = %d, want %d", got, want)
	}
	if got, want := m.Extra["org_id"], any("abc"); got != want {
		t.Errorf("Extra[org_id] = %v, want %v", got, want)
	}

	// re-encode and ensure the spec-required fields are present
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back map[string]any
	if err := json.Unmarshal(out, &back); err != nil {
		t.Fatalf("re-decode: %v", err)
	}
	for _, key := range []string{"from", "to", "subject", "body", "body_content_type", "thread_id", "in_reply_to", "message_id", "attachments", "extra"} {
		if _, ok := back[key]; !ok {
			t.Errorf("re-encoded JSON missing key %q", key)
		}
	}
}

func TestMessage_OnlyBodyRequired(t *testing.T) {
	t.Parallel()
	in := []byte(`{"body":"hello"}`)
	var m Message
	if err := json.Unmarshal(in, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m.Body != "hello" {
		t.Errorf("Body = %q, want %q", m.Body, "hello")
	}
}

func TestChannelKind_Valid(t *testing.T) {
	t.Parallel()
	cases := []struct {
		k    ChannelKind
		want bool
	}{
		{KindEmail, true},
		{KindSlack, true},
		{KindSMS, true},
		{ChannelKind(""), false},
		{ChannelKind("postal"), false},
	}
	for _, c := range cases {
		if got := c.k.Valid(); got != c.want {
			t.Errorf("ChannelKind(%q).Valid() = %v, want %v", c.k, got, c.want)
		}
	}
}
