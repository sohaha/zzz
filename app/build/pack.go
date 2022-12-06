package build

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/sohaha/zlsgo/zfile"
	"github.com/sohaha/zlsgo/zstring"
	zstatic "github.com/sohaha/zstatic/build"
)

func ReadMod(pwd string) string {
	if zfile.FileExist(pwd + "go.mod") {
		content, _ := ioutil.ReadFile(pwd + "go.mod")
		return zstring.Bytes2String(content)
	}
	return ""
}

func Basename(pwd string) string {
	name := filepath.Base(pwd)
	content := ReadMod(pwd)
	if content != "" {
		str, err := zstring.RegexExtract(`module (.*)`, content)
		if err == nil && len(str) > 0 {
			p := strings.Split(str[1], "/")
			name = p[len(p)-1]
		}
	}
	return name
}

func clearRoot(rootDir string, path string) string {
	return strings.Replace(path, rootDir, "", 1)
}

func getAllFilesInDirectory(dir string) (result []string, err error) {
	rootDir := zfile.RealPath(".", true)
	if !zfile.DirExist(dir) {
		return
	}
	if strings.Contains(dir, "*") {
		var files []string
		files, err = filepath.Glob(dir)
		for _, path := range files {
			path = zfile.RealPath(path)
			info, err := os.Stat(path)
			if err != nil {
				return []string{}, err
			}
			if info.Mode().IsRegular() {
				result = append(result, clearRoot(rootDir, path))
			}
		}
		return
	}
	err = filepath.Walk(dir, func(path string, info os.FileInfo, e error) error {
		if e != nil {
			return e
		}
		path = zfile.RealPath(path)
		if info.Mode().IsRegular() {
			result = append(result, clearRoot(rootDir, path))
		}
		return nil
	})
	return
}

// GeneratePackFileString creates the contents of a pack file
func GeneratePackFileString(assetBundle *ReferencedAssets, ignoreErrors bool) (string, error) {
	var filesProcessed = make(map[string]bool)
	result := fmt.Sprintf("package %s\n\n", assetBundle.PackageName)
	content := zstring.Buffer()
	if len(assetBundle.Groups) > 0 || len(assetBundle.Assets) > 0 {
		for _, group := range assetBundle.Groups {
			// Read all assets from the directory
			files, err := getAllFilesInDirectory(group.FullPath)
			rootDir := zfile.RealPath(".", true)
			groupPrefix := clearRoot(rootDir, zfile.RealPath(group.FullPath)) + "/"
			if err != nil {
				return "", err
			}

			if len(files) == 0 {
				continue
			}

			for _, file := range files {
				// Read in File
				packedData, err := zstatic.CompressFile(file)
				if err != nil && !ignoreErrors {
					return "", err
				}
				localPath := clearRoot(groupPrefix, file)
				content.WriteString(fmt.Sprintf("  zstatic.AddByteAsset(\"%s\", \"%s\",%#v)\n", group.LocalPath, localPath, packedData))
				// result += fmt.Sprintf("  zstatic.AddAsset(\"%s\", \"%s\", \"%s\")\n", groupPrefix, localPath, packedData)
				filesProcessed[file] = true
				// fmt.Printf("Packed: %s\n", file)
			}
		}
		for _, asset := range assetBundle.Assets {
			groupPath := "."
			if asset.Group != nil {
				groupPath = asset.Group.LocalPath
			}
			fullPath, err := filepath.Abs(filepath.Join(groupPath, asset.AssetPath))
			if err != nil {
				return "", err
			}
			// if _, exists := filesProcessed[fullPath]; exists == true {
			// 	continue
			// }
			packedData, err := zstatic.CompressFile(fullPath)
			if err != nil && !ignoreErrors {
				return "", err
			}
			content.WriteString(fmt.Sprintf("  zstatic.AddByteAsset(\".\", \"%s\", %#v)\n", asset.Name, packedData))
			filesProcessed[fullPath] = true
		}
		if content.Len() > 0 {
			result += "import \"github.com/sohaha/zstatic\"\n\n"
			result += "func init() {\n"
			result += content.String()
			result += "}\n"
		}

	}

	return result, nil
}
