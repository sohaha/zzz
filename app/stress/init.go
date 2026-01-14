package stress

import (
	"io/ioutil"

	"github.com/sohaha/zlsgo/zfile"
	"github.com/sohaha/zzz/app/root"
	"github.com/sohaha/zzz/util"
	"github.com/spf13/cobra"
)

var (
	initCmdUse = "init"
	initCmd    = &cobra.Command{
		Use:   initCmdUse,
		Short: "生成配置文件",
		Run: func(cmd *cobra.Command, args []string) {
			cfg, _ := cmd.Flags().GetString("cfg")
			path := zfile.RealPath(cfg)
			if zfile.FileExist(path) && !force {
				util.Log.Fatal("配置文件已存在，如需覆盖请使用 --force")
			}
			err := initCfg(path)
			if err != nil {
				util.Log.Fatal(err)
			}
			util.Log.Successf("创建 %s 成功", path)
		},
	}
	force bool
)

func InitCmd(watchCmd *cobra.Command) {
	watchCmd.AddCommand(initCmd)
}

func init() {
	initCmd.Flags().BoolVarP(&force, "force", "f", false, "覆盖配置文件")
}

func initCfg(path string) error {
	config := root.GetExampleStressConfig("")
	return ioutil.WriteFile(path, []byte(config), 0o644)
}
