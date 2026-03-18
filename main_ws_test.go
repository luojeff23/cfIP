package main

import (
	"cfping/scanner"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func newTestWebSocketConn(t *testing.T) (*websocket.Conn, func()) {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", handleWebSocket)
	server := httptest.NewServer(mux)

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		server.Close()
		t.Fatalf("websocket dial error = %v", err)
	}

	cleanup := func() {
		_ = conn.Close()
		server.Close()
	}

	return conn, cleanup
}

func readScanResults(t *testing.T, conn *websocket.Conn) []scanner.Result {
	t.Helper()

	if err := conn.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline() error = %v", err)
	}

	results := make([]scanner.Result, 0, 4)

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("ReadMessage() error = %v", err)
		}

		var envelope struct {
			Status string `json:"status"`
			IP     string `json:"ip"`
		}
		if err := json.Unmarshal(message, &envelope); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}

		if envelope.Status == "done" && envelope.IP == "" {
			return results
		}

		var result scanner.Result
		if err := json.Unmarshal(message, &result); err != nil {
			t.Fatalf("result unmarshal error = %v", err)
		}
		results = append(results, result)
	}
}

func TestNoCacheHeaders(t *testing.T) {
	t.Parallel()

	handler := noCache(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Cache-Control"); got != "no-store, no-cache, must-revalidate, proxy-revalidate" {
		t.Fatalf("Cache-Control = %q", got)
	}

	if got := rec.Header().Get("Pragma"); got != "no-cache" {
		t.Fatalf("Pragma = %q", got)
	}

	if got := rec.Header().Get("Expires"); got != "0" {
		t.Fatalf("Expires = %q", got)
	}
}

func TestHandleWebSocketPingFlow(t *testing.T) {
	t.Parallel()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	defer listener.Close()

	go func() {
		conn, err := listener.Accept()
		if err == nil {
			_ = conn.Close()
		}
	}()

	conn, cleanup := newTestWebSocketConn(t)
	defer cleanup()

	port := listener.Addr().(*net.TCPAddr).Port
	req := ScanRequest{
		Type:       "ping",
		IPs:        []string{"127.0.0.1/32"},
		Port:       port,
		MaxLatency: 10_000,
	}

	if err := conn.WriteJSON(req); err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}

	results := readScanResults(t, conn)
	if len(results) != 1 {
		t.Fatalf("results count = %d, want 1", len(results))
	}
	gotResult := results[0]

	if gotResult.IP != "127.0.0.1" {
		t.Fatalf("result IP = %s, want 127.0.0.1", gotResult.IP)
	}

	if gotResult.Port != port {
		t.Fatalf("result port = %d, want %d", gotResult.Port, port)
	}

	if gotResult.Status != "ok" {
		t.Fatalf("result status = %s, want ok", gotResult.Status)
	}

	if gotResult.Order != 0 {
		t.Fatalf("result order = %d, want 0", gotResult.Order)
	}
}

func TestHandleWebSocketSpeedFlow(t *testing.T) {
	t.Parallel()

	downloadServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		chunk := make([]byte, 32*1024)
		for i := 0; i < 8; i++ {
			if _, err := w.Write(chunk); err != nil {
				return
			}
		}
	}))
	defer downloadServer.Close()

	host, portText, err := net.SplitHostPort(strings.TrimPrefix(downloadServer.URL, "http://"))
	if err != nil {
		t.Fatalf("net.SplitHostPort() error = %v", err)
	}

	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatalf("strconv.Atoi() error = %v", err)
	}

	conn, cleanup := newTestWebSocketConn(t)
	defer cleanup()

	req := ScanRequest{
		Type:        "speed",
		IPs:         []string{host},
		Port:        port,
		DownloadURL: downloadServer.URL,
	}

	if err := conn.WriteJSON(req); err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}

	results := readScanResults(t, conn)
	if len(results) != 1 {
		t.Fatalf("results count = %d, want 1", len(results))
	}

	gotResult := results[0]
	if gotResult.IP != host {
		t.Fatalf("result IP = %s, want %s", gotResult.IP, host)
	}

	if gotResult.Port != port {
		t.Fatalf("result port = %d, want %d", gotResult.Port, port)
	}

	if gotResult.Status != "ok" {
		t.Fatalf("result status = %s, want ok", gotResult.Status)
	}

	if gotResult.Download <= 0 {
		t.Fatalf("result download = %f, want > 0", gotResult.Download)
	}
}

func TestHandleWebSocketInvalidJSONKeepsConnectionAlive(t *testing.T) {
	t.Parallel()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	defer listener.Close()

	go func() {
		conn, err := listener.Accept()
		if err == nil {
			_ = conn.Close()
		}
	}()

	conn, cleanup := newTestWebSocketConn(t)
	defer cleanup()

	if err := conn.WriteMessage(websocket.TextMessage, []byte("not-json")); err != nil {
		t.Fatalf("WriteMessage() error = %v", err)
	}

	req := ScanRequest{
		Type:       "ping",
		IPs:        []string{"127.0.0.1"},
		Port:       listener.Addr().(*net.TCPAddr).Port,
		MaxLatency: 10_000,
	}

	if err := conn.WriteJSON(req); err != nil {
		t.Fatalf("WriteJSON() after invalid json error = %v", err)
	}

	results := readScanResults(t, conn)
	if len(results) != 1 {
		t.Fatalf("results count = %d, want 1", len(results))
	}

	if results[0].Status != "ok" {
		t.Fatalf("result status = %s, want ok", results[0].Status)
	}
}

func TestHandleWebSocketEmptyRequestStillFinishes(t *testing.T) {
	t.Parallel()

	conn, cleanup := newTestWebSocketConn(t)
	defer cleanup()

	req := ScanRequest{
		Type: "ping",
		IPs:  []string{},
		Port: 443,
	}

	if err := conn.WriteJSON(req); err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}

	results := readScanResults(t, conn)
	if len(results) != 0 {
		t.Fatalf("results count = %d, want 0", len(results))
	}
}
