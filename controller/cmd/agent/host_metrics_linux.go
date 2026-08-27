//go:build linux

package main

import (
	"golang.org/x/sys/unix"
	"runtime"
)

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
	return map[string]any{"cpu": runtime.NumCPU(), "memoryBytes": uint64(info.Totalram) * unit, "memoryAvailableBytes": uint64(info.Freeram+info.Bufferram) * unit, "diskBytes": fs.Blocks * uint64(fs.Bsize), "diskAvailableBytes": fs.Bavail * uint64(fs.Bsize), "uptimeSeconds": info.Uptime}, nil
}
