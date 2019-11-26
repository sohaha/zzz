package cmd

import (
	"fmt"
	"github.com/sohaha/zlsgo/zlog"
	"github.com/sohaha/zlsgo/zstring"
	"github.com/sohaha/zzz/app/more"
	"github.com/sohaha/zzz/util"
	"github.com/spf13/cobra"
)

var moreCmdUse = "more"
var moreCmd = &cobra.Command{
	Use:   moreCmdUse,
	Short: "More commands",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) <= 0 {
			_ = cmd.Help()
			return
		}
		err := more.RunMethod(args[0], args)
		if err != nil {
			util.Log.Errorf("unknown commands: %s\n",args[0])
			_ = cmd.Help()
		}
	},
}

func init() {
	var example = zstring.Buffer()
	example.WriteString(fmt.Sprintf("  %-12s", "install"))
	example.WriteString(" ")
	example.WriteString("Install Zzz into the system")
	if util.IsInstall() {
		example.WriteString(zlog.ColorTextWrap(zlog.ColorRed, " (Installed)"))
	}
	moreCmd.Example = example.String()
	rootCmd.AddCommand(moreCmd)
}
