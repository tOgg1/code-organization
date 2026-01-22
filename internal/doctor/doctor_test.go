package doctor

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/tormodhaugland/co/internal/model"
)

func TestParseSlug(t *testing.T) {
	owner, name, ok := ParseSlug("acme--app")
	if !ok {
		t.Fatal("expected ok for valid slug")
	}
	if owner != "acme" || name != "app" {
		t.Fatalf("unexpected parse result: owner=%q name=%q", owner, name)
	}

	owner, name, ok = ParseSlug("acme--app--legacy")
	if !ok {
		t.Fatal("expected ok for qualifier slug")
	}
	if owner != "acme" || name != "app--legacy" {
		t.Fatalf("unexpected parse result: owner=%q name=%q", owner, name)
	}

	_, _, ok = ParseSlug("invalid")
	if ok {
		t.Fatal("expected invalid slug to fail")
	}
}

func TestFindMissingProjects(t *testing.T) {
	tmpDir := t.TempDir()

	missingSlug := "acme--missing"
	missingPath := filepath.Join(tmpDir, missingSlug)
	if err := os.MkdirAll(filepath.Join(missingPath, "repos"), 0o755); err != nil {
		t.Fatalf("mkdir missing workspace: %v", err)
	}

	okSlug := "oss--present"
	okPath := filepath.Join(tmpDir, okSlug)
	if err := os.MkdirAll(filepath.Join(okPath, "repos"), 0o755); err != nil {
		t.Fatalf("mkdir ok workspace: %v", err)
	}
	project := model.NewProject("oss", "present")
	if err := project.Save(okPath); err != nil {
		t.Fatalf("save project.json: %v", err)
	}

	missing, err := FindMissingProjects(tmpDir)
	if err != nil {
		t.Fatalf("FindMissingProjects error: %v", err)
	}

	if len(missing) != 1 {
		t.Fatalf("expected 1 missing project, got %d", len(missing))
	}

	if missing[0].Slug != missingSlug || missing[0].Path != missingPath {
		t.Fatalf("unexpected missing project: %+v", missing[0])
	}
}

func TestCreateProjectJSON(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmpDir := t.TempDir()
	slug := "acme--app"
	workspacePath := filepath.Join(tmpDir, slug)
	reposPath := filepath.Join(workspacePath, "repos")
	if err := os.MkdirAll(reposPath, 0o755); err != nil {
		t.Fatalf("mkdir repos: %v", err)
	}

	apiRepo := filepath.Join(reposPath, "api")
	if err := os.MkdirAll(apiRepo, 0o755); err != nil {
		t.Fatalf("mkdir api repo: %v", err)
	}
	initGitRepo(t, apiRepo, "https://github.com/acme/api.git")

	webRepo := filepath.Join(reposPath, "web")
	if err := os.MkdirAll(webRepo, 0o755); err != nil {
		t.Fatalf("mkdir web repo: %v", err)
	}

	project, err := CreateProjectJSON(slug, workspacePath)
	if err != nil {
		t.Fatalf("CreateProjectJSON error: %v", err)
	}

	if project.Slug != slug || project.Owner != "acme" || project.Name != "app" {
		t.Fatalf("unexpected project: %+v", project)
	}

	loaded, err := model.LoadProject(filepath.Join(workspacePath, "project.json"))
	if err != nil {
		t.Fatalf("load project.json: %v", err)
	}

	if len(loaded.Repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(loaded.Repos))
	}

	api := findRepo(loaded.Repos, "api")
	if api == nil {
		t.Fatal("expected api repo")
	}
	if api.Remote != "https://github.com/acme/api.git" {
		t.Fatalf("unexpected api remote: %q", api.Remote)
	}

	web := findRepo(loaded.Repos, "web")
	if web == nil {
		t.Fatal("expected web repo")
	}
	if web.Remote != "" {
		t.Fatalf("expected empty web remote, got %q", web.Remote)
	}
}

func findRepo(repos []model.RepoSpec, name string) *model.RepoSpec {
	for i := range repos {
		if repos[i].Name == name {
			return &repos[i]
		}
	}
	return nil
}

func initGitRepo(t *testing.T, dir, remote string) {
	t.Helper()

	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Skipf("git init failed: %v", err)
	}

	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Skipf("git config email failed: %v", err)
	}

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Skipf("git config name failed: %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("test"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Skipf("git add failed: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "init")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Skipf("git commit failed: %v", err)
	}

	if remote != "" {
		cmd = exec.Command("git", "remote", "add", "origin", remote)
		cmd.Dir = dir
		if err := cmd.Run(); err != nil {
			t.Skipf("git remote add failed: %v", err)
		}
	}
}
