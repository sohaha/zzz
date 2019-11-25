package cmd

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/mitchellh/go-homedir"
	"github.com/sohaha/zzz/app/filemanager/gui"
	"github.com/spf13/cobra"
)

// copyright: github.com/skanehira/ff
var fileManagerCmd = &cobra.Command{
	Use:     "filemanager",
	Aliases: []string{"f"},
	Short:   "Terminal file management tool",
	Long:    ``,
	Run: func(cmd *cobra.Command, args []string) {
		os.Exit(run())
	},
}

func init() {
	rootCmd.AddCommand(fileManagerCmd)
	enablePreview = fileManagerCmd.PersistentFlags().BoolP("preview", "p", true, "enable preview panel")
	// enableLog = fileManagerCmd.PersistentFlags().BoolP("log", "", false, "enable log")
	ignorecase = fileManagerCmd.PersistentFlags().BoolP("ignorecase", "i", false, "ignore case when searcing")
	// viper.BindEnv("editor", "EDITOR")
	// fmt.Println(viper.Get("editor"))
}

var (
	enableLog      = new(bool)
	enablePreview  *bool
	ignorecase     *bool
	ErrGetHomeDir  = errors.New("cannot get home dir")
	ErrOpenLogFile = errors.New("cannot open log file")
)

func run() int {
	if err := initLogger(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		return 1
	}

	if err := gui.New(*enablePreview, *ignorecase).Run(); err != nil {
		return 1
	}

	return 0
}

func initLogger() error {
	var logWriter io.Writer
	if *enableLog {
		home, err := homedir.Dir()
		if err != nil {
			return fmt.Errorf("%s: %s", ErrGetHomeDir, err)
		}

		logWriter, err = os.OpenFile(filepath.Join(home, "ff.log"),
			os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)

		if err != nil {
			return fmt.Errorf("%s: %s", ErrOpenLogFile, err)
		}
		log.SetFlags(log.Lshortfile)
	} else {
		// no print log
		logWriter = ioutil.Discard
	}

	log.SetOutput(logWriter)
	return nil
}
