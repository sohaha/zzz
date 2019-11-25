package cmd

import (
	"fmt"
	"github.com/sohaha/zlsgo/zfile"
	"github.com/sohaha/zlsgo/zlog"
	"github.com/sohaha/zzz/app/root"
	"github.com/sohaha/zzz/util"
	"github.com/spf13/viper"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"
	
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"os"
)

const cfgFilename = ".zzz/config.yaml"

var (
	use            = "zzz"
	version        = "1.2.2"
	buildTime      = ""
	buildGoVersion = ""
	homePath       string
	homePathErr    error
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
	homePath, homePathErr = homedir.Dir()
	// if homePathErr == nil {
	// 	defConfig = fmt.Sprintf("config file (default is $HOME/%s)", cfgFilename)
	// }
	// rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "", "", defConfig)
	cobra.OnInitialize(initConfig)
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
		if homePathErr != nil {
			util.Log.Fatal(homePathErr)
		}
		viper.AddConfigPath(homePath)
		viper.SetConfigName(strings.TrimSuffix(cfgFilename, ".yaml"))
	}
	viper.SetEnvPrefix("ZZZ_")
	viper.AutomaticEnv()
	
	if err := viper.ReadInConfig(); err == nil {
		// fmt.Println("Using config file:", viper.ConfigFileUsed())
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
