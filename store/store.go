// Package store wraps a GORM SQLite handle. AutoMigrate only — no migration
// files. Schema mirrors domain.Channel and domain.StoredMessage.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/helixml/comms-demo/domain"
)

// Store is the persistence boundary. Constructed via Open.
type Store struct {
	db *gorm.DB
}

// Open opens the SQLite database at path and runs AutoMigrate.
func Open(path string) (*Store, error) {
	if path == "" {
		return nil, errors.New("store: empty path")
	}
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("store: open %s: %w", path, err)
	}
	if err := db.AutoMigrate(&domain.Channel{}, &domain.StoredMessage{}); err != nil {
		return nil, fmt.Errorf("store: automigrate: %w", err)
	}
	return &Store{db: db}, nil
}

// Close releases the underlying SQL handle.
func (s *Store) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return fmt.Errorf("store: close: %w", err)
	}
	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("store: close: %w", err)
	}
	return nil
}

// UpsertChannel inserts or updates a Channel keyed by ChannelID.
func (s *Store) UpsertChannel(c *domain.Channel) error {
	if c == nil {
		return errors.New("store: nil channel")
	}
	if c.ChannelID == "" {
		return errors.New("store: channel_id required")
	}
	if !c.Kind.Valid() {
		return fmt.Errorf("store: invalid channel kind %q", c.Kind)
	}
	if err := s.db.Save(c).Error; err != nil {
		return fmt.Errorf("store: upsert channel: %w", err)
	}
	return nil
}

// Channels returns every configured channel ordered by ChannelID.
func (s *Store) Channels() ([]domain.Channel, error) {
	var out []domain.Channel
	if err := s.db.Order("channel_id asc").Find(&out).Error; err != nil {
		return nil, fmt.Errorf("store: list channels: %w", err)
	}
	return out, nil
}

// Channel returns a single channel by ID, or wrapped gorm.ErrRecordNotFound.
func (s *Store) Channel(id string) (*domain.Channel, error) {
	var out domain.Channel
	if err := s.db.First(&out, "channel_id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("store: get channel %q: %w", id, err)
	}
	return &out, nil
}

// AppendMessage stores a wire Message plus its routing metadata.
func (s *Store) AppendMessage(channelID string, dir domain.Direction, m *domain.Message, sendErr string) (*domain.StoredMessage, error) {
	if m == nil {
		return nil, errors.New("store: nil message")
	}
	if channelID == "" {
		return nil, errors.New("store: channel_id required")
	}

	attBytes, err := json.Marshal(m.Attachments)
	if err != nil {
		return nil, fmt.Errorf("store: marshal attachments: %w", err)
	}
	extraBytes, err := json.Marshal(m.Extra)
	if err != nil {
		return nil, fmt.Errorf("store: marshal extra: %w", err)
	}

	row := &domain.StoredMessage{
		ChannelID:       channelID,
		Direction:       dir,
		From:            m.From,
		ToCSV:           strings.Join(m.To, ","),
		Subject:         m.Subject,
		Body:            m.Body,
		BodyContentType: m.BodyContentType,
		ThreadID:        m.ThreadID,
		InReplyTo:       m.InReplyTo,
		MessageID:       m.MessageID,
		AttachmentsJSON: string(attBytes),
		ExtraJSON:       string(extraBytes),
		SendError:       sendErr,
	}
	if err := s.db.Create(row).Error; err != nil {
		return nil, fmt.Errorf("store: append message: %w", err)
	}
	return row, nil
}

// MessagesForChannel returns every message in a channel ordered oldest-first.
func (s *Store) MessagesForChannel(channelID string) ([]domain.StoredMessage, error) {
	var out []domain.StoredMessage
	if err := s.db.
		Where("channel_id = ?", channelID).
		Order("created_at asc, id asc").
		Find(&out).Error; err != nil {
		return nil, fmt.Errorf("store: list messages: %w", err)
	}
	return out, nil
}

// Threads collapses the channel's messages into one bucket per thread_id, with
// the latest message metadata kept on top. Threads are ordered by their most
// recent activity (newest first).
func (s *Store) Threads(channelID string) ([]ThreadSummary, error) {
	msgs, err := s.MessagesForChannel(channelID)
	if err != nil {
		return nil, err
	}
	byThread := make(map[string]*ThreadSummary)
	order := make([]string, 0)
	for i := range msgs {
		m := msgs[i]
		key := m.ThreadID
		if key == "" {
			key = m.MessageID
		}
		if key == "" {
			key = fmt.Sprintf("orphan-%d", m.ID)
		}
		t, ok := byThread[key]
		if !ok {
			t = &ThreadSummary{ThreadID: key}
			byThread[key] = t
			order = append(order, key)
		}
		t.Messages = append(t.Messages, m)
		t.LastAt = m.CreatedAt
		t.LastFrom = m.From
		t.LastBody = m.Body
		if m.Subject != "" {
			t.Subject = m.Subject
		}
	}
	out := make([]ThreadSummary, 0, len(order))
	for _, k := range order {
		out = append(out, *byThread[k])
	}
	// newest first
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

// Thread returns a single thread by ID for the given channel.
func (s *Store) Thread(channelID, threadID string) (*ThreadSummary, error) {
	all, err := s.Threads(channelID)
	if err != nil {
		return nil, err
	}
	for i := range all {
		if all[i].ThreadID == threadID {
			return &all[i], nil
		}
	}
	return nil, fmt.Errorf("store: thread %q not found in channel %q", threadID, channelID)
}

// MessageIDExists reports whether a message_id has already been stored. Used
// to dedupe Helix Org retries (defensive — Helix doesn't retry today).
func (s *Store) MessageIDExists(messageID string) (bool, error) {
	if messageID == "" {
		return false, nil
	}
	var n int64
	if err := s.db.
		Model(&domain.StoredMessage{}).
		Where("message_id = ?", messageID).
		Count(&n).Error; err != nil {
		return false, fmt.Errorf("store: count message_id: %w", err)
	}
	return n > 0, nil
}

// Reset truncates every table. Used by `mock-channels reset`.
func (s *Store) Reset() error {
	if err := s.db.Exec("DELETE FROM messages").Error; err != nil {
		return fmt.Errorf("store: reset messages: %w", err)
	}
	if err := s.db.Exec("DELETE FROM channels").Error; err != nil {
		return fmt.Errorf("store: reset channels: %w", err)
	}
	return nil
}

// ThreadSummary aggregates one thread's messages with a header for list views.
type ThreadSummary struct {
	ThreadID string
	Subject  string
	LastAt   int64
	LastFrom string
	LastBody string
	Messages []domain.StoredMessage
}
