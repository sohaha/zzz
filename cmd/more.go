package cmd

import (
	"fmt"

	"github.com/sohaha/zlsgo/zlog"
	"github.com/sohaha/zlsgo/zstring"
	"github.com/sohaha/zzz/app/more"
	"github.com/sohaha/zzz/util"
	"github.com/spf13/cobra"
)

var (
	moreCmdUse = "more"
	moreCmd    = &cobra.Command{
		Use:   moreCmdUse,
		Short: "更多命令",
		Long:  ``,
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) <= 0 {
				_ = cmd.Help()
				return
			}
			err := more.RunMethod(args[0], args)
			if err != nil {
				util.Log.Errorf("未知命令: %s\n", args[0])
				_ = cmd.Help()
			}
		},
	}
)

func init() {
	example := zstring.Buffer()
	example.WriteString(fmt.Sprintf("  %-12s", "install"))
	example.WriteString(" ")
	example.WriteString("安装 zzz 到系统路径")
	if util.IsInstall() {
		example.WriteString(zlog.ColorTextWrap(zlog.ColorRed, " (Installed)"))
	}

	example.WriteString(fmt.Sprintf("\n  %-12s", "sh"))
	example.WriteString(" ")
	example.WriteString("zzz 一键安装脚本")

	moreCmd.Example = example.String()
	rootCmd.AddCommand(moreCmd)
}
