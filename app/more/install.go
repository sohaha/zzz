package more

import (
	"strings"

	"github.com/sohaha/zlsgo/zfile"
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
	ePath := zfile.ExecutablePath()
	err := zfile.CopyFile(ePath, path)
	if err != nil {
		if strings.Contains(err.Error(), "permission denied") {
			util.Log.Fatal("权限不足，请使用 root 权限执行")
		}
		util.Log.Fatal(err)
	}

	util.Log.Success("安装完成")
}
