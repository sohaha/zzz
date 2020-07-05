package build

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/sohaha/zlsgo/zenv"
	"github.com/sohaha/zlsgo/zshell"
	"github.com/sohaha/zlsgo/zutil"

	"github.com/sohaha/zzz/util"
)

var DockerDist = "seekwe/go-builder:"

type OSData struct {
	Goos   string
	Goarch string
	CXX    string
	CC     string
}

func CheckDocker() error {
	util.Log.Println("Checking docker ...")
	if _, _, _, err := zshell.Run("docker version"); err != nil {
		return err
	}
	return nil
}

func CheckDockerImage(goVersion string) (bool, error, string) {
	image := DockerDist + goVersion
	util.Log.Printf("Checking for required docker image %s ...\n", image)
	out, err := exec.Command("docker", "images", "--no-trunc").Output()
	if err != nil {
		return false, err, ""
	}
	res := strings.SplitN(image, ":", 2)
	r, t := res[0], res[1]
	match, _ := regexp.Match(fmt.Sprintf(`%s\s+%s`, r, t), out)
	return match, nil, image
}

func PullDockerImage(image string) error {
	util.Log.Printf("Pulling %s from docker registry...\n", image)
	_, _, _, err := zshell.ExecCommand(context.Background(), []string{"docker", "pull", image}, nil, os.Stdout, os.Stderr)
	return err
}

func CommadString(os []OSData, isVendor, isCGO bool, packageName, outDir string) (commad []string) {
	cgo := ""
	vendor := ""
	if isCGO {
		cgo = " CGO_ENABLED=1"
	}
	if isVendor {
		vendor = " -mod=vendor "
	}
	for _, v := range os {
		cc := ""
		if isCGO {
			switch v.Goos {
			case "windows":
				if v.Goarch == "386" {
					cc = fmt.Sprintf("CC=%s CXX=%s", "i686-w64-mingw32-gcc", "i686-w64-mingw32-g++")
				} else {
					cc = fmt.Sprintf("CC=%s CXX=%s", "x86_64-w64-mingw32-gcc", "x86_64-w64-mingw32-g++")
				}
			case "darwin":
				if v.Goarch == "386" {
					cc = fmt.Sprintf("CC=%s CXX=%s", "o32-clang", "o32-clang++")
				} else {
					cc = fmt.Sprintf("CC=%s CXX=%s", "o64-clang", "o64-clang++")
				}
			case "linux":
				if v.Goarch == "386" {
					cc = "HOST=i686-linux PREFIX=/usr/local"
				}
			}

		}
		name := packageName + "_" + v.Goos + "_" + v.Goarch
		commad = append(commad, fmt.Sprintf("%s%s GOARCH=%s GOOS=%s go build -o=%s%s%s", cc, cgo, v.Goarch, v.Goos, outDir, zutil.IfVal(v.Goos == "windows", name+".exe", name).(string), vendor))
	}

	if len(commad) == 0 {
		cmd := "go build " + vendor
		commad = []string{cmd}
		if outDir != "" {
			name := packageName
			commad = []string{cmd + " -o=" + outDir + zutil.IfVal(zenv.IsWin(), name+".exe", name).(string)}
		}
	}
	return
}

func TargetsCommad(target string) map[string][]string {
	var (
		goos   = target
		goarch = ""
		commad = map[string][]string{}
	)
	t := strings.Split(target, "/")
	if len(t) > 1 {
		goos = t[0]
		goarch = t[1]
	}
	switch goos {
	case "w", "win", "windows":
		commad["windows"] = ParserArch(goarch, []string{"386", "amd64"})
	case "l", "linux":
		commad["linux"] = ParserArch(goarch, []string{"386", "amd64"})
	case "d", "darwin", "mac":
		commad["darwin"] = ParserArch(goarch, []string{"amd64"})
	case "android", "a":
		commad["android"] = ParserArch(goarch, []string{"arm64"})
	default:
		if goos == "" {
			break
		}
		if goarch == "*" || goarch == "" {
			util.Log.Fatalf("There is no GOARCH preset for %s, please complete it, for example: linux/amd64,windows/386\n", goos)
		}
		commad[goos] = ParserArch(goarch, []string{})
	}
	return commad
}

func ParserArch(arch string, lists []string) []string {
	archs := make([]string, 0)
	switch arch {
	case "", "*":
		archs = lists
	case "32":
		archs = append(archs, "386")
	case "64":
		archs = append(archs, "amd64")
	default:
		archs = append(archs, arch)
	}
	return archs
}

func ParserTarget(cross string) []string {
	targets := strings.Split(cross, ",")
	return targets
}
