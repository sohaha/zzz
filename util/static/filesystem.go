package static

import (
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"github.com/sohaha/zzz/util"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

var cwd string

func init() {
	var err error
	cwd, err = os.Getwd()
	if err != nil {
		util.Log.Fatal(err)
	}
}

// CompressFile reads the given file and converts it to a
// gzip compressed hex string
func CompressFile(filename string) (string, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return "", err
	}

	var byteBuffer bytes.Buffer
	writer := gzip.NewWriter(&byteBuffer)
	writer.Write(data)
	writer.Close()

	return hex.EncodeToString(byteBuffer.Bytes()), nil
}

// FindGoFiles finds all go files recursively from the given directory
func FindGoFiles(directory string) ([]string, error) {
	result := []string{}
	err := filepath.Walk(directory,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			goFilePath := filepath.Ext(path)
			if goFilePath == ".go" {
				isMewnFile := strings.HasSuffix(path, "-mewn.go")
				if !isMewnFile {
					result = append(result, path)
				}
			}
			return nil
		})
	return result, err
}

// DecompressHexString decompresses the gzip/hex encoded data
func DecompressHexString(hexdata string) ([]byte, error) {

	data, err := hex.DecodeString(hexdata)
	if err != nil {
		panic(err)
	}
	datareader := bytes.NewReader(data)

	gzipReader, err := gzip.NewReader(datareader)
	if err != nil {
		return nil, err
	}
	defer gzipReader.Close()

	return ioutil.ReadAll(gzipReader)
}

// HasMewnReference determines if the current file has a reference
// to the mewn library
func HasMewnReference(filename string) (bool, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return false, err
	}
	for _, imprt := range node.Imports {
		if imprt.Path.Value == `"github.com/sohaha/zzz/lib/static"` {
			return true, nil
		}
	}
	return false, nil
}

// GetMewnFiles returns a list of files referencing mewn assets
func GetMewnFiles(args []string, ignoreErrors bool) []string {

	var goFiles []string
	var err error

	if len(args) > 0 {
		for _, inputFile := range args {
			inputFile, err = filepath.Abs(inputFile)
			if err != nil && !ignoreErrors {
				util.Log.Fatal(err)
			}
			inputFile = filepath.ToSlash(inputFile)
			goFiles = append(goFiles, inputFile)
		}

	} else {
		// Find all go files
		goFiles, err = FindGoFiles(cwd)
		if err != nil && !ignoreErrors {
			util.Log.Fatal(err)
		}
	}

	var mewnFiles []string

	for _, goFile := range goFiles {
		isReferenced, err := HasMewnReference(goFile)
		if err != nil && !ignoreErrors {
			util.Log.Fatal(err)
		}
		if isReferenced {
			mewnFiles = append(mewnFiles, goFile)
		}
	}

	return mewnFiles
}
