package main

import (
	"cfping/scanner"
	"log"
	"strings"
)

const (
	maxTargetsPerRequest = 10000
	pingConcurrency      = 50
	speedConcurrency     = 1
)

func expandTargets(inputs []string) []string {
	targets := make([]string, 0, len(inputs))

	for _, line := range inputs {
		parts := strings.Split(line, "\n")
		for _, part := range parts {
			target := strings.TrimSpace(part)
			if target == "" {
				continue
			}

			if strings.Contains(target, "/") {
				hostsInCIDR, err := hosts(target)
				if err != nil {
					log.Println("CIDR parse error:", err)
					continue
				}
				targets = append(targets, hostsInCIDR...)
				continue
			}

			targets = append(targets, target)
		}
	}

	return targets
}

func limitTargets(targets []string, max int) []string {
	if max <= 0 || len(targets) <= max {
		return targets
	}
	return targets[:max]
}

func scanConcurrency(scanType string) int {
	if scanType == "speed" {
		return speedConcurrency
	}
	return pingConcurrency
}

func shouldSkipPingResult(result *scanner.Result, maxLatency int) bool {
	return result != nil &&
		result.Status == "ok" &&
		maxLatency > 0 &&
		result.PingTime > int64(maxLatency)
}
