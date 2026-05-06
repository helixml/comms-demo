package store

import (
	"path/filepath"
	"testing"

	"github.com/helixml/comms-demo/domain"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestStore_UpsertAndListChannels(t *testing.T) {
	t.Parallel()
	s := newStore(t)

	c := &domain.Channel{
		ChannelID:   "email-main",
		Kind:        domain.KindEmail,
		DisplayName: "Inbox",
	}
	if err := s.UpsertChannel(c); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	list, err := s.Channels()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || list[0].ChannelID != "email-main" {
		t.Fatalf("list = %+v, want one email-main", list)
	}

	// update with same id changes display name
	c.DisplayName = "Updated"
	if err := s.UpsertChannel(c); err != nil {
		t.Fatalf("re-upsert: %v", err)
	}
	got, err := s.Channel("email-main")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.DisplayName != "Updated" {
		t.Errorf("DisplayName = %q, want %q", got.DisplayName, "Updated")
	}
}

func TestStore_RejectsInvalidChannelKind(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	err := s.UpsertChannel(&domain.Channel{ChannelID: "x", Kind: "postal"})
	if err == nil {
		t.Fatal("expected error for invalid kind")
	}
}

func TestStore_AppendAndThreads(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	if err := s.UpsertChannel(&domain.Channel{
		ChannelID: "ch", Kind: domain.KindEmail, DisplayName: "x",
	}); err != nil {
		t.Fatal(err)
	}

	// inbound: open new thread
	root := &domain.Message{
		From: "alice@example.com", To: []string{"bob@example.com"},
		Subject: "hi", Body: "hello",
		MessageID: "msg-1", ThreadID: "msg-1",
	}
	if _, err := s.AppendMessage("ch", domain.DirectionInbound, root, ""); err != nil {
		t.Fatalf("append root: %v", err)
	}

	// reply: same thread
	reply := &domain.Message{
		From: "bob@example.com", To: []string{"alice@example.com"},
		Subject: "Re: hi", Body: "hi back",
		MessageID: "msg-2", ThreadID: "msg-1", InReplyTo: "msg-1",
	}
	if _, err := s.AppendMessage("ch", domain.DirectionOutbound, reply, ""); err != nil {
		t.Fatalf("append reply: %v", err)
	}

	threads, err := s.Threads("ch")
	if err != nil {
		t.Fatal(err)
	}
	if len(threads) != 1 {
		t.Fatalf("threads = %d, want 1", len(threads))
	}
	if got := threads[0].ThreadID; got != "msg-1" {
		t.Errorf("thread id = %q, want msg-1", got)
	}
	if got := len(threads[0].Messages); got != 2 {
		t.Errorf("thread messages = %d, want 2", got)
	}
}

func TestStore_MessageIDExists(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	if err := s.UpsertChannel(&domain.Channel{
		ChannelID: "ch", Kind: domain.KindSMS, DisplayName: "x",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AppendMessage("ch", domain.DirectionInbound, &domain.Message{
		Body: "hi", MessageID: "abc", ThreadID: "abc",
	}, ""); err != nil {
		t.Fatal(err)
	}
	exists, err := s.MessageIDExists("abc")
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Error("exists = false, want true")
	}
	exists, err = s.MessageIDExists("missing")
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Error("exists = true, want false")
	}
}

func TestStore_Reset(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	if err := s.UpsertChannel(&domain.Channel{
		ChannelID: "ch", Kind: domain.KindSMS, DisplayName: "x",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AppendMessage("ch", domain.DirectionInbound, &domain.Message{
		Body: "hi", MessageID: "abc", ThreadID: "abc",
	}, ""); err != nil {
		t.Fatal(err)
	}
	if err := s.Reset(); err != nil {
		t.Fatal(err)
	}
	chs, _ := s.Channels()
	if len(chs) != 0 {
		t.Errorf("channels after reset = %d, want 0", len(chs))
	}
}
