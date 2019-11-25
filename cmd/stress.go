package cmd

import (
	"fmt"
	"github.com/sohaha/zzz/app/stress"
	
	"github.com/spf13/cobra"
)

var stressFlags *stress.Cli
var stressUse = "stress"
var stressCmd = &cobra.Command{
	Use:   stressUse,
	Short: "Web service stress test",
	Example: fmt.Sprintf(`  %s %s www.baidu.com
  %[1]s %[2]s -c 10 -t 10 -u https://www.baidu.com`, use, stressUse),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("stress called")
		if len(cmd.Flags().Args()) > 0 {
			if u, _ := cmd.Flags().GetString("url"); u == "" && len(args) > 0 {
				*stressFlags.RequestUrl = args[0]
			}
			stress.Run(stressFlags)
			return
		}
		_ = cmd.Help()
	},
}

func init() {
	rootCmd.AddCommand(stressCmd)
	stressFlags = &stress.Cli{
		Timeout:     stressCmd.Flags().IntP("timeout", "t", 30, "Timeout, Unit seconds"),
		Concurrency: stressCmd.Flags().IntP("concurrent", "c", 100, "Concurrent number"),
		Duration:    stressCmd.Flags().IntP("duration", "d", 0, "Duration, Unit seconds"),
		RequestUrl:  stressCmd.Flags().StringP("url", "u", "", "Pressured url"),
		Debug:       stressCmd.Flags().Bool("debug", false, "Debug mode"),
		Stat:        stressCmd.Flags().BoolP("stat", "S", false, "Show Http stat"),
		Header:      stressCmd.Flags().StringP("header", "H", "", "Header, eg: Content-Type:application/x-www-form-urlencoded"),
		Body:        stressCmd.Flags().StringP("body", "B", "", "Param, eg: id=1&name=abc"),
		Method:      stressCmd.Flags().StringP("method", "M", "", "Method: get/post..."),
		Hidelog:     stressCmd.Flags().Bool("hide", false, "Hide request log"),
	}
}
