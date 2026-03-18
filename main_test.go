package main

import (
	"reflect"
	"strconv"
	"testing"

	"cfping/scanner"
)

func TestExpandTargets(t *testing.T) {
	t.Parallel()

	inputs := []string{
		" 1.1.1.1 \n\n1.1.1.2 ",
		"1.1.1.0/30",
		"invalid/33",
		"example.com",
	}

	got := expandTargets(inputs)
	want := []string{
		"1.1.1.1",
		"1.1.1.2",
		"1.1.1.1",
		"1.1.1.2",
		"example.com",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expandTargets() = %v, want %v", got, want)
	}
}

func TestLimitTargets(t *testing.T) {
	t.Parallel()

	targets := make([]string, 0, maxTargetsPerRequest+5)
	for i := range maxTargetsPerRequest + 5 {
		targets = append(targets, "192.0.2."+strconv.Itoa(i))
	}

	got := limitTargets(targets, maxTargetsPerRequest)

	if len(got) != maxTargetsPerRequest {
		t.Fatalf("limitTargets() length = %d, want %d", len(got), maxTargetsPerRequest)
	}

	if got[0] != "192.0.2.0" || got[len(got)-1] != "192.0.2.9999" {
		t.Fatalf("limitTargets() kept wrong range: first=%s last=%s", got[0], got[len(got)-1])
	}
}

func TestScanConcurrency(t *testing.T) {
	t.Parallel()

	if got := scanConcurrency("ping"); got != pingConcurrency {
		t.Fatalf("scanConcurrency(ping) = %d, want %d", got, pingConcurrency)
	}

	if got := scanConcurrency("speed"); got != speedConcurrency {
		t.Fatalf("scanConcurrency(speed) = %d, want %d", got, speedConcurrency)
	}
}

func TestShouldSkipPingResult(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		result     *scanner.Result
		maxLatency int
		want       bool
	}{
		{
			name:       "nil result",
			result:     nil,
			maxLatency: 400,
			want:       false,
		},
		{
			name: "slow ok result",
			result: &scanner.Result{
				Status:   "ok",
				PingTime: 500,
			},
			maxLatency: 400,
			want:       true,
		},
		{
			name: "fast ok result",
			result: &scanner.Result{
				Status:   "ok",
				PingTime: 300,
			},
			maxLatency: 400,
			want:       false,
		},
		{
			name: "error result is not filtered",
			result: &scanner.Result{
				Status:   "error",
				PingTime: 999,
			},
			maxLatency: 400,
			want:       false,
		},
		{
			name: "disabled max latency",
			result: &scanner.Result{
				Status:   "ok",
				PingTime: 999,
			},
			maxLatency: 0,
			want:       false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := shouldSkipPingResult(tc.result, tc.maxLatency); got != tc.want {
				t.Fatalf("shouldSkipPingResult() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestHosts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cidr    string
		want    []string
		wantErr bool
	}{
		{
			name: "ipv4 slash 30 strips network and broadcast",
			cidr: "198.51.100.0/30",
			want: []string{"198.51.100.1", "198.51.100.2"},
		},
		{
			name: "ipv4 slash 32 keeps single host",
			cidr: "198.51.100.7/32",
			want: []string{"198.51.100.7"},
		},
		{
			name:    "invalid cidr",
			cidr:    "198.51.100.0/99",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := hosts(tc.cidr)
			if tc.wantErr {
				if err == nil {
					t.Fatal("hosts() error = nil, want error")
				}
				return
			}

			if err != nil {
				t.Fatalf("hosts() error = %v", err)
			}

			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("hosts() = %v, want %v", got, tc.want)
			}
		})
	}
}
