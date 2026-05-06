package domain

// ChannelKind identifies one of the three mocked transport idioms.
type ChannelKind string

// The three concrete channel kinds the mock app speaks.
const (
	KindEmail ChannelKind = "email"
	KindSlack ChannelKind = "slack"
	KindSMS   ChannelKind = "sms"
)

// Valid reports whether k is one of the three known channel kinds.
func (k ChannelKind) Valid() bool {
	switch k {
	case KindEmail, KindSlack, KindSMS:
		return true
	}
	return false
}

// Channel is a single mocked transport endpoint backed by one Helix Org
// Stream. ChannelID is the path segment used in /in/<channel_id>; it is also
// the local-side identity. HelixStreamID is the remote Stream ID used in
// /webhooks/<stream_id> on Helix Org.
type Channel struct {
	ChannelID     string      `gorm:"primaryKey;column:channel_id" json:"channel_id"`
	Kind          ChannelKind `gorm:"column:kind;not null"           json:"kind"`
	DisplayName   string      `gorm:"column:display_name"            json:"display_name"`
	HelixStreamID string      `gorm:"column:helix_stream_id"         json:"helix_stream_id"`
	HelixBaseURL  string      `gorm:"column:helix_base_url"          json:"helix_base_url"`
	LocalIdentity string      `gorm:"column:local_identity"          json:"local_identity"`
	CreatedAt     int64       `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt     int64       `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

// TableName overrides the GORM default to keep the schema name stable.
func (Channel) TableName() string { return "channels" }
