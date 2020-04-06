/*
 * @Author: seekwe
 * @Date: 2020-01-03 13:42:31
 * @Last Modified by:: seekwe
 * @Last Modified time: 2020-04-04 17:34:50
 */
package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/sohaha/zzz/util"
	lib "github.com/sohaha/zzz/util/static"

	"github.com/spf13/cobra"
)

var (
	buildIgnore bool
	isPack      bool
	buildUse    = "build"
)

// copyright: https://github.com/leaanthony/mewn
var buildCmd = &cobra.Command{
	Use:   buildUse,
	Short: "Generates asset packs replace 'go build'",
	Args:  cobra.ArbitraryArgs,
	Example: fmt.Sprintf(`  %s %s -- -o output 
  %[1]s %[2]s --pack -- -o output`, use, buildUse),
	Run: func(cmd *cobra.Command, args []string) {
		mewnFiles := lib.GetMewnFiles([]string{}, buildIgnore)
		targetFiles := make([]string, 0)
		if len(mewnFiles) > 0 {
			referencedAssets, err := lib.GetReferencedAssets(mewnFiles)
			util.CheckIfError(err)
			for _, referencedAsset := range referencedAssets {
				packfileData, err := lib.GeneratePackFileString(referencedAsset, buildIgnore)
				util.CheckIfError(err)
				targetFile := filepath.Join(referencedAsset.BaseDir, referencedAsset.PackageName+"____tmp.go")
				targetFiles = append(targetFiles, targetFile)
				err = ioutil.WriteFile(targetFile, []byte(packfileData), 0644)
				util.CheckIfError(err)
			}
		}

		buildArgs := args
		cmdargs := make([]string, 0)
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
			_ = os.Remove(filename + "..")
		}
	},
}

func init() {
	rootCmd.AddCommand(buildCmd)
	buildCmd.Flags().BoolVarP(&isPack, "pack", "P", false, "Same as build, will compile with '-w -s' flags")
	buildCmd.Flags().BoolVarP(&buildIgnore, "ignoreE", "I", false, "Ignore files that don't exist")
}
