//go:build windows

package main

import "runtime"

func readHostMetrics(_ string) (map[string]any, error) {
	return map[string]any{"cpu": runtime.NumCPU(), "cpuPercent": 0, "memoryBytes": 0, "memoryAvailableBytes": 0, "diskBytes": 0, "diskAvailableBytes": 0, "uptimeSeconds": 0}, nil
}
