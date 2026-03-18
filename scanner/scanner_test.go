package scanner

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPingSuccess(t *testing.T) {
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

	port := listener.Addr().(*net.TCPAddr).Port
	result := Ping("127.0.0.1", port, time.Second)

	if result == nil {
		t.Fatal("Ping() returned nil")
	}

	if result.Status != "ok" {
		t.Fatalf("Ping() status = %s, want ok", result.Status)
	}

	if result.PingTime < 0 {
		t.Fatalf("Ping() ping time = %d, want >= 0", result.PingTime)
	}
}

func TestPingFailure(t *testing.T) {
	t.Parallel()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()

	result := Ping("127.0.0.1", port, 200*time.Millisecond)

	if result == nil {
		t.Fatal("Ping() returned nil")
	}

	if result.Status != "error" {
		t.Fatalf("Ping() status = %s, want error", result.Status)
	}

	if result.ErrorMsg == "" {
		t.Fatal("Ping() error message is empty")
	}
}

func TestSpeedTestSuccess(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		chunk := make([]byte, 32*1024)
		for i := 0; i < 16; i++ {
			if _, err := w.Write(chunk); err != nil {
				return
			}
		}
	}))
	defer server.Close()

	host, portText, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatalf("net.SplitHostPort() error = %v", err)
	}

	var port int
	if _, err := fmt.Sscanf(portText, "%d", &port); err != nil {
		t.Fatalf("Sscanf() error = %v", err)
	}

	result := SpeedTest(host, port, server.URL, 2*time.Second)

	if result == nil {
		t.Fatal("SpeedTest() returned nil")
	}

	if result.Status != "ok" {
		t.Fatalf("SpeedTest() status = %s, want ok", result.Status)
	}

	if result.Download <= 0 {
		t.Fatalf("SpeedTest() download = %f, want > 0", result.Download)
	}
}

func TestSpeedTestFailure(t *testing.T) {
	t.Parallel()

	result := SpeedTest("127.0.0.1", 443, "://bad-url", time.Second)

	if result == nil {
		t.Fatal("SpeedTest() returned nil")
	}

	if result.Status != "error" {
		t.Fatalf("SpeedTest() status = %s, want error", result.Status)
	}

	if result.ErrorMsg == "" {
		t.Fatal("SpeedTest() error message is empty")
	}
}
