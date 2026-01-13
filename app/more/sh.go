package more

import (
	"github.com/sohaha/zzz/util"
)

func (m *Methods) Sh(vars []string) {
	util.Log.Println("Linux 或 MacOS:")
	util.Log.Println("  sudo curl -L https://raw.githubusercontent.com/sohaha/zzz/master/install.sh | bash")

	util.Log.Println("\nWindows:")
	util.Log.Println("  下载地址: https://github.com/sohaha/zzz/releases")
}
