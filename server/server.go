// Package server wires the HTTP surface for the mock channels app.
package server

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/helixml/comms-demo/domain"
	"github.com/helixml/comms-demo/helixclient"
	"github.com/helixml/comms-demo/store"
)

// Deps bundles the collaborators a Server needs.
type Deps struct {
	Store       *store.Store
	Helix       *helixclient.Client
	Logger      *slog.Logger
	TemplatesFS fs.FS // root containing templates/*.html and static/*
	IDGen       IDGenerator
	Clock       Clock
}

// IDGenerator produces fresh message_ids. Indirected for tests.
type IDGenerator interface {
	NewID() string
}

// Clock returns wall time. Indirected for tests.
type Clock interface {
	Now() time.Time
}

// Server is the HTTP handler bundle.
type Server struct {
	deps    Deps
	tmpl    *template.Template
	staticH http.Handler
}

// New constructs a Server with parsed templates and a static-asset handler.
func New(d Deps) (*Server, error) {
	if d.Store == nil {
		return nil, errors.New("server: nil store")
	}
	if d.Helix == nil {
		return nil, errors.New("server: nil helix client")
	}
	if d.Logger == nil {
		return nil, errors.New("server: nil logger")
	}
	if d.TemplatesFS == nil {
		return nil, errors.New("server: nil templates fs")
	}
	if d.IDGen == nil {
		return nil, errors.New("server: nil id generator")
	}
	if d.Clock == nil {
		return nil, errors.New("server: nil clock")
	}

	tmpl, err := parseTemplates(d.TemplatesFS)
	if err != nil {
		return nil, err
	}

	staticFS, err := fs.Sub(d.TemplatesFS, "static")
	if err != nil {
		return nil, fmt.Errorf("server: sub static: %w", err)
	}

	return &Server{
		deps:    d,
		tmpl:    tmpl,
		staticH: http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))),
	}, nil
}

// Handler returns the http.Handler with all routes registered.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/admin", s.handleAdmin)
	mux.HandleFunc("/admin/channels", s.handleAdminChannels)
	mux.HandleFunc("/admin/reset", s.handleAdminReset)
	mux.HandleFunc("/channel/", s.handleChannel)
	mux.HandleFunc("/in/", s.handleInbound)
	mux.Handle("/static/", s.staticH)
	return logMiddleware(s.deps.Logger, mux)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	channels, err := s.deps.Store.Channels()
	if err != nil {
		s.serverError(w, "list channels", err)
		return
	}
	bound := bindChannels(channels)
	if err := s.tmpl.ExecuteTemplate(w, "index.html", indexView(bound)); err != nil {
		s.deps.Logger.Error("render index", "err", err)
	}
}

// handleChannel multiplexes everything under /channel/...
//
//	GET  /channel/{id}                       — list threads
//	GET  /channel/{id}/thread/{thread_id}    — thread detail
//	POST /channel/{id}/reply                 — reply form submit
//	POST /channel/{id}/compose               — new-thread form submit
//	GET  /channel/{id}/compose               — new-thread form
func (s *Server) handleChannel(w http.ResponseWriter, r *http.Request) {
	parts := splitPath(r.URL.Path)
	// parts: ["channel", "<id>", ...]
	if len(parts) < 2 || parts[0] != "channel" {
		http.NotFound(w, r)
		return
	}
	channelID := parts[1]

	ch, err := s.deps.Store.Channel(channelID)
	if err != nil {
		http.Error(w, "channel not found", http.StatusNotFound)
		return
	}

	switch {
	case len(parts) == 2 && r.Method == http.MethodGet:
		s.renderChannelInbox(w, r, ch)
	case len(parts) == 4 && parts[2] == "thread" && r.Method == http.MethodGet:
		s.renderThread(w, r, ch, parts[3])
	case len(parts) == 3 && parts[2] == "reply" && r.Method == http.MethodPost:
		s.handleReply(w, r, ch)
	case len(parts) == 3 && parts[2] == "compose" && r.Method == http.MethodGet:
		s.renderCompose(w, r, ch)
	case len(parts) == 3 && parts[2] == "compose" && r.Method == http.MethodPost:
		s.handleCompose(w, r, ch)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) renderChannelInbox(w http.ResponseWriter, r *http.Request, ch *domain.Channel) {
	threads, err := s.deps.Store.Threads(ch.ChannelID)
	if err != nil {
		s.serverError(w, "list threads", err)
		return
	}
	frame := r.URL.Query().Get("frame") == "phone"
	view := channelInboxView{
		Channel: *ch,
		Threads: threads,
		Frame:   frame,
	}
	tmpl := templateForKind(ch.Kind, "inbox")
	if err := s.tmpl.ExecuteTemplate(w, tmpl, view); err != nil {
		s.deps.Logger.Error("render inbox", "err", err)
	}
}

func (s *Server) renderThread(w http.ResponseWriter, r *http.Request, ch *domain.Channel, threadID string) {
	thread, err := s.deps.Store.Thread(ch.ChannelID, threadID)
	if err != nil {
		http.Error(w, "thread not found", http.StatusNotFound)
		return
	}
	frame := r.URL.Query().Get("frame") == "phone"
	view := threadView{
		Channel: *ch,
		Thread:  *thread,
		Frame:   frame,
	}
	tmpl := templateForKind(ch.Kind, "thread")
	if err := s.tmpl.ExecuteTemplate(w, tmpl, view); err != nil {
		s.deps.Logger.Error("render thread", "err", err)
	}
}

func (s *Server) renderCompose(w http.ResponseWriter, r *http.Request, ch *domain.Channel) {
	frame := r.URL.Query().Get("frame") == "phone"
	view := composeView{
		Channel: *ch,
		Frame:   frame,
	}
	tmpl := templateForKind(ch.Kind, "compose")
	if err := s.tmpl.ExecuteTemplate(w, tmpl, view); err != nil {
		s.deps.Logger.Error("render compose", "err", err)
	}
}

func (s *Server) handleReply(w http.ResponseWriter, r *http.Request, ch *domain.Channel) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	threadID := strings.TrimSpace(r.FormValue("thread_id"))
	parentID := strings.TrimSpace(r.FormValue("in_reply_to"))
	to := strings.TrimSpace(r.FormValue("to"))
	subject := strings.TrimSpace(r.FormValue("subject"))
	body := strings.TrimSpace(r.FormValue("body"))
	if body == "" {
		http.Error(w, "body required", http.StatusBadRequest)
		return
	}
	if threadID == "" {
		http.Error(w, "thread_id required", http.StatusBadRequest)
		return
	}

	msg := &domain.Message{
		From:            ch.LocalIdentity,
		To:              splitRecipients(to),
		Subject:         subject,
		Body:            body,
		BodyContentType: "text/plain",
		ThreadID:        threadID,
		InReplyTo:       parentID,
		MessageID:       s.deps.IDGen.NewID(),
	}
	s.sendOutbound(w, r, ch, msg, "/channel/"+ch.ChannelID+"/thread/"+threadID)
}

func (s *Server) handleCompose(w http.ResponseWriter, r *http.Request, ch *domain.Channel) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	to := strings.TrimSpace(r.FormValue("to"))
	subject := strings.TrimSpace(r.FormValue("subject"))
	body := strings.TrimSpace(r.FormValue("body"))
	if body == "" {
		http.Error(w, "body required", http.StatusBadRequest)
		return
	}

	id := s.deps.IDGen.NewID()
	msg := &domain.Message{
		From:            ch.LocalIdentity,
		To:              splitRecipients(to),
		Subject:         subject,
		Body:            body,
		BodyContentType: "text/plain",
		ThreadID:        id,
		MessageID:       id,
	}
	s.sendOutbound(w, r, ch, msg, "/channel/"+ch.ChannelID+"/thread/"+id)
}

func (s *Server) sendOutbound(w http.ResponseWriter, r *http.Request, ch *domain.Channel, msg *domain.Message, redirect string) {
	if ch.HelixBaseURL == "" || ch.HelixStreamID == "" {
		http.Error(w, "channel missing helix config", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	sendErrText := ""
	if err := s.deps.Helix.Send(ctx, ch.HelixBaseURL, ch.HelixStreamID, msg); err != nil {
		s.deps.Logger.Error("helix send failed", "err", err, "channel", ch.ChannelID)
		sendErrText = err.Error()
	}
	if _, err := s.deps.Store.AppendMessage(ch.ChannelID, domain.DirectionOutbound, msg, sendErrText); err != nil {
		s.serverError(w, "append outbound", err)
		return
	}

	if r.URL.Query().Get("frame") == "phone" {
		redirect += "?frame=phone"
	}
	http.Redirect(w, r, redirect, http.StatusSeeOther)
}

func (s *Server) handleInbound(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	parts := splitPath(r.URL.Path)
	// /in/<channel_id>
	if len(parts) != 2 || parts[0] != "in" {
		http.NotFound(w, r)
		return
	}
	channelID := parts[1]

	ch, err := s.deps.Store.Channel(channelID)
	if err != nil {
		http.Error(w, "unknown channel", http.StatusNotFound)
		return
	}

	msg, err := decodeMessage(r)
	if err != nil {
		http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
		return
	}
	if msg.Body == "" {
		http.Error(w, "body required", http.StatusBadRequest)
		return
	}

	// Backfill threading discipline on first messages so the UI groups cleanly.
	if msg.MessageID == "" {
		msg.MessageID = s.deps.IDGen.NewID()
	}
	if msg.ThreadID == "" {
		msg.ThreadID = msg.MessageID
	}

	exists, err := s.deps.Store.MessageIDExists(msg.MessageID)
	if err != nil {
		s.serverError(w, "dedupe lookup", err)
		return
	}
	if exists {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if _, err := s.deps.Store.AppendMessage(ch.ChannelID, domain.DirectionInbound, msg, ""); err != nil {
		s.serverError(w, "append inbound", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAdmin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	channels, err := s.deps.Store.Channels()
	if err != nil {
		s.serverError(w, "list channels", err)
		return
	}
	view := adminView{Channels: channels}
	if err := s.tmpl.ExecuteTemplate(w, "admin.html", view); err != nil {
		s.deps.Logger.Error("render admin", "err", err)
	}
}

func (s *Server) handleAdminChannels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	c := &domain.Channel{
		ChannelID:     strings.TrimSpace(r.FormValue("channel_id")),
		Kind:          domain.ChannelKind(strings.TrimSpace(r.FormValue("kind"))),
		DisplayName:   strings.TrimSpace(r.FormValue("display_name")),
		HelixStreamID: strings.TrimSpace(r.FormValue("helix_stream_id")),
		HelixBaseURL:  strings.TrimSpace(r.FormValue("helix_base_url")),
		LocalIdentity: strings.TrimSpace(r.FormValue("local_identity")),
	}
	if err := s.deps.Store.UpsertChannel(c); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (s *Server) handleAdminReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := s.deps.Store.Reset(); err != nil {
		s.serverError(w, "reset", err)
		return
	}
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (s *Server) serverError(w http.ResponseWriter, msg string, err error) {
	s.deps.Logger.Error(msg, "err", err)
	http.Error(w, msg+": "+err.Error(), http.StatusInternalServerError)
}
