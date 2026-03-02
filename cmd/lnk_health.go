package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/sohaha/zzz/app/lnk/core"
	"github.com/sohaha/zzz/util"
	"github.com/spf13/cobra"
)

func newDiffCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "diff",
		Short:        "查看仓库未提交差异",
		Long:         "显示 lnk 仓库当前未提交的差异内容（等同在仓库目录执行 git diff）",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			lnk := createLnkInstance("")
			output, err := lnk.Diff(isTerminal())
			if err != nil {
				return err
			}
			if strings.TrimSpace(output) == "" {
				util.Log.Successf("当前无未提交变更\n")
				return nil
			}
			cmd.Print(output)
			return nil
		},
	}
}

func newDoctorCmd() *cobra.Command {
	var (
		host   string
		dryRun bool
	)

	cmd := &cobra.Command{
		Use:          "doctor",
		Short:        "诊断并修复仓库健康问题",
		Long:         "检查并修复无效跟踪条目和损坏符号链接，支持 --dry-run 预览",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			lnk := createLnkInstance(host)
			var (
				result *core.DoctorResult
				err    error
			)

			if dryRun {
				result, err = lnk.PreviewDoctor()
			} else {
				result, err = lnk.Doctor()
			}
			if err != nil {
				return err
			}

			printDoctorResult(result, host, dryRun)
			return nil
		},
	}

	cmd.Flags().StringVarP(&host, "host", "H", "", "指定主机名（默认使用系统主机名）")
	cmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "仅预览将修复的问题，不执行修改")
	return cmd
}

func printDoctorResult(result *core.DoctorResult, host string, dryRun bool) {
	hostText := ""
	if host != "" {
		hostText = fmt.Sprintf("（主机: %s）", host)
	}

	if !result.HasIssues() {
		util.Log.Successf("仓库状态健康%s\n", hostText)
		return
	}

	action := "发现"
	if !dryRun {
		action = "已修复"
	}
	util.Log.Warnf("%s %d 个问题%s\n", action, result.TotalIssues(), hostText)

	if len(result.BrokenSymlinks) > 0 {
		prefix := "检测到"
		if !dryRun {
			prefix = "已恢复"
		}
		util.Log.Warnf("%s %d 个损坏链接:\n", prefix, len(result.BrokenSymlinks))
		for _, item := range result.BrokenSymlinks {
			util.Log.Printf("  - %s\n", item)
		}
	}

	if len(result.InvalidEntries) > 0 {
		prefix := "检测到"
		if !dryRun {
			prefix = "已移除"
		}
		util.Log.Warnf("%s %d 个无效条目:\n", prefix, len(result.InvalidEntries))
		for _, item := range result.InvalidEntries {
			util.Log.Printf("  - %s\n", item)
		}
	}

	if dryRun {
		util.Log.Println("使用不带 --dry-run 的 doctor 执行修复")
	}
}

func isTerminal() bool {
	fileInfo, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}
