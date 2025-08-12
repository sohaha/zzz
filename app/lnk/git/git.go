package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type Git struct {
	repoPath string
}

type StatusInfo struct {
	Ahead  int
	Behind int
	Remote string
	Dirty  bool
}

func New(repoPath string) *Git {
	return &Git{
		repoPath: repoPath,
	}
}

func (g *Git) Init() error {
	cmd := exec.Command("git", "init")
	cmd.Dir = g.repoPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return &GitCommandError{
			Command: "git init",
			Output:  string(output),
			Err:     err,
		}
	}

	return nil
}

func (g *Git) Clone(url string) error {

	parentDir := filepath.Dir(g.repoPath)
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		return &GitCommandError{
			Command: "mkdir",
			Err:     err,
		}
	}

	cmd := exec.Command("git", "clone", url, g.repoPath)
	output, err := cmd.CombinedOutput()
	if err != nil {

		if strings.Contains(string(output), "Could not resolve host") ||
			strings.Contains(string(output), "Connection refused") {
			return &NetworkError{
				Operation: "clone",
				URL:       url,
				Err:       err,
			}
		}

		if strings.Contains(string(output), "Authentication failed") ||
			strings.Contains(string(output), "Permission denied") {
			return &AuthenticationError{
				URL: url,
				Err: err,
			}
		}

		if strings.Contains(string(output), "not found") ||
			strings.Contains(string(output), "does not exist") {
			return &InvalidRemoteURLError{
				URL: url,
			}
		}

		return &GitCommandError{
			Command: fmt.Sprintf("git clone %s %s", url, g.repoPath),
			Output:  string(output),
			Err:     err,
		}
	}

	return nil
}

func (g *Git) Add(filename string) error {
	cmd := exec.Command("git", "add", filename)
	cmd.Dir = g.repoPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return &GitCommandError{
			Command: fmt.Sprintf("git add %s", filename),
			Output:  string(output),
			Err:     err,
		}
	}

	return nil
}

func (g *Git) AddMultiple(filenames []string) error {
	if len(filenames) == 0 {
		return nil
	}

	args := append([]string{"add"}, filenames...)
	cmd := exec.Command("git", args...)
	cmd.Dir = g.repoPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return &GitCommandError{
			Command: fmt.Sprintf("git add %s", strings.Join(filenames, " ")),
			Output:  string(output),
			Err:     err,
		}
	}

	return nil
}

func (g *Git) AddAll() error {
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = g.repoPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return &GitCommandError{
			Command: "git add .",
			Output:  string(output),
			Err:     err,
		}
	}

	return nil
}

func (g *Git) Remove(filename string) error {
	cmd := exec.Command("git", "rm", filename)
	cmd.Dir = g.repoPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return &GitCommandError{
			Command: fmt.Sprintf("git rm %s", filename),
			Output:  string(output),
			Err:     err,
		}
	}

	return nil
}

func (g *Git) Commit(message string) error {
	if message == "" {
		message = "lnk: automated commit"
	}

	cmd := exec.Command("git", "commit", "-m", message)
	cmd.Dir = g.repoPath

	output, err := cmd.CombinedOutput()
	if err != nil {

		if strings.Contains(string(output), "nothing to commit") {
			return nil
		}

		return &GitCommandError{
			Command: fmt.Sprintf("git commit -m \"%s\"", message),
			Output:  string(output),
			Err:     err,
		}
	}

	return nil
}

func (g *Git) Push() error {
	cmd := exec.Command("git", "push")
	cmd.Dir = g.repoPath

	output, err := cmd.CombinedOutput()
	if err != nil {

		if strings.Contains(string(output), "Could not resolve host") ||
			strings.Contains(string(output), "Connection refused") {
			return &NetworkError{
				Operation: "push",
				Err:       err,
			}
		}

		if strings.Contains(string(output), "Authentication failed") ||
			strings.Contains(string(output), "Permission denied") {
			return &AuthenticationError{
				Err: err,
			}
		}

		return &GitCommandError{
			Command: "git push",
			Output:  string(output),
			Err:     err,
		}
	}

	return nil
}

func (g *Git) Pull() error {
	cmd := exec.Command("git", "pull")
	cmd.Dir = g.repoPath

	output, err := cmd.CombinedOutput()
	if err != nil {

		if strings.Contains(string(output), "CONFLICT") {

			files := g.parseConflictFiles(string(output))
			return &MergeConflictError{
				Files: files,
			}
		}

		if strings.Contains(string(output), "Could not resolve host") ||
			strings.Contains(string(output), "Connection refused") {
			return &NetworkError{
				Operation: "pull",
				Err:       err,
			}
		}

		return &GitCommandError{
			Command: "git pull",
			Output:  string(output),
			Err:     err,
		}
	}

	return nil
}

func (g *Git) GetStatus() (*StatusInfo, error) {
	status := &StatusInfo{}

	dirty, err := g.isDirty()
	if err != nil {
		return nil, err
	}
	status.Dirty = dirty

	remote, err := g.getRemoteName()
	if err != nil {

		if _, ok := err.(*RemoteNotFoundError); ok {
			return status, nil
		}
		return nil, err
	}
	status.Remote = remote

	ahead, behind, err := g.getAheadBehind(remote)
	if err != nil {
		return nil, err
	}
	status.Ahead = ahead
	status.Behind = behind

	return status, nil
}

func (g *Git) IsRepo() bool {
	gitDir := filepath.Join(g.repoPath, ".git")
	_, err := os.Stat(gitDir)
	return err == nil
}

func (g *Git) HasRemote() bool {
	cmd := exec.Command("git", "remote")
	cmd.Dir = g.repoPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}

	return strings.TrimSpace(string(output)) != ""
}

func (g *Git) isDirty() (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = g.repoPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, &GitCommandError{
			Command: "git status --porcelain",
			Output:  string(output),
			Err:     err,
		}
	}

	return strings.TrimSpace(string(output)) != "", nil
}

func (g *Git) getRemoteName() (string, error) {
	cmd := exec.Command("git", "remote")
	cmd.Dir = g.repoPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", &GitCommandError{
			Command: "git remote",
			Output:  string(output),
			Err:     err,
		}
	}

	remotes := strings.Fields(strings.TrimSpace(string(output)))
	if len(remotes) == 0 {
		return "", &RemoteNotFoundError{
			RemoteName: "origin",
		}
	}

	for _, remote := range remotes {
		if remote == "origin" {
			return remote, nil
		}
	}

	return remotes[0], nil
}

func (g *Git) getAheadBehind(remote string) (int, int, error) {

	currentBranch, err := g.getCurrentBranch()
	if err != nil {
		return 0, 0, err
	}

	cmd := exec.Command("git", "rev-list", "--count", "--left-right",
		fmt.Sprintf("%s/%s...HEAD", remote, currentBranch))
	cmd.Dir = g.repoPath

	output, err := cmd.CombinedOutput()
	if err != nil {

		if strings.Contains(string(output), "unknown revision") {
			return 0, 0, nil
		}

		return 0, 0, &GitCommandError{
			Command: fmt.Sprintf("git rev-list --count --left-right %s/%s...HEAD", remote, currentBranch),
			Output:  string(output),
			Err:     err,
		}
	}

	parts := strings.Fields(strings.TrimSpace(string(output)))
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("unexpected git rev-list output: %s", string(output))
	}

	behind, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse behind count: %v", err)
	}

	ahead, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse ahead count: %v", err)
	}

	return ahead, behind, nil
}

func (g *Git) getCurrentBranch() (string, error) {
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = g.repoPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", &GitCommandError{
			Command: "git branch --show-current",
			Output:  string(output),
			Err:     err,
		}
	}

	branch := strings.TrimSpace(string(output))
	if branch == "" {
		return "main", nil
	}

	return branch, nil
}

func (g *Git) parseConflictFiles(output string) []string {
	var files []string

	re := regexp.MustCompile(`CONFLICT.*in (.+)`)
	matches := re.FindAllStringSubmatch(output, -1)

	for _, match := range matches {
		if len(match) > 1 {
			files = append(files, match[1])
		}
	}

	return files
}

func (g *Git) SetRemote(name, url string) error {
	cmd := exec.Command("git", "remote", "add", name, url)
	cmd.Dir = g.repoPath

	output, err := cmd.CombinedOutput()
	if err != nil {

		if strings.Contains(string(output), "already exists") {
			return g.updateRemote(name, url)
		}

		return &GitCommandError{
			Command: fmt.Sprintf("git remote add %s %s", name, url),
			Output:  string(output),
			Err:     err,
		}
	}

	return nil
}

func (g *Git) updateRemote(name, url string) error {
	cmd := exec.Command("git", "remote", "set-url", name, url)
	cmd.Dir = g.repoPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return &GitCommandError{
			Command: fmt.Sprintf("git remote set-url %s %s", name, url),
			Output:  string(output),
			Err:     err,
		}
	}

	return nil
}
