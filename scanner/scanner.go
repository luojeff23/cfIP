package scanner

import (
	"context"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"
)

// Result struct to hold ping/speed test results
type Result struct {
	Order     int     `json:"order"`
	IP        string  `json:"ip"`
	Port      int     `json:"port"`
	PingTime  int64   `json:"ping_time"` // in ms
	Download  float64 `json:"download"`  // in MB/s
	Status    string  `json:"status"`    // "ok", "timeout", "error"
	ErrorMsg  string  `json:"error_msg,omitempty"`
}

// Ping performs a TCP connect to the IP:Port
func Ping(ip string, port int, timeout time.Duration) *Result {
	start := time.Now()
	address := net.JoinHostPort(ip, strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", address, timeout)
	
	res := &Result{
		IP:     ip,
		Port:   port,
		Status: "error",
	}

	if err != nil {
		res.ErrorMsg = err.Error()
		return res
	}
	defer conn.Close()

	duration := time.Since(start)
	res.PingTime = duration.Milliseconds()
	res.Status = "ok"
	return res
}

// SpeedTest performs a download speed test
// It tries to download from the given URL using the specific IP as the dialer
func SpeedTest(ip string, port int, downloadURL string, timeout time.Duration) *Result {
	res := &Result{
		IP:   ip,
		Port: port,
	}

	// Custom dialer to force connection via specific IP
	dialer := &net.Dialer{
		Timeout:   timeout,
		KeepAlive: 30 * time.Second,
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			// addr will be the hostname of the downloadURL.
			// We force it to connect to our specific IP but keep the original hostname for SNI/Host header if possible
			// However, for pure IP connectivity, we might just dial the IP directly if the server accepts it.
			// Ideally for Cloudflare, we connect to the IP but send the Host header of the target URL.
			
			// Force valid IP address for TCP connection
			// address := fmt.Sprintf("%s:%d", ip, port)
			// But wait, if we dial the IP directly, we need to make sure the HTTP request Host header is correct.
			// Standard http.Client does this if we preserve the URL.
			// We just need to hijack the dial process to connect to `ip:port` instead of `dns_result:port`.
			
			return dialer.DialContext(ctx, "tcp", net.JoinHostPort(ip, strconv.Itoa(port)))
		},
		TLSHandshakeTimeout: 10 * time.Second,
		DisableKeepAlives:   true,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   timeout + 10*time.Second, // Allow some extra time for the download itself
	}

	start := time.Now()
	resp, err := client.Get(downloadURL)
	if err != nil {
		res.Status = "error"
		res.ErrorMsg = err.Error()
		return res
	}
	defer resp.Body.Close()

	// Read body to measure speed (limited to e.g., 5 seconds or 10MB to be quick?)
	// For a real speed test we want to measure throughput.
	// Let's read for a fixed duration or up to a fixed size.
	
	buffer := make([]byte, 32*1024) // 32KB buffer
	var totalBytes int64
	
	stopTime := time.Now().Add(5 * time.Second) // Run max 5 seconds
	
	for {
		if time.Now().After(stopTime) {
			break
		}
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			totalBytes += int64(n)
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			// If it's a timeout execution, we just stop
			break
		}
	}

	duration := time.Since(start).Seconds()
	if duration < 0.1 {
		duration = 0.1 // avoid division by zero
	}
	
	// MB/s
	speed := float64(totalBytes) / 1024 / 1024 / duration
	
	res.Download = speed
	res.Status = "ok"
	
	return res
}
