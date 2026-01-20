package cmd

import (
	"github.com/spf13/cobra"
	"github.com/sohaha/zzz/app/shell"
)

var shellCmd = &cobra.Command{
	Use:   "shell",
	Short: "启动移动端优化的 Web Terminal",
	Long:  `通过浏览器访问的终端模拟器,支持触控操作和虚拟键盘`,
	Run: func(cmd *cobra.Command, args []string) {
		port, _ := cmd.Flags().GetString("port")
		host, _ := cmd.Flags().GetString("host")
		username, _ := cmd.Flags().GetString("user")
		password, _ := cmd.Flags().GetString("pass")
		shellPath, _ := cmd.Flags().GetString("shell")

		cfg := shell.Config{
			Port:     port,
			Host:     host,
			Username: username,
			Password: password,
			Shell:    shellPath,
		}

		if err := shell.Start(cfg); err != nil {
			panic(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(shellCmd)
	shellCmd.Flags().StringP("port", "p", "8080", "服务器端口")
	shellCmd.Flags().StringP("host", "H", "0.0.0.0", "监听地址")
	shellCmd.Flags().StringP("user", "u", "admin", "登录用户名")
	shellCmd.Flags().StringP("pass", "P", "", "登录密码 (留空随机生成)")
	shellCmd.Flags().StringP("shell", "s", "", "使用的 Shell (留空使用系统默认)")
}
