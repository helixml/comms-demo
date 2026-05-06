// Package domain holds the wire types exchanged with Helix Org.
//
// Message must remain JSON-byte-for-byte compatible with the equivalent type
// in helix-org. Do not add wrapper envelopes or version fields here.
package domain

// Attachment represents a file attached to a Message. Content is referenced by
// URL — bytes are not transferred over this channel.
type Attachment struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	URL         string `json:"url"`
	SizeBytes   int64  `json:"size_bytes,omitempty"`
}

// Message is the JSON envelope that Helix Org POSTs out and that the mock app
// POSTs back to simulate inbound replies.
//
// Identity rules:
//   - From / To are transport-native verbatim (email address, Slack U-id, E.164).
//     Worker IDs (w-...) appear when the originator is a Helix Worker.
//   - Empty From means "no human originator" (system feed).
//   - For first messages set ThreadID == MessageID. For replies copy ThreadID
//     from the parent and set InReplyTo to the parent's MessageID.
type Message struct {
	From            string         `json:"from,omitempty"`
	To              []string       `json:"to,omitempty"`
	Subject         string         `json:"subject,omitempty"`
	Body            string         `json:"body"`
	BodyContentType string         `json:"body_content_type,omitempty"`
	ThreadID        string         `json:"thread_id,omitempty"`
	InReplyTo       string         `json:"in_reply_to,omitempty"`
	MessageID       string         `json:"message_id,omitempty"`
	Attachments     []Attachment   `json:"attachments,omitempty"`
	Extra           map[string]any `json:"extra,omitempty"`
}
