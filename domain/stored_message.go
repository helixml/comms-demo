package domain

// Direction is the flow of a stored message relative to the mock app.
type Direction string

const (
	// DirectionInbound is a message Helix Org POSTed to /in/<channel_id>.
	DirectionInbound Direction = "inbound"
	// DirectionOutbound is a message the operator sent from the mock UI to
	// Helix Org's /webhooks/<stream_id>.
	DirectionOutbound Direction = "outbound"
)

// StoredMessage is one row in the local SQLite store. It carries the wire
// Message plus envelope-level metadata (which channel, which direction, when,
// and any send error encountered).
type StoredMessage struct {
	ID              uint      `gorm:"primaryKey"                       json:"id"`
	ChannelID       string    `gorm:"index;column:channel_id;not null" json:"channel_id"`
	Direction       Direction `gorm:"column:direction;not null"     json:"direction"`
	From            string    `gorm:"column:msg_from"                  json:"from"`
	ToCSV           string    `gorm:"column:msg_to_csv"                json:"to_csv"`
	Subject         string    `gorm:"column:subject"                   json:"subject"`
	Body            string    `gorm:"column:body"                      json:"body"`
	BodyContentType string    `gorm:"column:body_content_type"         json:"body_content_type"`
	ThreadID        string    `gorm:"index;column:thread_id"           json:"thread_id"`
	InReplyTo       string    `gorm:"column:in_reply_to"               json:"in_reply_to"`
	MessageID       string    `gorm:"uniqueIndex;column:message_id"    json:"message_id"`
	AttachmentsJSON string    `gorm:"column:attachments_json"          json:"attachments_json"`
	ExtraJSON       string    `gorm:"column:extra_json"                json:"extra_json"`
	SendError       string    `gorm:"column:send_error"                json:"send_error,omitempty"`
	CreatedAt       int64     `gorm:"column:created_at;autoCreateTime;index" json:"created_at"`
}

// TableName overrides the GORM default to keep the schema name stable.
func (StoredMessage) TableName() string { return "messages" }
