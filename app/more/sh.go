package more

import (
	"github.com/sohaha/zzz/util"
)

func (m *Methods) Sh(vars []string) {
	util.Log.Println("Linux or MacOS:")
	util.Log.Println("  sudo curl -L https://raw.githubusercontent.com/sohaha/zzz/master/install.sh | bash")

	util.Log.Println("\nWindows:")
	util.Log.Println("  Download: https://github.com/sohaha/zzz/releases")
}
