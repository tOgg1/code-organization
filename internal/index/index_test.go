package index

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tormodhaugland/co/internal/config"
	"github.com/tormodhaugland/co/internal/model"
)

func TestBuilderSyncProjectRepos(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{CodeRoot: tmp}

	workspacePath := filepath.Join(tmp, "acme--app")
	repoPath := filepath.Join(workspacePath, "repos", "app")
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}

	proj := model.NewProject("acme", "app")
	proj.Repos = []model.RepoSpec{}
	if err := proj.Save(workspacePath); err != nil {
		t.Fatalf("save project.json: %v", err)
	}

	remote := "git@github.com:acme/app.git"
	initGitRepo(t, repoPath, remote)

	builder := NewBuilder(cfg)
	if _, err := builder.Build(); err != nil {
		t.Fatalf("build index: %v", err)
	}

	updated, err := model.LoadProject(filepath.Join(workspacePath, "project.json"))
	if err != nil {
		t.Fatalf("load project.json: %v", err)
	}

	if len(updated.Repos) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(updated.Repos))
	}
	if updated.Repos[0].Path != "repos/app" {
		t.Fatalf("unexpected repo path: %s", updated.Repos[0].Path)
	}
	if updated.Repos[0].Remote != remote {
		t.Fatalf("unexpected repo remote: %s", updated.Repos[0].Remote)
	}
}

func TestBuilderNoProjectSync(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{CodeRoot: tmp}

	workspacePath := filepath.Join(tmp, "acme--app")
	repoPath := filepath.Join(workspacePath, "repos", "app")
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}

	proj := model.NewProject("acme", "app")
	proj.Repos = []model.RepoSpec{}
	if err := proj.Save(workspacePath); err != nil {
		t.Fatalf("save project.json: %v", err)
	}

	initGitRepo(t, repoPath, "git@github.com:acme/app.git")

	builder := NewBuilder(cfg)
	builder.SetSyncProjectRepos(false)
	if _, err := builder.Build(); err != nil {
		t.Fatalf("build index: %v", err)
	}

	updated, err := model.LoadProject(filepath.Join(workspacePath, "project.json"))
	if err != nil {
		t.Fatalf("load project.json: %v", err)
	}
	if len(updated.Repos) != 0 {
		t.Fatalf("expected no repos, got %d", len(updated.Repos))
	}
}

func TestBuilderProgress(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{CodeRoot: tmp}

	for _, slug := range []string{"acme--one", "acme--two"} {
		workspacePath := filepath.Join(tmp, slug)
		if err := os.MkdirAll(filepath.Join(workspacePath, "repos"), 0755); err != nil {
			t.Fatalf("mkdir workspace: %v", err)
		}
		parts := strings.Split(slug, "--")
		proj := model.NewProject(parts[0], parts[1])
		if err := proj.Save(workspacePath); err != nil {
			t.Fatalf("save project.json: %v", err)
		}
	}

	var calls []int
	builder := NewBuilder(cfg)
	builder.SetProgress(func(done, total int) {
		calls = append(calls, done)
	})

	if _, err := builder.Build(); err != nil {
		t.Fatalf("build index: %v", err)
	}

	if len(calls) != 2 {
		t.Fatalf("expected 2 progress calls, got %d", len(calls))
	}
	if calls[len(calls)-1] != 2 {
		t.Fatalf("expected final progress to be 2, got %d", calls[len(calls)-1])
	}
}

func initGitRepo(t *testing.T, repoPath, remote string) {
	t.Helper()

	runGit(t, repoPath, nil, "init")
	if err := os.WriteFile(filepath.Join(repoPath, "README.md"), []byte("test\n"), 0644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	runGit(t, repoPath, nil, "add", ".")

	env := map[string]string{
		"GIT_AUTHOR_NAME":     "Test User",
		"GIT_AUTHOR_EMAIL":    "test@example.com",
		"GIT_COMMITTER_NAME":  "Test User",
		"GIT_COMMITTER_EMAIL": "test@example.com",
	}
	runGit(t, repoPath, env, "commit", "-m", "init")

	if remote != "" {
		runGit(t, repoPath, nil, "remote", "add", "origin", remote)
	}
}

func runGit(t *testing.T, repoPath string, extraEnv map[string]string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repoPath}, args...)...)
	if len(extraEnv) > 0 {
		cmd.Env = append(os.Environ(), envMapToList(extraEnv)...)
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
}

func envMapToList(env map[string]string) []string {
	out := make([]string, 0, len(env))
	for key, value := range env {
		out = append(out, key+"="+value)
	}
	return out
}
