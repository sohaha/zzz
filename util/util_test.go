package util

import (
	"os"
	"runtime"
	"testing"
)

func TestGetLineDujt(t *testing.T) {
	if len(List) == 0 {
		t.Fatal("List 不应为空")
	}
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		line := GetLineDujt()
		if !seen[line] {
			seen[line] = true
		}
		found := false
		for _, v := range List {
			if v == line {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("GetLineDujt 返回了不在 List 中的内容: %q", line)
		}
	}
}

func TestJudge(t *testing.T) {
	cases := map[string][]string{
		"windows": {"win", "windows", "w"},
		"darwin":  {"mac", "macOS", "macos", "m"},
		"linux":   {"linux", "l"},
	}
	aliases, ok := cases[runtime.GOOS]
	if !ok {
		t.Skipf("未覆盖的平台: %s", runtime.GOOS)
	}
	for _, alias := range aliases {
		if !judge(alias) {
			t.Errorf("judge(%q) 在 %s 上应为 true", alias, runtime.GOOS)
		}
	}
	// 当前平台之外的别名应为 false
	for goos, otherAliases := range cases {
		if goos == runtime.GOOS {
			continue
		}
		for _, alias := range otherAliases {
			if judge(alias) {
				t.Errorf("judge(%q) 在 %s 上应为 false", alias, runtime.GOOS)
			}
		}
	}
	if judge("unknown-os") {
		t.Error("judge(\"unknown-os\") 应为 false")
	}
}

func TestOSCommand(t *testing.T) {
	// 无 @ 前缀，原样返回
	if got := OSCommand("echo hello"); got != "echo hello" {
		t.Errorf("OSCommand 无前缀应原样返回，得到 %q", got)
	}
	// 当前平台前缀，去掉前缀返回
	current := ""
	switch runtime.GOOS {
	case "windows":
		current = "win"
	case "darwin":
		current = "mac"
	case "linux":
		current = "linux"
	}
	if current != "" {
		if got := OSCommand(current + "@echo hi"); got != "echo hi" {
			t.Errorf("OSCommand(%q) 应为 %q，得到 %q", current+"@echo hi", "echo hi", got)
		}
		if got := OSCommand(current + "@echo a@echo b"); got != "echo a@echo b" {
			t.Errorf("OSCommand 前缀后含 @ 应完整保留，得到 %q", got)
		}
	}
	// 非 @ 别名形式的其他平台前缀，返回空串
	other := "windows"
	if runtime.GOOS == "windows" {
		other = "linux"
	}
	if got := OSCommand(other + "@echo hi"); got != "" {
		t.Errorf("OSCommand 其他平台前缀应返回空串，得到 %q", got)
	}
	// 多平台选择：包含当前平台则返回命令，否则空串
	switch runtime.GOOS {
	case "darwin":
		if got := OSCommand("w|m@echo hi"); got != "echo hi" {
			t.Errorf("OSCommand(\"w|m@echo hi\") 在 darwin 上应为 %q，得到 %q", "echo hi", got)
		}
		if got := OSCommand("l|w@echo hi"); got != "" {
			t.Errorf("OSCommand(\"l|w@echo hi\") 在 darwin 上应为空串，得到 %q", got)
		}
	case "linux":
		if got := OSCommand("w|m@echo hi"); got != "" {
			t.Errorf("OSCommand(\"w|m@echo hi\") 在 linux 上应为空串，得到 %q", got)
		}
		if got := OSCommand("m|w@echo hi"); got != "" {
			t.Errorf("OSCommand(\"m|w@echo hi\") 在 linux 上应为空串，得到 %q", got)
		}
		if got := OSCommand("w|l@echo hi"); got != "echo hi" {
			t.Errorf("OSCommand(\"w|l@echo hi\") 在 linux 上应为 %q，得到 %q", "echo hi", got)
		}
	case "windows":
		if got := OSCommand("m|l@echo hi"); got != "" {
			t.Errorf("OSCommand(\"m|l@echo hi\") 在 windows 上应为空串，得到 %q", got)
		}
	}
	// 未知前缀：无 | 且不在别名表中，原样返回
	if got := OSCommand("unknown@echo hi"); got != "unknown@echo hi" {
		t.Errorf("OSCommand 未知前缀应原样返回，得到 %q", got)
	}
}

func TestGetHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("无法获取用户主目录")
	}
	if got := GetHome(); got != home+"/" {
		t.Errorf("GetHome() = %q, want %q", got, home+"/")
	}
}

func TestGetInstallPath(t *testing.T) {
	if GetInstallPath() == "" {
		t.Error("GetInstallPath() 不应为空")
	}
}

func TestIsInstall(t *testing.T) {
	// 仅验证类型与无 panic；具体值依赖安装状态
	_ = IsInstall()
}

func TestCheckIfErrorNil(t *testing.T) {
	// nil 不应触发 Fatal
	CheckIfError(nil)
}
