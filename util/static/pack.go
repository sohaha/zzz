package static

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func getAllFilesInDirectory(dir string) ([]string, error) {
	var result []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, e error) error {
		if e != nil {
			return e
		}

		// check if it is a regular file (not dir)
		if info.Mode().IsRegular() {
			result = append(result, path)
		}
		return nil
	})

	return result, err
}

// GeneratePackFileString creates the contents of a pack file
func GeneratePackFileString(assetBundle *ReferencedAssets, ignoreErrors bool) (string, error) {

	var filesProcessed = make(map[string]bool)
	// fmt.Printf("Bundling this asset: %+v\n", assetBundle)
	result := fmt.Sprintf("package %s\n\n", assetBundle.PackageName)
	if len(assetBundle.Groups) > 0 || len(assetBundle.Assets) > 0 {
		result += "import \"github.com/sohaha/zzz/lib/static\"\n\n"
		result += "func init() {\n"
		for _, group := range assetBundle.Groups {
			// Read all assets from the directory
			files, err := getAllFilesInDirectory(group.FullPath)
			if err != nil {
				return "", err
			}
			for _, file := range files {
				// Read in File
				packedData, err := CompressFile(file)
				if err != nil && !ignoreErrors {
					return "", err
				}
				localPath := strings.TrimPrefix(file, group.FullPath+"/")
				result += fmt.Sprintf("  static.AddAsset(\"%s\", \"%s\", \"%s\")\n", group.LocalPath, localPath, packedData)
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
			if _, exists := filesProcessed[fullPath]; exists == true {
				continue
			}
			packedData, err := CompressFile(fullPath)
			if err != nil && !ignoreErrors {
				return "", err
			}
			result += fmt.Sprintf("  static.AddAsset(\".\", \"%s\", \"%s\")\n", asset.Name, packedData)
			filesProcessed[fullPath] = true
			// fmt.Printf("Packed: %s\n", fullPath)
		}
		result += "}\n"
	}

	return result, nil
}
