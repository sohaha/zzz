package cmd

import (
	"github.com/sohaha/zzz/util"
	lib "github.com/sohaha/zzz/util/static"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	buildIgnore bool
	isPack      bool
)

// copyright: https://github.com/leaanthony/mewn
var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Generates asset packs replace 'go build'",
	Args:  cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		mewnFiles := lib.GetMewnFiles([]string{}, buildIgnore)
		targetFiles := make([]string, 0)
		if len(mewnFiles) > 0 {
			referencedAssets, err := lib.GetReferencedAssets(mewnFiles)
			if err != nil {
				util.Log.Fatal(err)
			}
			for _, referencedAsset := range referencedAssets {
				packfileData, err := lib.GeneratePackFileString(referencedAsset, buildIgnore)
				util.CheckIfError(err)
				targetFile := filepath.Join(referencedAsset.BaseDir, referencedAsset.PackageName+"____.go")
				targetFiles = append(targetFiles, targetFile)
				err = ioutil.WriteFile(targetFile, []byte(packfileData), 0644)
				util.CheckIfError(err)
			}
		}

		buildArgs := args
		var cmdargs []string

		cmdargs = append(cmdargs, "build")
		cmdargs = append(cmdargs, buildArgs...)
		if isPack {
			cmdargs = append(cmdargs, "-ldflags")
			cmdargs = append(cmdargs, "-w -s")
		}
		cmdRun := exec.Command("go", cmdargs...)
		stdoutStderr, err := cmdRun.CombinedOutput()
		if err != nil {
			util.Log.Errorf("Error running command %s\n", cmdargs)
			util.Log.Infof("From program: %s\n", stdoutStderr)
		}

		for _, filename := range targetFiles {
			_ = os.Remove(filename)
		}
	},
}

func init() {
	rootCmd.AddCommand(buildCmd)
	buildCmd.Flags().BoolVarP(&isPack, "pack", "P", false, "Same as build, will compile with '-w -s' flags")
	buildCmd.Flags().BoolVarP(&buildIgnore, "ignoreE", "I", false, "Ignore files that don't exist")
}
