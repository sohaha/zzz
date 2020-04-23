package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/sohaha/zlsgo/zfile"
	"github.com/sohaha/zlsgo/zshell"
	"github.com/sohaha/zlsgo/zstring"
	"github.com/sohaha/zzz/app/build"
	"github.com/sohaha/zzz/util"
	"github.com/spf13/cobra"
)

var (
	isVendor    bool
	buildIgnore bool
	isPack      bool
	skipStatic  bool
	isCGO       bool
	cross       string
	goVersion   string
	buildUse    = "build"
	outDir      string
	GOPROXY     = os.Getenv("GOPROXY")
)

var buildCmd = &cobra.Command{
	Use:   buildUse,
	Short: "Generates asset packs replace 'go build'",
	Args:  cobra.ArbitraryArgs,
	Example: fmt.Sprintf(`  %s %s 
  %[1]s %[2]s --pack -- -o output
  %[1]s %[2]s --os win,mac,linux --go 1.11`, use, buildUse),
	Run: func(cmd *cobra.Command, args []string) {
		dirPath := zfile.RealPath(".", true)
		name := build.Basename(dirPath)
		if !skipStatic {
			mewnFiles := build.GetMewnFiles([]string{}, buildIgnore)
			targetFiles := make([]string, 0)
			if len(mewnFiles) > 0 {
				referencedAssets, err := build.GetReferencedAssets(mewnFiles)
				util.CheckIfError(err)
				for _, referencedAsset := range referencedAssets {
					packfileData, err := build.GeneratePackFileString(referencedAsset, buildIgnore)
					util.CheckIfError(err)
					targetFile := filepath.Join(referencedAsset.BaseDir, referencedAsset.PackageName+"____tmp.go")
					targetFiles = append(targetFiles, targetFile)
					err = ioutil.WriteFile(targetFile, []byte(packfileData), 0644)
					util.CheckIfError(err)
				}
			}
			defer func() {
				for _, filename := range targetFiles {
					_ = os.Remove(filename)
				}
			}()
		}
		buildArgs := args
		buildArgs = append(buildArgs, ` -ldflags`)
		ldflags := zstring.Buffer()
		ldflags.WriteString(` "`)
		ldflags.WriteString(` -X 'main.BUILD_COMMIT=` + build.GetBuildGitID() + `'`)
		ldflags.WriteString(` -X 'main.BUILD_GOVERSION=` + build.GetGoVersion() + `'`)
		ldflags.WriteString(` -X 'main.BUILD_TIME=` + build.GetBuildTime() + `'`)
		ldflags.WriteString(` -X 'github.com/sohaha/zlsgo/zcli.BuildTime=` + build.GetBuildTime() + `'`)
		ldflags.WriteString(` -X 'github.com/sohaha/zlsgo/zcli.BuildGoVersion=` + build.GetGoVersion() + `'`)
		ldflags.WriteString(` -X 'github.com/sohaha/zlsgo/zcli.BuildGitCommitID=` + build.GetBuildGitID() + `'`)
		if isPack {
			ldflags.WriteString(` -w -s `)
		}
		ldflags.WriteString(` "`)

		buildArgs = append(buildArgs, ldflags.String())
		if zfile.DirExist(dirPath + "vendor") {
			isVendor = true
		}

		if GOPROXY != "" {
			GOPROXY = "GOPROXY=" + GOPROXY
		}
		if outDir != "" && !strings.HasSuffix(outDir, "/") {
			outDir = outDir + "/"
		}
		targets := make([]build.OSData, 0)
		for _, v := range build.ParserTarget(cross) {
			for k, v := range build.TargetsCommad(v) {
				for _, a := range v {
					targets = append(targets, build.OSData{
						Goos:   k,
						Goarch: a,
					})
				}
			}
		}
		buildCommads := build.CommadString(targets, isVendor, isCGO, name, outDir)
		if !isDocker() {
			for _, v := range buildCommads {
				localCommad(v, buildArgs)
			}
			return
		}
		if err := build.CheckDocker(); err != nil {
			util.Log.Fatalf("Failed to check docker: %v\n", err)
		}
		if goVersion == "" {
			goVersion = "latest"
		}
		found, err, image := build.CheckDockerImage(goVersion)
		switch {
		case err != nil:
			util.Log.Fatalf("Failed to check docker image availability: %v", err)
		case !found:
			if err := build.PullDockerImage(image); err != nil {
				util.Log.Fatalf("Failed to pull docker image from the registry: %v", err)
			}
		}
		buildGoversionOld := ` -X 'main.BUILD_GOVERSION=` + build.GetGoVersion() + `'`
		buildGoversionNew := ` -X 'main.BUILD_GOVERSION=` + build.DockerDist + goVersion + `'`
		if goVersion == "1.11" {
			GOPROXY = "GO111MODULE=on " + strings.Split(GOPROXY, ",")[0]
		}
		for _, v := range buildCommads {
			v = GOPROXY + " " + v + strings.Join(buildArgs, " ")
			v = strings.Replace(v, buildGoversionOld, buildGoversionNew, 1)
			cmd := strings.Split(fmt.Sprintf(`docker run --rm -v %s:/app -w /app seekwe/go-builder:%s sh -c`, dirPath, goVersion), " ")
			cmd = append(cmd, v)
			name, err := zstring.RegexExtract(`-o=([\w\\\/\-\_\.]*) `, v)
			if err == nil && len(name) > 1 {
				util.Log.Printf("Build %s ...\n", name[1])
			}
			_, _, _, err = zshell.ExecCommand(cmd, nil, os.Stdout, os.Stderr)
			if err != nil {
				util.Log.Fatalf("Failed to check docker image availability: %v\n", err)
			}
		}
	},
}

func localCommad(v string, buildArgs []string) {
	v = strings.Trim(v, " ")
	osEnv := os.Environ()
	envs := strings.Split(v, " ")
	for i, vv := range envs {
		if vv == "go" {
			v = strings.Join(envs[i:], " ")
			break
		}
		osEnv = append(osEnv, vv)
	}
	zshell.Env = osEnv
	cmd := strings.Split(v, " ")
	cmd = append(cmd, buildArgs...)
	_, _, _, err := zshell.ExecCommand(cmd, nil, os.Stdout, os.Stderr)
	if err != nil {
		util.Log.Fatalf("%v\n", err)
	}
}

func isDocker() bool {
	if goVersion == "" && !isCGO {
		return false
	}
	return true
}

func init() {
	rootCmd.AddCommand(buildCmd)
	buildCmd.Flags().BoolVarP(&skipStatic, "skip-static", "", false, "skip static analysis, do not use package static file function")
	buildCmd.Flags().BoolVarP(&isPack, "pack", "P", false, "Same as build, will compile with '-w -s' flags")
	buildCmd.Flags().StringVarP(&cross, "os", "O", "", "Cross-compile, compile to the specified system application, use more ',' separate")
	buildCmd.Flags().StringVarP(&outDir, "out", "", "", "Output directory")
	buildCmd.Flags().BoolVarP(&isCGO, "cgo", "", false, "Turn on CGO_ENABLED, need to install docker")
	buildCmd.Flags().StringVarP(&goVersion, "go", "G", "", "specify go version, need to install docker")
	buildCmd.Flags().BoolVarP(&buildIgnore, "ignoreE", "I", false, "Ignore files that don't exist")
}
