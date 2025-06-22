package build

import (
	"runtime"
	"strings"

	"github.com/sohaha/zlsgo/zshell"
	"github.com/sohaha/zlsgo/zutil"

	"github.com/sohaha/zzz/util"
)

type OSData struct {
	Goos   string
	Goarch string
	CXX    string
	CC     string
}

func CheckZig() error {
	envs := zshell.Env
	defer func() {
		zshell.Env = envs
	}()
	if _, _, _, err := zshell.Run("zig version"); err != nil {
		return err
	}
	return nil
}

func CommadString(
	os []OSData,
	isVendor, isCGO, cShared bool, obfuscate int,
	NoStatic bool, ldflags string,
	packageName, outDir string,
) (commads [][]string, envs [][]string, goos []string) {
	vendor := ""
	envs = make([][]string, 0)
	goos = make([]string, 0)

	if isVendor {
		vendor = "-mod=vendor"
	}

	enableSanitize := zutil.Getenv("ZIGCC_ENABLE_SANITIZE", "0") == "1"
	appendSysroot := zutil.Getenv("ZIGCC_APPEND_SYSROOT", "0") == "1"
	zigccFlags := strings.TrimSpace(zutil.Getenv("ZIGCC_FLAGS", ""))
	if !enableSanitize && zigccFlags == "" && !cShared {
		zigccFlags = " -fno-sanitize=undefined -static"
	}

	if len(os) == 0 {
		osname := ""
		switch zutil.GetOs() {
		case "windows":
			osname = "windows"
		case "linux":
			osname = "linux"
		case "darwin":
			osname = "darwin"
		}
		arch := ""
		switch runtime.GOARCH {
		case "amd64":
			arch = "amd64"
		case "arm64":
			arch = "arm64"
		case "386":
			arch = "386"
		}
		if osname != "" && arch != "" {
			os = []OSData{
				{
					Goos:   osname,
					Goarch: arch,
				},
			}
		}
	}

	for _, v := range os {
		env := []string{"GOARCH=" + v.Goarch, "GOOS=" + v.Goos}
		if isCGO {
			env = append(env, "CGO_ENABLED=1")
		}
		if isCGO {
			target := ""
			switch v.Goos {
			case "windows":
				switch v.Goarch {
				case "386":
					target = "x86_64-windows"
				case "arm64":
					target = "aarch64-windows"
				default:
					target = "x86_64-windows"
				}
			case "darwin":
				sysroot := ""
				if appendSysroot {
					if _, rootPath, _, err := zshell.Run("xcrun --show-sdk-path"); err == nil &&
						rootPath != "" {
						sysroot = " --sysroot=" + rootPath + " -F" + rootPath + "/System/Library/Frameworks -I/usr/include -L/usr/lib"
					}
				}
				switch v.Goarch {
				case "386":
					target = "x86-macos" + sysroot
				case "amd64":
					target = "x86_64-macos" + sysroot
				default:
					target = "aarch64-macos" + sysroot
				}
			case "linux":
				switch v.Goarch {
				case "386":
					target = "x86-linux-musl"
				case "arm64":
					target = "aarch64-linux-musl"
				default:
					target = "x86_64-linux-musl"
				}
			}

			if target != "" {
				v := `-target ` + target + ` ` + zigccFlags
				env = append(env, `CGO_CFLAGS=`+v)
				env = append(env, `CGO_LDFLAGS=`+v)
				env = append(env, `CGO_CXXFLAGS=`+v)
				env = append(env, `CXX=zig c++`)
				env = append(env, `CC=zig cc`)
			}
		}
		commad := baseCommand(outDir, packageName+"_"+v.Goos+"_"+v.Goarch, vendor, cShared, v.Goos)
		commads = append(commads, commad)
		envs = append(envs, env)
		goos = append(goos, v.Goos)
	}

	if len(commads) == 0 {
		commads = [][]string{baseCommand(outDir, packageName, vendor, cShared, zutil.GetOs())}
		env := []string{}
		if isCGO {
			env = append(env, "CGO_ENABLED=1")
		}
		envs = append(envs, env)
		goos = append(goos, zutil.GetOs())
	}

	if obfuscate > 0 {
		err := CheckGarble()
		if err != nil {
			util.Log.Fatal(err)
		}
		commad := []string{"garble", "-tiny"}
		if obfuscate > 1 {
			commad = append(commad, "-literals")
		}
		for i := range commads {
			commads[i] = append(commad, commads[i][1:]...)
		}
	}
	return
}

func baseCommand(outDir, name, vendor string, cShared bool, goos string) []string {
	commad := []string{"go", "build"}
	if vendor != "" {
		commad = append(commad, vendor)
	}
	commad = append(
		commad,
		"-o="+filename(outDir, name, cShared, goos),
	)
	if cShared {
		commad = append(commad, "-buildmode=c-shared")
	}
	return commad
}

func filename(outDir, name string, cShared bool, goos string) string {
	if cShared {
		switch goos {
		case "windows":
			return outDir + name + ".dll"
		case "linux":
			return outDir + name + ".so"
		case "darwin":
			return outDir + name + ".dylib"
		default:
			return outDir + name
		}
	}

	return outDir + zutil.IfVal(goos == "windows", name+".exe", name).(string)
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
	case "d", "darwin", "mac", "m":
		if goarch == "64" {
			goarch = "arm64"
		}
		commad["darwin"] = ParserArch(goarch, []string{"arm64"})
	case "android", "a":
		commad["android"] = ParserArch(goarch, []string{"arm64"})
	default:
		if goos == "" {
			break
		}
		if goarch == "*" || goarch == "" {
			util.Log.Errorf(
				"There is no GOARCH preset for %s, please complete it, for example: linux/amd64,windows/386\n",
				goos,
			)
			return nil
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
