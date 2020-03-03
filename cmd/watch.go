package cmd

import (
	"fmt"
	"time"

	"github.com/sohaha/zlsgo/zfile"
	"github.com/sohaha/zzz/app/watch"
	"github.com/sohaha/zzz/util"
	"github.com/spf13/cobra"
)

var (
	watchUse = "watch"
	watchCfg string
	startCmd *cobra.Command
)

// watchCmd represents the watch command
var watchCmd = &cobra.Command{
	Use:     watchUse,
	Short:   "File update monitoring",
	Long:    ``,
	Aliases: []string{"w"},
	// Example: fmt.Sprintf(`  %s %s    Start listening service (equal "%[1]s %[2]s start")
	// `, use, watchUse),
	Run: func(cmd *cobra.Command, args []string) {
		if !zfile.FileExist(watchCfg) {
			_ = cmd.Help()
			return
		}
		go func() {
			time.Sleep(1 * time.Second)
			util.Log.Warn(fmt.Sprintf("The configuration file exists and starts directly. If you need to view the help information, please use `%s %s --help`", use, watchUse))
		}()
		startCmd.Run(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(watchCmd)
	watch.InitCmd(watchCmd)
	startCmd = watch.StartCmd(watchCmd)
	watchCmd.PersistentFlags().StringVarP(&watchCfg, "cfg", "C", "./zls-watch.yaml", "Watch config file path")
}
