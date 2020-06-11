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
	Short:   "Daily development aids",
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
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	// var defConfig string
	var versionText = fmt.Sprintf("version %s\n", version)
	//noinspection GoBoolExpressions
	if buildTime != "" {
		versionText = fmt.Sprintf("%sbuild time %s\n", versionText, buildTime)
	}
	//noinspection GoBoolExpressions
	if buildGoVersion != "" {
		versionText = fmt.Sprintf("%s%s\n", versionText, buildGoVersion)
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
		`Examples:`, `{{StyleHeading "Examples:"}}`,
		`Usage:`, `{{StyleHeading "Usage:"}}`,
		`Aliases:`, `{{StyleHeading "Aliases:"}}`,
		`Available Commands:`, `{{StyleHeading "Available Commands:"}}`,
		`Global Flags:`, `{{StyleHeading "Global Flags:"}}`,
		`Flags:`, `{{StyleHeading "Flags:"}}`,
	).Replace(usageTemplate)
	re := regexp.MustCompile(`(?m)^Flags:\s*$`)
	usageTemplate = re.ReplaceAllLiteralString(usageTemplate, `{{StyleHeading "Flags:"}}`)
	rootCmd.SetUsageTemplate(usageTemplate)
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
	if lastNow != 0 && ((now - lastNow) < 600) {
		c <- struct{}{}
		return
	}
	updateDetectionTime(now)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(zstring.RandInt(1, 4))*time.Second)
	defer cancel()
	res, err := zhttp.Get("https://api.github.com/repos/sohaha/zzz/releases/latest", ctx)
	if err != nil {
		zlog.Debug(err)
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
