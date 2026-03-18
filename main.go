package main

import (
	"cfping/scanner"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
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
	http.Handle("/", noCache(fs))
	http.HandleFunc("/ws", handleWebSocket)

	port := "13334"
	url := "http://localhost:" + port
	fmt.Printf("Server started at %s\n", url)

	// Automated checks can skip browser launch to avoid noisy popups.
	if os.Getenv("CFPING_SKIP_BROWSER") != "1" {
		go openBrowser(url)
	}

	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

func noCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		next.ServeHTTP(w, r)
	})
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
	expandedIPs := limitTargets(expandTargets(req.IPs), maxTargetsPerRequest)

	var wg sync.WaitGroup
	sem := make(chan struct{}, scanConcurrency(req.Type))

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
				if shouldSkipPingResult(result, req.MaxLatency) {
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
