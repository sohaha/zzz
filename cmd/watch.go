package cmd

import (
	"fmt"

	"github.com/sohaha/zlsgo/zfile"
	"github.com/spf13/cobra"

	"github.com/sohaha/zzz/app/watch"
	"github.com/sohaha/zzz/util"
)

var (
	watchUse = "watch"
	watchCfg string
	startCmd *cobra.Command
)

var watchCmd = &cobra.Command{
	Use:     watchUse,
	Short:   "文件变更监控",
	Long:    ``,
	Aliases: []string{"w"},
	// Example: fmt.Sprintf(`  %s %s    Start listening service (equal "%[1]s %[2]s start")
	// `, use, watchUse),
	Run: func(cmd *cobra.Command, args []string) {
		oldCfg := "./zls-watch.yaml"
		if !zfile.FileExist(watchCfg) {
			// compatibleWithOlderVersions
			if !zfile.FileExist(oldCfg) {
				_ = cmd.Help()
				return
			}
		}
		util.Log.Warn(fmt.Sprintf("检测到配置文件，直接启动。如需查看帮助，请运行 `%s %s --help`", use, watchUse))
		startCmd.Run(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(watchCmd)
	watch.InitCmd(watchCmd)
	startCmd = watch.StartCmd(watchCmd)
	watchCmd.PersistentFlags().StringVarP(&watchCfg, "cfg", "C", "./zzz-watch.yaml", "监听配置文件路径")
}
