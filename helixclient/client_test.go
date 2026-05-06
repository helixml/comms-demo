package helixclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/helixml/comms-demo/domain"
)

func TestClient_Send_PostsToWebhooksPath(t *testing.T) {
	t.Parallel()

	var (
		gotPath        string
		gotContentType string
		gotBody        []byte
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotContentType = r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.Client())
	msg := &domain.Message{Body: "hi", MessageID: "m1", ThreadID: "m1"}
	if err := c.Send(context.Background(), srv.URL, "stream-xyz", msg); err != nil {
		t.Fatalf("send: %v", err)
	}

	if gotPath != "/webhooks/stream-xyz" {
		t.Errorf("path = %q, want /webhooks/stream-xyz", gotPath)
	}
	if !strings.HasPrefix(gotContentType, "application/json") {
		t.Errorf("content-type = %q, want application/json", gotContentType)
	}
	var back domain.Message
	if err := json.Unmarshal(gotBody, &back); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if back.Body != "hi" || back.MessageID != "m1" {
		t.Errorf("decoded = %+v", back)
	}
}

func TestClient_Send_ReturnsErrOnNon2xx(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	t.Cleanup(srv.Close)

	c := New(srv.Client())
	err := c.Send(context.Background(), srv.URL, "s", &domain.Message{Body: "x"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("err = %v, want includes 500", err)
	}
}

func TestClient_Send_RejectsEmptyParams(t *testing.T) {
	t.Parallel()
	c := New(nil)
	cases := []struct {
		name, base, stream string
		msg                *domain.Message
	}{
		{"empty base", "", "s", &domain.Message{Body: "x"}},
		{"empty stream", "http://x", "", &domain.Message{Body: "x"}},
		{"nil msg", "http://x", "s", nil},
	}
	for _, c2 := range cases {
		t.Run(c2.name, func(t *testing.T) {
			t.Parallel()
			if err := c.Send(context.Background(), c2.base, c2.stream, c2.msg); err == nil {
				t.Errorf("%s: expected error", c2.name)
			}
		})
	}
}
