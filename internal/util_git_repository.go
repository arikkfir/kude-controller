package internal

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type gitRepository struct {
	url *url.URL
	dir string
}

func newGitRepository(name string) (*gitRepository, error) {
	dir, err := ioutil.TempDir("", "kude-controller")
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

	r := &gitRepository{
		url: gitRepositoryUrl,
		dir: dir,
	}
	if err := r.git("init", "--initial-branch=main"); err != nil {
		return nil, fmt.Errorf("failed to initialize Git repository: %w", err)
	} else if err := r.git("config", "user.name", "kude"); err != nil {
		return nil, fmt.Errorf("failed to set Git user name: %w", err)
	} else if err := r.git("config", "user.email", "arik+kude@kfirs.com"); err != nil {
		return nil, fmt.Errorf("failed to set Git user email: %w", err)
	} else {
		return r, nil
	}
}

func (r *gitRepository) git(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to run git command '%s' in dir '%s': %w\n%s", strings.Join(cmd.Args, " "), r.dir, err, string(out))
	} else {
		return nil
	}
}

func (r *gitRepository) commitFile(file, content string) error {
	path := filepath.Join(r.dir, file)
	if err := ioutil.WriteFile(path, []byte(content), 0600); err != nil {
		return fmt.Errorf("failed to write file '%s': %w", path, err)
	} else if err := r.git("add", file); err != nil {
		return fmt.Errorf("failed to add file '%s': %w", file, err)
	} else if err := r.git("commit", "-m", "Adding "+file); err != nil {
		return fmt.Errorf("failed to commit file '%s': %w", file, err)
	} else {
		return nil
	}
}
