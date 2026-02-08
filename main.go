package main

import (
	"cfping/scanner"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type ScanRequest struct {
	Type        string   `json:"type"` // "ping" or "speed"
	IPs         []string `json:"ips"`  // List of IPs or CIDRs
	Port        int      `json:"port"`
	DownloadURL string   `json:"download_url"`
	MaxLatency  int      `json:"max_latency"`
}

func main() {
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/", fs)
	http.HandleFunc("/ws", handleWebSocket)

	port := "13334"
	url := "http://localhost:" + port
	fmt.Printf("Server started at %s\n", url)

	// Open browser automatically
	go openBrowser(url)

	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

func openBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	if err != nil {
		log.Printf("Failed to open browser: %v", err)
	}
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("upgrade:", err)
		return
	}
	defer conn.Close()

	for {
		mt, message, err := conn.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			break
		}

		var req ScanRequest
		if err := json.Unmarshal(message, &req); err != nil {
			log.Println("json unmarshal:", err)
			continue
		}

		go processRequest(conn, mt, req)
	}
}

func processRequest(conn *websocket.Conn, messageType int, req ScanRequest) {
	// Parse IPs
	expandedIPs := []string{}
	for _, line := range req.IPs {
		parts := strings.Split(line, "\n")
		for _, p := range parts {
			trim := strings.TrimSpace(p)
			if trim != "" {
				if strings.Contains(trim, "/") {
					ips, err := hosts(trim)
					if err == nil {
						expandedIPs = append(expandedIPs, ips...)
					} else {
						log.Println("CIDR parse error:", err)
					}
				} else {
					expandedIPs = append(expandedIPs, trim)
				}
			}
		}
	}

	if len(expandedIPs) > 10000 {
		expandedIPs = expandedIPs[:10000]
	}

	var wg sync.WaitGroup
	maxConcurrency := 50
	if req.Type == "speed" {
		// Speed tests compete for local bandwidth; run one-at-a-time for accurate per-target results.
		maxConcurrency = 1
	}
	sem := make(chan struct{}, maxConcurrency)

	// Mutex to protect WebSocket writes
	var wsMutex sync.Mutex

	for i, ip := range expandedIPs {
		wg.Add(1)
		sem <- struct{}{}

		go func(order int, targetIP string) {
			defer wg.Done()
			defer func() { <-sem }()

			var result *scanner.Result

			if req.Type == "speed" {
				result = scanner.SpeedTest(targetIP, req.Port, req.DownloadURL, 10*time.Second)
			} else {
				result = scanner.Ping(targetIP, req.Port, 2*time.Second)
				if result != nil && result.Status == "ok" && req.MaxLatency > 0 && result.PingTime > int64(req.MaxLatency) {
					return
				}
			}

			if result != nil {
				result.Order = order
			}

			jsonMsg, _ := json.Marshal(result)

			wsMutex.Lock()
			conn.WriteMessage(messageType, jsonMsg)
			wsMutex.Unlock()

		}(i, ip)
	}

	wg.Wait()
	wsMutex.Lock()
	conn.WriteMessage(messageType, []byte(`{"status":"done"}`))
	wsMutex.Unlock()
}

func hosts(cidr string) ([]string, error) {
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, err
	}

	var ips []string
	for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); inc(ip) {
		ips = append(ips, ip.String())
	}
	// For Cloudflare IP ranges, normally exclude network/broadcast,
	// but often 0 and 255 are valid IPs in some contexts or large networks.
	// Using standard logic:
	if len(ips) > 2 {
		return ips[1 : len(ips)-1], nil
	}
	return ips, nil
}

func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}
