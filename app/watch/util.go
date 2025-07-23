package watch

import (
	"errors"
	"io/ioutil"
	"net"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/sohaha/zlsgo/zfile"
	"github.com/sohaha/zlsgo/zstring"
	"github.com/sohaha/zzz/util"

	"github.com/sohaha/zlsgo/ztype"
)

func dirParse2Array(s string) []string {
	a := strings.Split(s, ",")
	r := make([]string, 0)

	for i := 0; i < len(a); i++ {
		if ss := strings.Trim(a[i], " "); ss != "" {
			r = append(r, ss)
		}
	}
	return r
}

func isIgnoreDirectory(folder string) bool {
	base := filepath.Base(folder)
	for _, v := range ignoreDirectory {
		if base == v {
			return true
		}
	}
	return false
}

func listFile(folder string, fun func(string)) {
	folder = zfile.RealPath(folder)
	if isIgnoreDirectory(folder) {
		util.Log.Debugf("Ignore directory: %s", folder)
		return
	}

	if isExcept(exceptDirs, folder) {
		util.Log.Debugf("Excluding directory: %s", folder)
		return
	}

	files, err := ioutil.ReadDir(folder)
	if err != nil {
		util.Log.Errorf("Failed to read directory %s: %v", folder, err)
		return
	}

	for _, file := range files {
		if file.IsDir() {
			d := zfile.RealPath(folder + "/" + file.Name())

			if isIgnoreDirectory(d) {
				util.Log.Debugf("Ignoring directory: %s", d)
				continue
			}
			if isExcept(exceptDirs, d) {
				util.Log.Debugf("Excluding directory: %s", d)
				continue
			}

			fun(d)
			listFile(d, fun)
		}
	}
}

func arrayUniqueAdd(a []string, add string) []string {
	if inStringArray(add, a) || isExcept(exceptDirs, add) {
		return a
	}
	return append(a, add)
}

func isExcept(e []string, path string) bool {
	for _, pattern := range e {
		normalizedPath := filepath.ToSlash(path)
		normalizedPattern := filepath.ToSlash(pattern)

		if normalizedPattern == ".,*" || normalizedPattern == "*" {
			return true
		}

		if strings.HasPrefix(normalizedPattern, "*/") {
			subPattern := strings.TrimPrefix(normalizedPattern, "*/")
			pathParts := strings.Split(normalizedPath, "/")
			for i, part := range pathParts {
				if matched, _ := filepath.Match(subPattern, strings.Join(pathParts[i:], "/")); matched {
					return true
				}
				if strings.Contains(subPattern, "/") {
					remainingPath := strings.Join(pathParts[i:], "/")
					if matched, _ := filepath.Match(subPattern, remainingPath); matched {
						return true
					}
				} else {
					if matched, _ := filepath.Match(subPattern, part); matched {
						return true
					}
				}
			}
		}

		cleanPattern := strings.Replace(strings.Replace(normalizedPattern, "*", "", -1), "//", "/", -1)
		if strings.HasPrefix(normalizedPath, cleanPattern) {
			return true
		}

		if zstring.Match(normalizedPath, normalizedPattern) {
			return true
		}
		if matched, err := filepath.Match(normalizedPattern, normalizedPath); err == nil && matched {
			return true
		}

		baseName := filepath.Base(normalizedPath)
		if matched, err := filepath.Match(normalizedPattern, baseName); err == nil && matched {
			return true
		}
		if strings.Contains(normalizedPattern, "**") {
			if matchRecursivePattern(normalizedPath, normalizedPattern) {
				return true
			}
		}
	}
	return false
}

func inStringArray(value string, arr []string) bool {
	for _, v := range arr {
		if value == v {
			return true
		}
	}
	return false
}

func cmdParse2Array(s string, cf *changedFile) []string {
	a := strings.Split(s, " ")
	r := make([]string, 0)
	for i := 0; i < len(a); i++ {
		if ss := strings.Trim(a[i], " "); ss != "" {
			r = append(r, strParseRealStr(ss, cf))
		}
	}
	return r
}

func strParseRealStr(s string, cf *changedFile) string {
	return strings.Replace(
		strings.Replace(
			strings.Replace(s, "{{file}}", cf.Name, -1),
			"{{ext}}", cf.Ext, -1,
		),
		"{{changed}}", strconv.FormatInt(cf.Changed, 10), -1,
	)
}

func getDelay() time.Duration {
	delay := task.delay
	if delay <= 0 {
		delay = 100
	}
	return time.Millisecond * time.Duration(delay)
}

func getPort(port int, generate bool) (newPort int, err error) {
	host := "127.0.0.1:" + ztype.ToString(port)
	listener, err := net.Listen("tcp", host)
	if err != nil {
		if !generate && port != 0 {
			return 0, err
		}
		listener, err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return 0, err
		}
	}
	defer listener.Close()
	addr := listener.Addr().String()
	_, portString, err := net.SplitHostPort(addr)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(portString)
}

func openBrowser(url string) error {
	var err error
	if !strings.HasPrefix(url, "http") {
		url = "http://" + url
	}
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = errors.New("unsupported platform")
	}
	return err
}

func matchRecursivePattern(path, pattern string) bool {
	if pattern == "**" {
		return true
	}
	if strings.HasPrefix(pattern, "**/") && strings.HasSuffix(pattern, "/**") {
		middle := strings.TrimPrefix(strings.TrimSuffix(pattern, "/**"), "**/")
		if strings.Contains(path, "/"+middle+"/") ||
			strings.Contains(path, middle+"/") ||
			strings.HasPrefix(path, middle+"/") {
			return true
		}
	}

	if strings.HasPrefix(pattern, "**/") && strings.Contains(pattern, "*") && !strings.HasSuffix(pattern, "/**") {
		suffix := strings.TrimPrefix(pattern, "**/")
		pathParts := strings.Split(path, "/")
		for _, part := range pathParts {
			if matched, _ := filepath.Match(suffix, part); matched {
				return true
			}
		}
		if matched, _ := filepath.Match(suffix, filepath.Base(path)); matched {
			return true
		}
	}
	if strings.HasPrefix(pattern, "**/") && !strings.Contains(pattern, "*") {
		suffix := strings.TrimPrefix(pattern, "**/")
		if strings.HasSuffix(path, suffix) || strings.Contains(path, "/"+suffix) {
			return true
		}
	}

	if strings.HasSuffix(pattern, "/**") && !strings.HasPrefix(pattern, "**/") {
		prefix := strings.TrimSuffix(pattern, "/**")
		if strings.HasPrefix(path, prefix+"/") || strings.Contains(path, "/"+prefix+"/") {
			return true
		}
	}

	regexPattern := strings.ReplaceAll(pattern, "**", ".*")
	regexPattern = strings.ReplaceAll(regexPattern, "*", "[^/]*")
	regexPattern = "^" + regexPattern + "$"

	matched, err := regexp.MatchString(regexPattern, path)
	return err == nil && matched
}

func isIgnoreType(fileExt string) (yes bool) {
	if len(ignoreFormat) == 0 {
		return
	}
	for _, v := range ignoreFormat {
		if "."+strings.ToLower(fileExt) == v {
			return true
		}
	}
	return
}
