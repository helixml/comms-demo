package server

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/helixml/comms-demo/domain"
	"github.com/helixml/comms-demo/helixclient"
	"github.com/helixml/comms-demo/store"
	"github.com/helixml/comms-demo/web"
)

type counterIDGen struct{ n atomic.Uint64 }

func (c *counterIDGen) NewID() string {
	c.n.Add(1)
	return "id-" + strconv.FormatUint(c.n.Load(), 10)
}

type fixedClock time.Time

func (f fixedClock) Now() time.Time { return time.Time(f) }

func newTestServer(t *testing.T, helixBase string) (*Server, *store.Store) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	if err := st.UpsertChannel(&domain.Channel{
		ChannelID:     "email-main",
		Kind:          domain.KindEmail,
		DisplayName:   "Inbox",
		HelixStreamID: "strm-test",
		HelixBaseURL:  helixBase,
		LocalIdentity: "alice@example.com",
	}); err != nil {
		t.Fatalf("seed channel: %v", err)
	}

	srv, err := New(Deps{
		Store:       st,
		Helix:       helixclient.New(nil),
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		TemplatesFS: web.FS,
		IDGen:       &counterIDGen{},
		Clock:       fixedClock(time.Unix(1700000000, 0)),
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	return srv, st
}

func TestServer_Inbound_StoresMessage(t *testing.T) {
	t.Parallel()
	srv, st := newTestServer(t, "http://unused")

	body, _ := json.Marshal(domain.Message{
		From: "bob@example.com", To: []string{"alice@example.com"},
		Subject: "hi", Body: "hello there",
		MessageID: "m1", ThreadID: "m1",
	})
	req := httptest.NewRequest(http.MethodPost, "/in/email-main", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body=%s", rr.Code, rr.Body.String())
	}
	threads, err := st.Threads("email-main")
	if err != nil {
		t.Fatalf("threads: %v", err)
	}
	if len(threads) != 1 {
		t.Fatalf("threads = %d, want 1", len(threads))
	}
	if got := threads[0].Messages[0].Body; got != "hello there" {
		t.Errorf("body = %q, want hello there", got)
	}
}

func TestServer_Inbound_DedupesByMessageID(t *testing.T) {
	t.Parallel()
	srv, st := newTestServer(t, "http://unused")
	body, _ := json.Marshal(domain.Message{Body: "x", MessageID: "dup", ThreadID: "dup"})
	req := func() *http.Request {
		r := httptest.NewRequest(http.MethodPost, "/in/email-main", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		return r
	}
	rr1 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr1, req())
	rr2 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr2, req())

	if rr2.Code != http.StatusNoContent {
		t.Fatalf("dup status = %d, want 204", rr2.Code)
	}
	msgs, _ := st.MessagesForChannel("email-main")
	if len(msgs) != 1 {
		t.Errorf("messages = %d, want 1 (deduped)", len(msgs))
	}
}

func TestServer_Inbound_UnknownChannel_404(t *testing.T) {
	t.Parallel()
	srv, _ := newTestServer(t, "http://unused")
	req := httptest.NewRequest(http.MethodPost, "/in/no-such", strings.NewReader(`{"body":"x"}`))
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
}

func TestServer_Reply_PostsToHelixAndStoresOutbound(t *testing.T) {
	t.Parallel()

	var (
		gotPath string
		gotBody []byte
	)
	helix := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(helix.Close)

	srv, st := newTestServer(t, helix.URL)

	// seed an inbound message we'll reply to
	if _, err := st.AppendMessage("email-main", domain.DirectionInbound, &domain.Message{
		From: "bob@example.com", To: []string{"alice@example.com"},
		Subject: "hi", Body: "hello",
		MessageID: "parent", ThreadID: "parent",
	}, ""); err != nil {
		t.Fatalf("seed inbound: %v", err)
	}

	form := strings.NewReader("thread_id=parent&in_reply_to=parent&to=bob@example.com&subject=Re: hi&body=howdy")
	req := httptest.NewRequest(http.MethodPost, "/channel/email-main/reply", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303; body=%s", rr.Code, rr.Body.String())
	}
	if gotPath != "/webhooks/strm-test" {
		t.Errorf("helix path = %q, want /webhooks/strm-test", gotPath)
	}

	var sent domain.Message
	if err := json.Unmarshal(gotBody, &sent); err != nil {
		t.Fatalf("decode helix body: %v", err)
	}
	if sent.ThreadID != "parent" || sent.InReplyTo != "parent" {
		t.Errorf("threading = %+v, want thread_id=parent in_reply_to=parent", sent)
	}
	if sent.From != "alice@example.com" {
		t.Errorf("from = %q, want alice@example.com", sent.From)
	}
	if sent.Body != "howdy" {
		t.Errorf("body = %q, want howdy", sent.Body)
	}
	if sent.MessageID == "" {
		t.Error("message_id empty, expected fresh id")
	}
}

func TestServer_Compose_NewThreadHasMatchingIDs(t *testing.T) {
	t.Parallel()

	helix := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(helix.Close)

	srv, _ := newTestServer(t, helix.URL)

	form := strings.NewReader("to=bob@example.com&subject=hi&body=hello")
	req := httptest.NewRequest(http.MethodPost, "/channel/email-main/compose", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rr.Code)
	}
	loc := rr.Header().Get("Location")
	if !strings.HasPrefix(loc, "/channel/email-main/thread/") {
		t.Errorf("redirect = %q, want /channel/email-main/thread/...", loc)
	}
}

func TestServer_Healthz(t *testing.T) {
	t.Parallel()
	srv, _ := newTestServer(t, "http://unused")
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d", rr.Code)
	}
	if rr.Body.String() != "ok" {
		t.Errorf("body = %q", rr.Body.String())
	}
}

func TestServer_RendersIndexAndAdmin(t *testing.T) {
	t.Parallel()
	srv, _ := newTestServer(t, "http://unused")
	for _, path := range []string{"/", "/admin", "/channel/email-main"} {
		rr := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, path, nil))
		if rr.Code != http.StatusOK {
			t.Errorf("GET %s = %d, body=%s", path, rr.Code, rr.Body.String())
		}
	}
}
