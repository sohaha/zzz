package fs

import "fmt"

type FileNotExistsError struct {
	Path string
	Err  error
}

func (e *FileNotExistsError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("文件不存在: %s (%v)", e.Path, e.Err)
	}
	return fmt.Sprintf("文件不存在: %s", e.Path)
}

func (e *FileNotExistsError) Unwrap() error {
	return e.Err
}

type FilePermissionError struct {
	Path string
	Err  error
}

func (e *FilePermissionError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("文件权限不足: %s (%v)", e.Path, e.Err)
	}
	return fmt.Sprintf("文件权限不足: %s", e.Path)
}

func (e *FilePermissionError) Unwrap() error {
	return e.Err
}

type InvalidSymlinkError struct {
	Path   string
	Target string
	Err    error
}

func (e *InvalidSymlinkError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("无效的符号链接: %s -> %s (%v)", e.Path, e.Target, e.Err)
	}
	return fmt.Sprintf("无效的符号链接: %s -> %s", e.Path, e.Target)
}

func (e *InvalidSymlinkError) Unwrap() error {
	return e.Err
}

type FileOperationError struct {
	Operation string
	Path      string
	Err       error
}

func (e *FileOperationError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("文件操作失败 (%s): %s (%v)", e.Operation, e.Path, e.Err)
	}
	return fmt.Sprintf("文件操作失败 (%s): %s", e.Operation, e.Path)
}

func (e *FileOperationError) Unwrap() error {
	return e.Err
}

type DirectoryNotEmptyError struct {
	Path string
}

func (e *DirectoryNotEmptyError) Error() string {
	return fmt.Sprintf("目录非空: %s", e.Path)
}

type PathTraversalError struct {
	Path string
}

func (e *PathTraversalError) Error() string {
	return fmt.Sprintf("检测到路径遍历攻击: %s", e.Path)
}
