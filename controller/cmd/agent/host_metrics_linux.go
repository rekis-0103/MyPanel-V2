//go:build linux

package main

import (
	"bufio"
	"bytes"
	"golang.org/x/sys/unix"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

var hostCPUState struct {
	sync.Mutex
	total uint64
	idle  uint64
}

func readHostMetrics(path string) (map[string]any, error) {
	var info unix.Sysinfo_t
	if err := unix.Sysinfo(&info); err != nil {
		return nil, err
	}
	var fs unix.Statfs_t
	if err := unix.Statfs(path, &fs); err != nil {
		return nil, err
	}
	unit := uint64(info.Unit)
	if unit == 0 {
		unit = 1
	}
	memoryAvailable := uint64(info.Freeram+info.Bufferram) * unit
	if value, err := memAvailableBytes(); err == nil {
		memoryAvailable = value
	}
	cpuPercent, _ := currentCPUPercent()
	return map[string]any{"cpu": runtime.NumCPU(), "cpuPercent": cpuPercent, "memoryBytes": uint64(info.Totalram) * unit, "memoryAvailableBytes": memoryAvailable, "diskBytes": fs.Blocks * uint64(fs.Bsize), "diskAvailableBytes": fs.Bavail * uint64(fs.Bsize), "uptimeSeconds": info.Uptime}, nil
}

func memAvailableBytes() (uint64, error) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, err
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 && fields[0] == "MemAvailable:" {
			value, parseErr := strconv.ParseUint(fields[1], 10, 64)
			return value * 1024, parseErr
		}
	}
	return 0, scanner.Err()
}

func currentCPUPercent() (float64, error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, err
	}
	line := strings.SplitN(string(data), "\n", 2)[0]
	fields := strings.Fields(line)
	if len(fields) < 5 || fields[0] != "cpu" {
		return 0, os.ErrInvalid
	}
	values := make([]uint64, 0, len(fields)-1)
	for _, raw := range fields[1:] {
		value, parseErr := strconv.ParseUint(raw, 10, 64)
		if parseErr != nil {
			return 0, parseErr
		}
		values = append(values, value)
	}
	var total uint64
	for _, value := range values {
		total += value
	}
	idle := values[3]
	if len(values) > 4 {
		idle += values[4]
	}
	hostCPUState.Lock()
	defer hostCPUState.Unlock()
	previousTotal, previousIdle := hostCPUState.total, hostCPUState.idle
	hostCPUState.total, hostCPUState.idle = total, idle
	if previousTotal == 0 || total <= previousTotal {
		return 0, nil
	}
	deltaTotal, deltaIdle := total-previousTotal, idle-previousIdle
	if deltaIdle > deltaTotal {
		deltaIdle = deltaTotal
	}
	return float64(deltaTotal-deltaIdle) * 100 / float64(deltaTotal), nil
}
