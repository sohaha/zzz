package fs

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/sohaha/zlsgo/zfile"
)

type FileSystem struct{}

func New() *FileSystem {
	return &FileSystem{}
}

func (fs *FileSystem) ValidateFileForAdd(filePath string) error {

	if filePath == "" {
		return &FileNotExistsError{Path: filePath}
	}

	info, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &FileNotExistsError{Path: filePath}
		}
		return &FilePermissionError{Path: filePath, Err: err}
	}

	if !info.Mode().IsRegular() && !info.IsDir() {
		return &FileOperationError{Operation: "validate", Path: filePath, Err: fmt.Errorf("不支持的文件类型")}
	}

	if info.Mode().IsRegular() && info.Mode().Perm()&0o400 == 0 {
		return &FilePermissionError{Path: filePath}
	}

	return nil
}

func (fs *FileSystem) ValidateSymlinkForRemove(filePath, repoPath string) error {

	realPath := zfile.RealPath(filePath)
	if realPath == "" {
		return &FileNotExistsError{Path: filePath}
	}

	if !zfile.FileExist(realPath) {
		return &FileNotExistsError{Path: realPath}
	}

	fileInfo, err := os.Lstat(realPath)
	if err != nil {
		return &FileOperationError{
			Operation: "lstat",
			Path:      realPath,
			Err:       err,
		}
	}

	if fileInfo.Mode()&os.ModeSymlink == 0 {
		return &InvalidSymlinkError{
			Path: realPath,
			Err:  fmt.Errorf("文件不是符号链接"),
		}
	}

	target, err := os.Readlink(realPath)
	if err != nil {
		return &InvalidSymlinkError{
			Path: realPath,
			Err:  err,
		}
	}

	realTarget := zfile.RealPath(target)
	if realTarget == "" {
		return &InvalidSymlinkError{
			Path:   realPath,
			Target: target,
			Err:    fmt.Errorf("无法解析符号链接目标路径"),
		}
	}

	realRepoPath := zfile.RealPath(repoPath)
	if realRepoPath == "" {
		return &FileOperationError{
			Operation: "validate_repo",
			Path:      repoPath,
			Err:       fmt.Errorf("无法解析仓库路径"),
		}
	}

	if !strings.HasPrefix(realTarget, realRepoPath) {
		return &InvalidSymlinkError{
			Path:   realPath,
			Target: realTarget,
			Err:    fmt.Errorf("符号链接目标不在 lnk 仓库中"),
		}
	}

	return nil
}

func (fs *FileSystem) Move(src, dst string, info os.FileInfo) error {

	dstDir := filepath.Dir(dst)
	if err := fs.EnsureDir(dstDir); err != nil {
		return fmt.Errorf("创建目标目录失败: %w", err)
	}

	linfo, err := os.Lstat(src)
	if err != nil {
		return &FileOperationError{Operation: "lstat", Path: src, Err: err}
	}
	if linfo.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(src)
		if err != nil {
			return &FileOperationError{Operation: "readlink", Path: src, Err: err}
		}

		if !filepath.IsAbs(target) {
			target = filepath.Clean(filepath.Join(filepath.Dir(src), target))
		}
		tinfo, err := os.Stat(target)
		if err != nil {
			return &FileOperationError{Operation: "stat_target", Path: target, Err: err}
		}
		if tinfo.IsDir() {
			if err := fs.copyDir(target, dst); err != nil {
				return &FileOperationError{Operation: "copy_dir", Path: target, Err: err}
			}
		} else {
			if err := fs.copyFile(target, dst, tinfo.Mode()); err != nil {
				return &FileOperationError{Operation: "copy_file", Path: target, Err: err}
			}
		}

		if err := os.Remove(src); err != nil {

			_ = os.RemoveAll(dst)
			return &FileOperationError{Operation: "remove_symlink", Path: src, Err: err}
		}
		return nil
	}

	if err := os.Rename(src, dst); err != nil {

		if linkErr, ok := err.(*os.LinkError); ok && linkErr.Err == syscall.EXDEV {
			if info.IsDir() {
				if err := fs.copyDir(src, dst); err != nil {
					return &FileOperationError{Operation: "copy_dir", Path: src, Err: err}
				}
				if err := os.RemoveAll(src); err != nil {
					_ = os.RemoveAll(dst)
					return &FileOperationError{Operation: "remove_src_dir", Path: src, Err: err}
				}
			} else {
				if err := fs.copyFile(src, dst, info.Mode()); err != nil {
					return &FileOperationError{Operation: "copy_file", Path: src, Err: err}
				}
				if err := os.Remove(src); err != nil {
					_ = os.RemoveAll(dst)
					return &FileOperationError{Operation: "remove_src_file", Path: src, Err: err}
				}
			}
			return nil
		}
		return &FileOperationError{Operation: "rename", Path: src, Err: err}
	}
	return nil
}

func (fs *FileSystem) copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := fs.EnsureDir(filepath.Dir(dst)); err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode.Perm())
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Chmod(mode.Perm())
}

func (fs *FileSystem) copyDir(src, dst string) error {

	sinfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, sinfo.Mode().Perm()); err != nil {
		return err
	}
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == src {
			return nil
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(dst, rel)

		if d.Type()&os.ModeSymlink != 0 {
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return err
			}

			if err := fs.EnsureDir(filepath.Dir(targetPath)); err != nil {
				return err
			}
			return os.Symlink(linkTarget, targetPath)
		}

		if d.IsDir() {
			info, err := os.Stat(path)
			if err != nil {
				return err
			}
			return os.MkdirAll(targetPath, info.Mode().Perm())
		}

		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		return fs.copyFile(path, targetPath, info.Mode())
	})
}

func (fs *FileSystem) CreateSymlink(target, linkPath string) error {

	absTarget := target
	if !filepath.IsAbs(absTarget) {
		absTarget = filepath.Join(filepath.Dir(linkPath), absTarget)
	}
	absTarget = filepath.Clean(absTarget)

	if absTarget == "" {
		return &FileNotExistsError{Path: target}
	}
	if info, err := os.Stat(absTarget); err != nil {
		return &FileNotExistsError{Path: absTarget}
	} else if !info.Mode().IsRegular() && !info.IsDir() {
		return &FileOperationError{Operation: "stat", Path: absTarget, Err: fmt.Errorf("不支持的目标类型")}
	}

	linkDir := filepath.Dir(linkPath)
	if err := fs.EnsureDir(linkDir); err != nil {
		return fmt.Errorf("创建链接目录失败: %w", err)
	}

	linkDirAbs, err := filepath.Abs(linkDir)
	if err != nil {
		linkDirAbs = linkDir
	}
	relTarget, relErr := filepath.Rel(linkDirAbs, absTarget)
	targetForLink := relTarget
	if relErr != nil {

		targetForLink = absTarget
	}

	if err := os.Symlink(targetForLink, linkPath); err != nil {
		return &FileOperationError{
			Operation: "symlink",
			Path:      linkPath,
			Err:       err,
		}
	}

	return nil
}

func (fs *FileSystem) EnsureDir(dirPath string) error {
	if dirPath == "" {
		return fmt.Errorf("目录路径不能为空")
	}

	realPath := zfile.RealPathMkdir(dirPath)
	if realPath == "" {
		return &FileOperationError{
			Operation: "mkdir",
			Path:      dirPath,
			Err:       fmt.Errorf("无法创建目录"),
		}
	}

	return nil
}

func (fs *FileSystem) FileExists(filePath string) bool {
	if filePath == "" {
		return false
	}
	info, err := os.Stat(filePath)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular() || info.IsDir()
}

func (fs *FileSystem) IsDir(path string) bool {
	return zfile.DirExist(path)
}

func (fs *FileSystem) GetFileInfo(filePath string) (os.FileInfo, error) {
	realPath := zfile.RealPath(filePath)
	if realPath == "" {
		return nil, &FileNotExistsError{Path: filePath}
	}

	info, err := os.Stat(realPath)
	if err != nil {
		return nil, &FileOperationError{
			Operation: "stat",
			Path:      realPath,
			Err:       err,
		}
	}

	return info, nil
}

func (fs *FileSystem) IsSymlink(filePath string) bool {
	info, err := os.Lstat(filePath)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSymlink != 0
}

func (fs *FileSystem) ReadSymlink(linkPath string) (string, error) {
	target, err := os.Readlink(linkPath)
	if err != nil {
		return "", &InvalidSymlinkError{
			Path: linkPath,
			Err:  err,
		}
	}
	return target, nil
}

func (fs *FileSystem) RemoveFile(filePath string) error {
	realPath := zfile.RealPath(filePath)
	if realPath == "" {
		return &FileNotExistsError{Path: filePath}
	}

	if err := os.Remove(realPath); err != nil {
		return &FileOperationError{
			Operation: "remove",
			Path:      realPath,
			Err:       err,
		}
	}

	return nil
}
