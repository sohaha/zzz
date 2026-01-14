package cmd

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/sohaha/zlsgo/zfile"
	"github.com/sohaha/zlsgo/zhttp"
	"github.com/sohaha/zlsgo/zjson"
	"github.com/sohaha/zlsgo/zlog"
	"github.com/sohaha/zlsgo/zstring"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/sohaha/zzz/app/root"
	"github.com/sohaha/zzz/util"
)

const (
	cfgFilename = util.CfgFilepath + util.CfgFilename + util.CfgFileExt
)

var (
	use            = "zzz"
	version        = util.Version
	buildTime      = util.BuildTime
	buildGoVersion = util.BuildGoVersion
	homePath       string
	cfgFile        string
)

var rootCmd = &cobra.Command{
	Use:     use,
	Short:   "日常开发辅助工具",
	Long:    ``,
	Version: version,
	Run: func(cmd *cobra.Command, args []string) {
		var dujt string
		if viper.GetBool("other.du") {
			dujt = "\n" + util.GetLineDujt()
		}

		logo := fmt.Sprintf(`  _____
 / _  /________
 \// /|_  /_  /
  / //\/ / / /
 /____/___/___| v%s%s
`, version, dujt)

		fmt.Println(logo)
		_ = cmd.Help()
	},
}

func Execute() {
	localizeHelpFlags(rootCmd)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func localizeHelpFlags(cmd *cobra.Command) {
	cmd.InitDefaultHelpFlag()
	if helpFlag := cmd.Flags().Lookup("help"); helpFlag != nil {
		if cmd == rootCmd {
			helpFlag.Usage = "显示帮助信息"
		} else {
			helpFlag.Usage = "显示 " + cmd.Name() + " 的帮助信息"
		}
	}
	for _, subcmd := range cmd.Commands() {
		localizeHelpFlags(subcmd)
	}
}

func init() {
	// var defConfig string
	versionText := fmt.Sprintf("version %s\n", version)
	//noinspection GoBoolExpressions
	if buildTime != "" {
		versionText = fmt.Sprintf("%s构建时间 %s\n", versionText, buildTime)
	}
	//noinspection GoBoolExpressions
	if buildGoVersion != "" {
		versionText = fmt.Sprintf("%sGo 版本 %s\n", versionText, buildGoVersion)
	}
	rootCmd.SetVersionTemplate(versionText)
	homePath = util.GetHome()
	// if homePathErr == nil {
	// 	defConfig = fmt.Sprintf("config file (default is $HOME/%s)", cfgFilename)
	// }
	// rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "", "", defConfig)
	initConfig()
	// cobra.OnInitialize(initConfig)
	cobra.AddTemplateFunc("StyleHeading", func(e string) string {
		return zlog.ColorTextWrap(zlog.ColorGreen, e)
	})
	cobra.AddTemplateFunc("StyleTip", func(s string, padding int) string {
		template := fmt.Sprintf("%%-%ds", padding)
		return zlog.ColorTextWrap(zlog.ColorYellow, fmt.Sprintf(template, s))
	})
	cobra.AddTemplateFunc("StyleAliases", func(s string) string {
		return zlog.ColorTextWrap(zlog.ColorLightBlue, s)
	})
	usageTemplate := rootCmd.UsageTemplate()
	usageTemplate = strings.NewReplacer(
		`{{.NameAndAliases}}`, `{{StyleAliases .NameAndAliases}}`,
		`{{rpad .Name .NamePadding }}`, `{{StyleTip .Name .NamePadding }}`,
		`Examples:`, `{{StyleHeading "示例:"}}`,
		`Usage:`, `{{StyleHeading "用法:"}}`,
		`Aliases:`, `{{StyleHeading "别名:"}}`,
		`Available Commands:`, `{{StyleHeading "可用命令:"}}`,
		`Global Flags:`, `{{StyleHeading "全局参数:"}}`,
		`Flags:`, `{{StyleHeading "参数:"}}`,
		`Use "{{.CommandPath}} [command] --help" for more information about a command.`, `使用 "{{.CommandPath}} [command] --help" 查看命令的详细信息。`,
		`Additional help topics:`, `{{StyleHeading "其他帮助主题:"}}`,
		`Use "{{.CommandPath}} [command] --help" for more information about that command.`, `使用 "{{.CommandPath}} [command] --help" 查看该命令的详细信息。`,
	).Replace(usageTemplate)
	usageTemplate = strings.ReplaceAll(usageTemplate, "help for {{.Name}}", "显示 {{.Name}} 的帮助信息")
	re := regexp.MustCompile(`(?m)^Flags:\s*$`)
	usageTemplate = re.ReplaceAllLiteralString(usageTemplate, `{{StyleHeading "参数:"}}`)
	rootCmd.SetUsageTemplate(usageTemplate)

	rootCmd.InitDefaultHelpFlag()
	rootCmd.Flags().Lookup("help").Usage = "显示帮助信息"
	rootCmd.InitDefaultVersionFlag()
	rootCmd.Flags().Lookup("version").Usage = "显示版本信息"

	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.SetHelpCommand(&cobra.Command{
		Use:    "help [command]",
		Short:  "查看命令帮助信息",
		Long:   "查看任意命令的详细帮助信息",
		Hidden: false,
		Run: func(c *cobra.Command, args []string) {
			cmd, _, e := c.Root().Find(args)
			if cmd == nil || e != nil {
				c.Printf("未知命令 \"%s\"\n", args)
				_ = c.Root().Usage()
			} else {
				cmd.InitDefaultHelpFlag()
				_ = cmd.Help()
			}
		},
	})
	rootCmd.AddCommand(&cobra.Command{
		Use:                   "completion [bash|zsh|fish|powershell]",
		Short:                 "生成命令行补全脚本",
		Long:                  "为指定的 shell 生成自动补全脚本",
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		Run: func(cmd *cobra.Command, args []string) {
			switch args[0] {
			case "bash":
				_ = cmd.Root().GenBashCompletion(os.Stdout)
			case "zsh":
				_ = cmd.Root().GenZshCompletion(os.Stdout)
			case "fish":
				_ = cmd.Root().GenFishCompletion(os.Stdout, true)
			case "powershell":
				_ = cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
			}
		},
	})

	zfile.ProjectPath, _ = os.Getwd()
}

func initConfig() {
	cfgFilepath := homePath + "/" + cfgFilename
	_ = createCfg(cfgFilepath)
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.AddConfigPath(homePath)
		viper.SetConfigName(strings.TrimSuffix(cfgFilename, ".yaml"))
	}
	viper.SetEnvPrefix("ZZZ_")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		// fmt.Println("Using config file:", viper.ConfigFileUsed(), viper.AllSettings())
	}
	// _ = updateCfg(cfgFilepath)
}

func createCfg(cfgFilepath string) error {
	if !zfile.FileExist(cfgFilepath) {
		config := root.GetExampleConfig(version)
		zfile.RealPathMkdir(filepath.Dir(cfgFilepath))
		return ioutil.WriteFile(cfgFilepath, []byte(config), 0644)
	}
	return nil
}

func updateCfg(cfgFilepath string) error {
	return viper.WriteConfigAs(cfgFilepath)
}

func updateDetectionTime(now int64) {
	viper.Set("core.detection_time", now)
	_ = viper.WriteConfig()
}

func GetNewVersion(c chan struct{}) {
	now := time.Now().Unix()
	lastNow := viper.GetInt64("core.detection_time")
	if lastNow != 0 && ((now - lastNow) < 60*60*24) {
		c <- struct{}{}
		return
	}
	updateDetectionTime(now)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(zstring.RandInt(1, 3))*time.Second)
	defer cancel()
	res, err := zhttp.Get("https://api.github.com/repos/sohaha/zzz/releases/latest", ctx)
	if err != nil {
		c <- struct{}{}
		return
	}
	json := zjson.ParseBytes(res.Bytes())
	version := strings.TrimPrefix(json.Get("tag_name").String(), "v")
	if util.VersionCompare(util.Version, version, "<") {
		util.Log.Warnf("New version (v%v) is released, please go to GitHub to update: https://github.com/sohaha/zzz\n", version)
	}
	c <- struct{}{}
}
