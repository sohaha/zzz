//go:build !windows
// +build !windows

package util

import (
	"syscall"
)

func SetLimit(l int) {
	var rLimit syscall.Rlimit
	err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	if err != nil {
		return
	}
	rLimit.Max = uint64(l)
	rLimit.Cur = uint64(l)
	err = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	if err != nil {
		return
	}
}
