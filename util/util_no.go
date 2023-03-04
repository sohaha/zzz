//go:build !windows
// +build !windows

package util

import (
	"syscall"
)

func hideFile(path string) error {
	return nil
}

func SetLimit(l uint64) {
	var rLimit syscall.Rlimit
	err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	if err != nil {
		return
	}
	rLimit.Max = l
	rLimit.Cur = l
	err = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	if err != nil {
		return
	}
}
