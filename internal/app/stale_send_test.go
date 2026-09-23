package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/0cv/herdr-mobile-relay/internal/config"
	"github.com/0cv/herdr-mobile-relay/internal/coordinator"
	"github.com/0cv/herdr-mobile-relay/internal/transport"
	"github.com/coder/websocket"
)

// The phone must never be left holding herdr's own wording, and the relay must
// keep a trace of what it refused: the original error, the pane, the session and
// who was told. Both halves are asserted here over a real websocket, the way the
// phone receives them.
func TestForwardedFailureIsReframedAndLogged(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	logs := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(logs, nil))
	cfg := &config.Config{Host: "127.0.0.1", InstanceID: "test-instance"}
	server := New(cfg, "0.9.0", "abc123", logger)
	if server.herdrC == nil {
		t.Fatal("the server has no herdr client to read the socket from")
	}
	server.sessionMu.Lock()
	server.mirroredSession = "main"
	server.sessionMu.Unlock()

	hub := transport.NewHub(cfg, logger)
	accepted := make(chan *transport.ClientConn, 1)
	hub.SetOnConnect(func(client *transport.ClientConn) { accepted <- client })
	httpServer := httptest.NewServer(http.HandlerFunc(hub.HandleWebSocket))
	defer httpServer.Close()
	server.hub = hub

	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(httpServer.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	conn.SetReadLimit(transport.MaxOutboundMessageBytes)

	var client *transport.ClientConn
	select {
	case client = <-accepted:
	case <-ctx.Done():
		t.Fatal("the websocket client was never registered")
	}

	// The refusal a person actually saw, captured from the live case: the relay's
	// own generation check after the session behind the pane was replaced.
	server.publishCommandResult(client, &coordinator.CommandResult{
		RequestID: "req-verify-1",
		Action:    "send_input",
		Phase:     "failed",
		PaneID:    "w1:p5",
		Error:     "pane session was replaced",
	})

	// The inventory reload travels first: the phone is told the list changed
	// before it is told what went wrong.
	readCtx, readCancel := context.WithTimeout(ctx, 5*time.Second)
	defer readCancel()
	var message map[string]any
	sawSessionsRefresh := false
	for attempt := 0; attempt < 10; attempt++ {
		_, data, err := conn.Read(readCtx)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, &message); err != nil {
			t.Fatal(err)
		}
		if message["type"] == "sessions" {
			sawSessionsRefresh = true
			continue
		}
		if message["type"] == "command_result" {
			break
		}
	}
	if !sawSessionsRefresh {
		t.Fatal("the phone was never told to reload its session list")
	}
	if message["type"] != "command_result" {
		t.Fatalf("frame = %#v, want a command_result", message)
	}
	if message["ok"] != false {
		t.Fatalf("ok = %v, want false", message["ok"])
	}
	text, _ := message["error"].(string)
	if strings.Contains(text, "pane session was replaced") {
		t.Fatalf("herdr's own words reached the phone: %q", text)
	}
	if !strings.Contains(text, "no longer running") {
		t.Fatalf("the phone was not told what happened: %q", text)
	}
	data_, _ := message["data"].(map[string]any)
	if data_["code"] != "pane_stale" || data_["refresh"] != true {
		t.Fatalf("data = %#v, want the pane_stale code and a refresh", data_)
	}

	logged := logs.String()
	for _, want := range []string{
		"relay refused a command and told the phone",
		"herdr_error=\"pane session was replaced\"",
		"pane=w1:p5",
		"session=\"main (",
		"action=send_input",
		"code=pane_stale",
	} {
		if !strings.Contains(logged, want) {
			t.Fatalf("the service log is missing %q:\n%s", want, logged)
		}
	}
	if !strings.Contains(logged, "inventory_reloaded=true") {
		t.Fatalf("the log does not record that the inventory was reloaded:\n%s", logged)
	}
}

// A failure that is not about a stale pane keeps its own words, and is logged
// just the same: the point of the trace is to see what herdr said.
func TestOrdinaryFailureIsLoggedUnchanged(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	logs := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(logs, nil))
	cfg := &config.Config{Host: "127.0.0.1", InstanceID: "test-instance"}
	server := New(cfg, "0.9.0", "abc123", logger)

	hub := transport.NewHub(cfg, logger)
	accepted := make(chan *transport.ClientConn, 1)
	hub.SetOnConnect(func(client *transport.ClientConn) { accepted <- client })
	httpServer := httptest.NewServer(http.HandlerFunc(hub.HandleWebSocket))
	defer httpServer.Close()
	server.hub = hub

	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(httpServer.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	conn.SetReadLimit(transport.MaxOutboundMessageBytes)
	var client *transport.ClientConn
	select {
	case client = <-accepted:
	case <-ctx.Done():
		t.Fatal("the websocket client was never registered")
	}

	server.publishCommandResult(client, &coordinator.CommandResult{
		RequestID: "req-verify-2",
		Action:    "send_input",
		Phase:     "failed",
		PaneID:    "w1:p6",
		Error:     "the agent refused the prompt",
	})

	readCtx, readCancel := context.WithTimeout(ctx, 5*time.Second)
	defer readCancel()
	_, data, err := conn.Read(readCtx)
	if err != nil {
		t.Fatal(err)
	}
	var message map[string]any
	if err := json.Unmarshal(data, &message); err != nil {
		t.Fatal(err)
	}
	if message["error"] != "the agent refused the prompt" {
		t.Fatalf("an ordinary failure was rewritten: %#v", message)
	}
	if _, hasData := message["data"]; hasData {
		t.Fatalf("an ordinary failure gained data: %#v", message)
	}
	if !strings.Contains(logs.String(), `herdr_error="the agent refused the prompt"`) {
		t.Fatalf("an ordinary failure was not logged:\n%s", logs.String())
	}
	if strings.Contains(logs.String(), "code=pane_stale") {
		t.Fatalf("an ordinary failure was classified as a stale pane:\n%s", logs.String())
	}
}

var _ = io.Discard
