package cmd

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/sohaha/zlsgo/ztype"

	"github.com/sohaha/zlsgo/zfile"
	"github.com/sohaha/zlsgo/zshell"
	"github.com/sohaha/zlsgo/zstring"
	"github.com/sohaha/zlsgo/zutil"

	"github.com/spf13/cobra"

	zbuild "github.com/sohaha/zstatic/build"

	"github.com/sohaha/zzz/app/build"
	"github.com/sohaha/zzz/util"
)

var (
	isVendor      bool
	buildIgnore   bool
	isPack        bool
	skipStatic    bool
	buildStatic   bool
	buildTrimpath bool
	isCGO         bool
	buildDebug    bool
	cross         string
	goVersion     string
	skipDirs      string
	buildUse      = "build"
	obfuscate     int
	outDir        string
	GOPROXY       = os.Getenv("GOPROXY")
)

var buildCmd = &cobra.Command{
	Use:   buildUse,
	Short: "Generates asset packs replace 'go build'",
	Args:  cobra.ArbitraryArgs,
	Example: fmt.Sprintf(`  %s %s
  %[1]s %[2]s --pack -- -o output
  %[1]s %[2]s --os win,mac,linux`, use, buildUse),
	Run: func(cmd *cobra.Command, args []string) {
		version, versionNum := build.GetGoVersion(), float64(0)
		v := strings.Split(strings.Split(strings.Replace(version, "go", "", 1), " ")[0], ".")
		if len(v) > 1 {
			versionNum = ztype.ToFloat64(strings.Join(v[:2], "."))
		} else {
			versionNum = ztype.ToFloat64(strings.Join(v, "."))
		}

		if build.DisabledCGO() {
			zshell.Env = []string{"CGO_ENABLED=0"}
		}

		dirPath := zfile.RealPath(".", true)
		name := build.Basename(dirPath)
		existZlsGO := strings.Contains(build.ReadMod(dirPath), "/zlsgo")
		sd := zutil.IfVal(skipDirs == "", []string{}, strings.Split(skipDirs, ",")).([]string)
		if !skipStatic && !buildDebug {
			mewnFiles, err := zbuild.GetBinFiles([]string{}, buildIgnore, sd)
			if err != nil {
				util.Log.Fatal(err)
			}
			targetFiles := make([]string, 0)
			if len(mewnFiles) > 0 {
				referencedAssets, err := build.GetReferencedAssets(mewnFiles)
				util.CheckIfError(err)
				for _, referencedAsset := range referencedAssets {
					packfileData, err := build.GeneratePackFileString(referencedAsset, buildIgnore)
					util.CheckIfError(err)
					targetFile := filepath.Join(
						referencedAsset.BaseDir,
						referencedAsset.PackageName+"_static_resources.go",
					)
					targetFiles = append(targetFiles, targetFile)
					err = ioutil.WriteFile(targetFile, []byte(packfileData), 0o644)
					util.CheckIfError(err)
				}
			}
			defer func() {
				for _, filename := range targetFiles {
					if zutil.Getenv("NODELETETMP") == "" && !buildStatic {
						_ = os.Remove(filename)
					}
				}
			}()
		}
		if buildStatic {
			return
		}
		buildArgs := args
		ldflags := zstring.Buffer()
		ldflags.WriteString(`"`)
		ldflags.WriteString(`-X 'main.BUILD_COMMIT=` + build.GetBuildGitID() + `'`)
		ldflags.WriteString(` -X 'main.BUILD_GOVERSION=` + version + `'`)
		ldflags.WriteString(` -X 'main.BUILD_TIME=` + build.GetBuildTime() + `'`)
		if existZlsGO {
			ldflags.WriteString(
				` -X 'github.com/sohaha/zlsgo/zcli.BuildTime=` + build.GetBuildTime() + `'`,
			)
			ldflags.WriteString(` -X 'github.com/sohaha/zlsgo/zcli.BuildGoVersion=` + version + `'`)
			ldflags.WriteString(
				` -X 'github.com/sohaha/zlsgo/zcli.BuildGitCommitID=` + build.GetBuildGitID() + `'`,
			)
		}

		if buildTrimpath && versionNum > 1.13 &&
			(goVersion == "" || ztype.ToFloat64(goVersion) > 1.13) {
			buildArgs = append(buildArgs, `-trimpath`)
		}

		if isPack {
			ldflags.WriteString(` -w -s `)
		}

		ldflags.WriteString(`"`)
		buildArgs = append(buildArgs, `-ldflags`)
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
			targetsCommad := build.TargetsCommad(v)
			if targetsCommad == nil {
				return
			}
			for k, v := range targetsCommad {
				for _, a := range v {
					targets = append(targets, build.OSData{
						Goos:   k,
						Goarch: a,
					})
				}
			}
		}

		if isCGO {
			if err := build.CheckZig(); err != nil {
				util.Log.Fatal(err)
			}
		}
		buildCommads, envs := build.CommadString(targets, isVendor, isCGO, obfuscate, name, outDir)

		if goVersion == "" {
			goVersion = "latest"
		}
		for i, v := range buildCommads {
			localCommad(strings.Join(v, " "), buildArgs, envs[i])
		}
	},
}

func localCommad(v string, buildArgs []string, env []string) {
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

	oldEnv := zshell.Env
	defer func() {
		zshell.Env = oldEnv
	}()

	zshell.Env = append(zshell.Env, osEnv...)
	zshell.Env = append(zshell.Env, env...)
	cmd := strings.Split(v, " ")
	cmd = append(cmd, buildArgs...)

	cmds := make([]string, 0)
	for _, v := range cmd {
		v = strings.Trim(v, " ")
		v = strings.Trim(v, "\"")
		cmds = append(cmds, v)
	}
	if buildDebug {
		util.Log.Println(strings.Join(cmd, " "))
		return
	}

	e := zshell.Env
	_, _, _, err := zshell.ExecCommand(context.Background(), cmds, nil, os.Stdout, os.Stderr)
	zshell.Env = e
	if err != nil {
		util.Log.Fatalf("%v\n", err)
	}
}

func init() {
	rootCmd.AddCommand(buildCmd)
	buildCmd.Flags().
		BoolVarP(&skipStatic, "skip-static", "S", false, "Skip static analysis, do not use package static file function")
	buildCmd.Flags().
		BoolVarP(&isPack, "pack", "P", false, "Same as build, will compile with '-w -s' flags")
	buildCmd.Flags().
		StringVarP(&cross, "os", "O", "", "Cross-compile, compile to the specified system application, use more ',' separate")
	buildCmd.Flags().StringVarP(&outDir, "out", "", "", "Output directory")
	buildCmd.Flags().BoolVarP(&isCGO, "cgo", "C", false, "Turn on CGO_ENABLED, need to install zig")
	buildCmd.Flags().BoolVarP(&buildIgnore, "ignoreE", "I", false, "Ignore files that don't exist")
	buildCmd.Flags().BoolVar(&buildDebug, "debug", false, "Print execution command")
	buildCmd.Flags().BoolVar(&buildStatic, "static", false, "Compile only static resource files")
	buildCmd.Flags().
		BoolVarP(&buildTrimpath, "trimpath", "T", false, "Removes all file system paths from the compiled executable")
	buildCmd.Flags().StringVar(&skipDirs, "skip-dirs", "", "Directory to skip static analysis")
	buildCmd.Flags().IntVarP(&obfuscate, "garble", "G", 0, "Obfuscate code, 1: fast mode, 2: strong mode, need to install garble")
}
