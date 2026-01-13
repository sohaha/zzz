package more

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sohaha/zlsgo/zutil"
	"github.com/sohaha/zzz/util"
)

func (m *Methods) Install(vars []string) {
	path := util.GetInstallPath()
	// if utils.IsInstall() {
	// 	util.Log.Fatalf("the path %s has exist, does not support install", path)
	// }
	// if err := os.MkdirAll(path, 0755); err != nil {
	// 	util.Log.Fatalf(err.Error())
	// }
	ePath, _ := exec.LookPath(os.Args[0])
	ePath, _ = filepath.Abs(ePath)
	res, err := copyMain(ePath, path)
	if err != nil {
		if strings.Contains(res, "Permission denied") {
			util.Log.Fatal("权限不足，请使用 root 权限执行")
		} else if strings.Contains(err.Error(), "exit status 1") {
			util.Log.Fatal("安装失败，请重试")
		} else {
			util.Log.Fatal(err)
		}
	}

	util.Log.Success("安装完成")
}

func copyMain(src, dest string) (data string, err error) {
	if zutil.IsWin() {
		data, err = util.ExecCommand("cmd", "/C", "copy", src, dest)
	} else {
		data, err = util.ExecCommand("cp", src, dest)
	}
	return
}
