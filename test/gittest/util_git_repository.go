package gittest

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type GitRepository struct {
	URL *url.URL
	Dir string
}

func NewGitRepository(name string) (*GitRepository, error) {
	dir, err := os.MkdirTemp("", "kude-controller")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	dir = filepath.Join(dir, name)
	if err := os.Mkdir(dir, 0700); err != nil {
		return nil, fmt.Errorf("failed to set permissions on temp dir: %w", err)
	}

	gitRepositoryUrl, err := url.Parse("file://" + dir)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Git repository URL '%s': %w", gitRepositoryUrl, err)
	}

	r := &GitRepository{
		URL: gitRepositoryUrl,
		Dir: dir,
	}
	if err := r.RunGit("init", "--initial-branch=main"); err != nil {
		return nil, fmt.Errorf("failed to initialize Git repository: %w", err)
	} else if err := r.RunGit("config", "user.name", "kude"); err != nil {
		return nil, fmt.Errorf("failed to set Git user name: %w", err)
	} else if err := r.RunGit("config", "user.email", "arik+kude@kfirs.com"); err != nil {
		return nil, fmt.Errorf("failed to set Git user email: %w", err)
	} else {
		return r, nil
	}
}

func (r *GitRepository) RunGit(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.Dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to run git command '%s' in dir '%s': %w\n%s", strings.Join(cmd.Args, " "), r.Dir, err, string(out))
	} else {
		return nil
	}
}

func (r *GitRepository) CommitFile(file, content string) error {
	path := filepath.Join(r.Dir, file)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		return fmt.Errorf("failed to write file '%s': %w", path, err)
	} else if err := r.RunGit("add", file); err != nil {
		return fmt.Errorf("failed to add file '%s': %w", file, err)
	} else if err := r.RunGit("commit", "-m", "Adding "+file); err != nil {
		return fmt.Errorf("failed to commit file '%s': %w", file, err)
	} else {
		return nil
	}
}
