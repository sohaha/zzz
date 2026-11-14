//go:build !windows
// +build !windows

package util

import (
	"syscall"
	"testing"
)

func TestSetLimitKeepsCurrentLimit(t *testing.T) {
	// 保持当前上限不变，避免真实降低进程文件描述符限制
	var orig syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &orig); err != nil {
		t.Skipf("无法读取 RLIMIT_NOFILE: %v", err)
	}
	SetLimit(int(orig.Cur))
	var now syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &now); err != nil {
		t.Skipf("无法复查 RLIMIT_NOFILE: %v", err)
	}
	if now.Cur != orig.Cur {
		t.Errorf("SetLimit(当前值) 后 Cur 变化: %d -> %d", orig.Cur, now.Cur)
	}

	// 非正值不应修改限制
	SetLimit(0)
	SetLimit(-1)
	var afterInvalid syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &afterInvalid); err != nil {
		t.Skipf("无法复查 RLIMIT_NOFILE: %v", err)
	}
	if afterInvalid.Cur != orig.Cur {
		t.Errorf("SetLimit 非正值后 Cur 变化: %d -> %d", orig.Cur, afterInvalid.Cur)
	}
}
