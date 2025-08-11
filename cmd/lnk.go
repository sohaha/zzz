package cmd

import (
	"fmt"
	"sort"

	"github.com/sohaha/zzz/app/lnk/core"
	"github.com/sohaha/zzz/util"
	"github.com/spf13/cobra"
)

var globalRepoPath string

var lnkCmd = &cobra.Command{
	Use:          "lnk",
	Short:        "Dotfiles 管理工具",
	Long:         `Git 原生的 dotfiles 管理工具，支持多主机配置`,
	SilenceUsage: true,
	Example: `  # 初始化本地仓库
  zzz lnk init

  # 从远程仓库初始化
  zzz lnk init -r https://github.com/user/dotfiles.git

  # 使用自定义仓库路径
  zzz lnk --repo ~/my-dotfiles init

  # 添加配置文件到管理
  zzz lnk add ~/.bashrc ~/.vimrc

  # 递归添加目录中的所有文件
  zzz lnk add ~/.config --recursive

  # 查看管理的文件列表
  zzz lnk list

  # 查看仓库状态
  zzz lnk status

  # 推送变更到远程仓库
  zzz lnk push "更新配置文件"

  # 从远程仓库拉取变更
  zzz lnk pull

  # 运行引导脚本
  zzz lnk bootstrap`,
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

func createLnkInstance(host string) *core.Lnk {
	var opts []core.Option

	if globalRepoPath != "" {
		opts = append(opts, core.WithRepoPath(globalRepoPath))
	}

	if host != "" {
		opts = append(opts, core.WithHost(host))
	}

	return core.NewLnk(opts...)
}

func init() {
	rootCmd.AddCommand(lnkCmd)

	lnkCmd.PersistentFlags().StringVar(&globalRepoPath, "repo", "", "指定 lnk 仓库路径（默认: ~/.config/lnk）")

	lnkCmd.AddCommand(newInitCmd())
	lnkCmd.AddCommand(newAddCmd())
	lnkCmd.AddCommand(newRmCmd())
	lnkCmd.AddCommand(newListCmd())
	lnkCmd.AddCommand(newStatusCmd())
	lnkCmd.AddCommand(newPushCmd())
	lnkCmd.AddCommand(newPullCmd())
	lnkCmd.AddCommand(newBootstrapCmd())
}

func newInitCmd() *cobra.Command {
	var (
		remote      string
		force       bool
		noBootstrap bool
		host        string
	)

	cmd := &cobra.Command{
		Use:          "init [flags]",
		Short:        "初始化 lnk 仓库",
		Long:         `初始化 lnk dotfiles 仓库`,
		SilenceUsage: true,
		Example: `  # 初始化本地仓库
  zzz lnk init

  # 从远程仓库初始化
  zzz lnk init -r https://github.com/user/dotfiles.git

  # 强制覆盖现有仓库
  zzz lnk init -r https://github.com/user/dotfiles.git --force

  # 禁用自动 bootstrap
  zzz lnk init -r https://github.com/user/dotfiles.git --no-bootstrap`,
		RunE: func(cmd *cobra.Command, args []string) error {
			lnk := createLnkInstance(host)

			if remote != "" {
				if force {
					return lnk.InitWithRemoteForce(remote, noBootstrap)
				} else {
					return lnk.InitWithRemote(remote)
				}
			} else {
				return lnk.Init()
			}
		},
	}

	cmd.Flags().StringVarP(&remote, "remote", "r", "", "远程仓库 URL")
	cmd.Flags().BoolVar(&force, "force", false, "强制覆盖现有仓库")
	cmd.Flags().BoolVar(&noBootstrap, "no-bootstrap", false, "禁用自动 bootstrap 脚本执行")
	cmd.Flags().StringVar(&host, "host", "", "指定主机名（默认使用系统主机名）")

	return cmd
}

func newAddCmd() *cobra.Command {
	var (
		recursive bool
		host      string
	)

	cmd := &cobra.Command{
		Use:          "add <file>... [flags]",
		Short:        "添加文件到 lnk 管理",
		Long:         `将配置文件添加到 lnk 管理`,
		SilenceUsage: true,
		Example: `  # 添加单个文件
  zzz lnk add ~/.bashrc

  # 添加多个文件
  zzz lnk add ~/.bashrc ~/.vimrc ~/.gitconfig

  # 递归添加目录中的所有文件
  zzz lnk add ~/.config --recursive

  # 为特定主机添加配置
  zzz lnk add ~/.bashrc --host workstation`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			lnk := createLnkInstance(host)

			if !lnk.IsInitialized() {
				return fmt.Errorf("lnk 仓库未初始化，请先运行 'zzz lnk init'")
			}

			if recursive {
				progressCallback := func(current, total int, currentFile string) {
					util.Log.Printf("正在处理 (%d/%d): %s\n", current, total, currentFile)
				}

				if err := lnk.AddRecursiveWithProgress(args, progressCallback); err != nil {
					return err
				}

				util.Log.Successf("成功递归添加文件到 lnk 管理\n")
			} else if len(args) > 1 {
				if err := lnk.AddMultiple(args); err != nil {
					return err
				}

				util.Log.Successf("成功添加 %d 个文件到 lnk 管理\n", len(args))
			} else {
				if err := lnk.Add(args[0]); err != nil {
					return err
				}

				util.Log.Successf("成功添加文件 %s 到 lnk 管理\n", args[0])
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "递归添加目录中的所有文件")
	cmd.Flags().StringVar(&host, "host", "", "指定主机名（默认使用系统主机名）")

	return cmd
}

func newRmCmd() *cobra.Command {
	var host string

	cmd := &cobra.Command{
		Use:          "rm <file>... [flags]",
		Short:        "从 lnk 管理中移除文件",
		Long:         `从 lnk 管理中移除指定的文件`,
		SilenceUsage: true,
		Example: `  # 移除单个文件
  zzz lnk rm ~/.bashrc

  # 移除多个文件
  zzz lnk rm ~/.bashrc ~/.vimrc

  # 移除特定主机的配置
  zzz lnk rm ~/.bashrc --host workstation`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			lnk := createLnkInstance(host)

			if !lnk.IsInitialized() {
				return fmt.Errorf("lnk 仓库未初始化，请先运行 'zzz lnk init'")
			}

			if len(args) > 1 {
				util.Log.Warnf("即将从 lnk 管理中移除 %d 个文件:\n", len(args))
				for _, file := range args {
					util.Log.Printf("  - %s\n", file)
				}

				util.Log.Warn("此操作将删除符号链接并恢复原始文件，是否继续? (y/N)")
				var response string
				fmt.Scanln(&response)
				if response != "y" && response != "Y" && response != "yes" && response != "Yes" {
					util.Log.Println("操作已取消")
					return nil
				}

				if err := lnk.RemoveMultiple(args); err != nil {
					return fmt.Errorf("批量移除文件失败: %w", err)
				}

				util.Log.Successf("成功从 lnk 管理中移除 %d 个文件\n", len(args))
			} else {
				file := args[0]
				util.Log.Warnf("即将从 lnk 管理中移除文件: %s\n", file)
				util.Log.Warn("此操作将删除符号链接并恢复原始文件，是否继续? (y/N)")

				var response string
				fmt.Scanln(&response)
				if response != "y" && response != "Y" && response != "yes" && response != "Yes" {
					util.Log.Println("操作已取消")
					return nil
				}

				if err := lnk.Remove(file); err != nil {
					return fmt.Errorf("移除文件失败: %w", err)
				}

				util.Log.Successf("成功从 lnk 管理中移除文件: %s\n", file)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&host, "host", "", "指定主机名（默认使用系统主机名）")

	return cmd
}

func newListCmd() *cobra.Command {
	var (
		all  bool
		host string
	)

	cmd := &cobra.Command{
		Use:          "list [flags]",
		Short:        "列出管理的文件",
		Long:         `显示当前被 lnk 管理的文件列表`,
		SilenceUsage: true,
		Example: `  # 显示当前主机的配置文件
  zzz lnk list

  # 显示所有主机的配置文件
  zzz lnk list --all

  # 显示特定主机的配置文件
  zzz lnk list --host workstation`,
		RunE: func(cmd *cobra.Command, args []string) error {
			lnk := createLnkInstance(host)

			if !lnk.IsInitialized() {
				return fmt.Errorf("lnk 仓库未初始化，请先运行 'zzz lnk init'")
			}

			if all {
				groups, err := lnk.ListAll()
				if err != nil {
					return fmt.Errorf("获取文件列表失败: %w", err)
				}

				total := 0
				for _, files := range groups {
					total += len(files)
				}
				if total == 0 {
					util.Log.Println("当前没有管理任何文件")
					return nil
				}

				util.Log.Printf("管理的文件列表 (共 %d 个文件):\n", total)

				if common, ok := groups["general"]; ok && len(common) > 0 {
					sort.Strings(common)
					util.Log.Printf("\n通用文件 (%d 个):\n", len(common))
					for _, file := range common {
						util.Log.Printf("  - %s\n", file)
					}
				}

				var hosts []string
				for k := range groups {
					if k == "general" {
						continue
					}
					hosts = append(hosts, k)
				}
				sort.Strings(hosts)
				for _, h := range hosts {
					files := groups[h]
					sort.Strings(files)
					util.Log.Printf("\n主机 %s (%d 个):\n", h, len(files))
					for _, file := range files {
						util.Log.Printf("  - %s\n", file)
					}
				}
			} else {
				files, err := lnk.List()
				if err != nil {
					return fmt.Errorf("获取文件列表失败: %w", err)
				}

				if len(files) == 0 {
					if host != "" {
						util.Log.Printf("主机 %s 当前没有管理任何文件\n", host)
					} else {
						util.Log.Println("当前主机没有管理任何文件")
					}
					return nil
				}

				if host != "" {
					util.Log.Printf("主机 %s 管理的文件列表 (共 %d 个文件):\n", host, len(files))
				} else {
					util.Log.Printf("当前主机管理的文件列表 (共 %d 个文件):\n", len(files))
				}

				for _, file := range files {
					util.Log.Printf("  %s\n", file)
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&all, "all", "a", false, "显示所有主机的配置文件")
	cmd.Flags().StringVar(&host, "host", "", "显示特定主机的配置文件")

	return cmd
}

func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "status",
		Short:        "显示仓库状态",
		Long:         `显示 lnk 仓库的当前状态`,
		SilenceUsage: true,
		Example: `  # 显示仓库状态
  zzz lnk status`,
		RunE: func(cmd *cobra.Command, args []string) error {
			lnk := createLnkInstance("")

			if !lnk.IsInitialized() {
				return fmt.Errorf("lnk 仓库未初始化，请先运行 'zzz lnk init'")
			}

			status, err := lnk.Status()
			if err != nil {
				return fmt.Errorf("获取仓库状态失败: %w", err)
			}

			util.Log.Println("lnk 仓库状态:")
			util.Log.Printf("仓库路径: %s\n", status.RepoPath)
			util.Log.Printf("当前主机: %s\n", status.Host)

			if status.GitStatus != nil {
				if status.GitStatus.Remote != "" {
					util.Log.Printf("远程仓库: %s\n", status.GitStatus.Remote)
				} else {
					util.Log.Println("远程仓库: 未配置")
				}

				if status.GitStatus.Remote != "" {
					if status.GitStatus.Ahead > 0 && status.GitStatus.Behind > 0 {
						util.Log.Warnf("同步状态: 领先 %d 个提交，落后 %d 个提交\n", status.GitStatus.Ahead, status.GitStatus.Behind)
					} else if status.GitStatus.Ahead > 0 {
						util.Log.Printf("同步状态: 领先远程 %d 个提交\n", status.GitStatus.Ahead)
					} else if status.GitStatus.Behind > 0 {
						util.Log.Warnf("同步状态: 落后远程 %d 个提交\n", status.GitStatus.Behind)
					} else {
						util.Log.Successf("同步状态: 与远程仓库同步\n")
					}
				}

				if status.GitStatus.Dirty {
					util.Log.Warn("工作区状态: 有未提交的变更")
				} else {
					util.Log.Successf("工作区状态: 干净，没有未提交的变更\n")
				}
			}

			util.Log.Printf("\n管理文件统计: 共 %d 个文件\n", status.ManagedFiles)

			if len(status.BrokenLinks) > 0 {
				util.Log.Warnf("\n损坏的符号链接 (%d 个):\n", len(status.BrokenLinks))
				for _, link := range status.BrokenLinks {
					util.Log.Warnf("  - %s\n", link)
				}
			} else {
				util.Log.Successf("\n所有符号链接状态正常\n")
			}

			return nil
		},
	}

	return cmd
}

func newPushCmd() *cobra.Command {
	var host string

	cmd := &cobra.Command{
		Use:          "push [message] [flags]",
		Short:        "推送变更到远程仓库",
		Long:         `将本地变更推送到远程仓库`,
		SilenceUsage: true,
		Example: `  # 推送变更（使用默认消息）
  zzz lnk push

  # 推送变更并指定提交消息
  zzz lnk push "更新 vim 配置"

  # 推送特定主机的变更
  zzz lnk push "更新工作站配置" --host workstation`,
		RunE: func(cmd *cobra.Command, args []string) error {
			lnk := createLnkInstance(host)

			if !lnk.IsInitialized() {
				return fmt.Errorf("lnk 仓库未初始化，请先运行 'zzz lnk init'")
			}

			var message string
			if len(args) > 0 {
				message = args[0]
			}

			committed, err := lnk.Push(message)
			if err != nil {
				return fmt.Errorf("推送失败: %w", err)
			}

			if committed {
				if message != "" {
					util.Log.Successf("成功推送变更到远程仓库，提交消息: %s\n", message)
				} else {
					util.Log.Successf("成功推送变更到远程仓库\n")
				}
			} else {
				util.Log.Successf("无新增提交，已推送远程（工作区干净）\n")
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&host, "host", "", "指定主机名（默认使用系统主机名）")

	return cmd
}

func newPullCmd() *cobra.Command {
	var host string

	cmd := &cobra.Command{
		Use:          "pull [flags]",
		Short:        "从远程仓库拉取变更",
		Long:         `从远程仓库拉取最新变更`,
		SilenceUsage: true,
		Example: `  # 拉取变更
  zzz lnk pull

  # 拉取特定主机的变更
  zzz lnk pull --host workstation`,
		RunE: func(cmd *cobra.Command, args []string) error {
			lnk := createLnkInstance(host)

			if !lnk.IsInitialized() {
				return fmt.Errorf("lnk 仓库未初始化，请先运行 'zzz lnk init'")
			}

			if err := lnk.Pull(); err != nil {
				return fmt.Errorf("拉取失败: %w", err)
			}

			if host != "" {
				if err := lnk.RestoreSymlinksForHost(host); err != nil {
					util.Log.Warnf("恢复主机 %s 的符号链接时出现问题: %v\n", host, err)
				} else {
					util.Log.Successf("成功恢复主机 %s 的符号链接\n", host)
				}
			}

			util.Log.Successf("成功从远程仓库拉取变更并恢复符号链接\n")
			return nil
		},
	}

	cmd.Flags().StringVar(&host, "host", "", "指定主机名（默认使用系统主机名）")

	return cmd
}

func newBootstrapCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "bootstrap",
		Short:        "运行引导脚本",
		Long:         `手动运行 bootstrap.sh 引导脚本`,
		SilenceUsage: true,
		Example: `  # 运行引导脚本
  zzz lnk bootstrap`,
		RunE: func(cmd *cobra.Command, args []string) error {
			lnk := createLnkInstance("")

			if !lnk.IsInitialized() {
				return fmt.Errorf("lnk 仓库未初始化，请先运行 'zzz lnk init'")
			}

			scriptPath, err := lnk.FindBootstrapScript()
			if err != nil {
				return fmt.Errorf("未找到 bootstrap 脚本: %w", err)
			}

			util.Log.Printf("找到 bootstrap 脚本: %s\n", scriptPath)
			util.Log.Warn("即将执行 bootstrap 脚本，此操作可能会修改系统配置或安装软件包")
			util.Log.Warn("是否继续执行? (y/N)")

			var response string
			fmt.Scanln(&response)
			if response != "y" && response != "Y" && response != "yes" && response != "Yes" {
				util.Log.Println("操作已取消")
				return nil
			}

			util.Log.Println("正在执行 bootstrap 脚本...")
			if err := lnk.RunBootstrapScript(); err != nil {
				return fmt.Errorf("执行 bootstrap 脚本失败: %w", err)
			}

			util.Log.Successf("bootstrap 脚本执行完成\n")
			return nil
		},
	}

	return cmd
}
