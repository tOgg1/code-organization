package sync

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tormodhaugland/co/internal/fs"
	"github.com/tormodhaugland/co/internal/model"
)

func TestBuildExcludes(t *testing.T) {
	opts := &Options{
		NoGit:           true,
		IncludeEnv:      false,
		ExcludePatterns: []string{"custom/"},
	}

	excludeList, err := opts.BuildExcludes()
	if err != nil {
		t.Fatalf("BuildExcludes returned error: %v", err)
	}

	// Should contain .git/ since NoGit is true
	hasGit := false
	hasCustom := false
	for _, p := range excludeList.Patterns {
		if p == ".git/" {
			hasGit = true
		}
		if p == "custom/" {
			hasCustom = true
		}
	}

	if !hasGit {
		t.Error("Expected .git/ in excludes when NoGit is true")
	}
	if !hasCustom {
		t.Error("Expected custom/ in excludes")
	}
}

func TestBuildExcludesIncludeEnv(t *testing.T) {
	opts := &Options{
		IncludeEnv: true,
	}

	excludeList, err := opts.BuildExcludes()
	if err != nil {
		t.Fatalf("BuildExcludes returned error: %v", err)
	}

	// Should NOT contain .env patterns when IncludeEnv is true
	for _, p := range excludeList.Patterns {
		if p == ".env" || p == ".env.*" {
			t.Errorf("Expected .env patterns to be excluded when IncludeEnv is true, found: %s", p)
		}
	}
}

func TestBuildExcludesFromFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "excludes.txt")
	content := "# comment line\ncustom1/\n \n# another\ncustom2/\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	opts := &Options{
		ExcludeFromFile: path,
	}

	excludeList, err := opts.BuildExcludes()
	if err != nil {
		t.Fatalf("BuildExcludes returned error: %v", err)
	}

	hasCustom1 := false
	hasCustom2 := false
	for _, p := range excludeList.Patterns {
		if p == "custom1/" {
			hasCustom1 = true
		}
		if p == "custom2/" {
			hasCustom2 = true
		}
	}

	if !hasCustom1 {
		t.Error("Expected custom1/ in excludes from file")
	}
	if !hasCustom2 {
		t.Error("Expected custom2/ in excludes from file")
	}
}

func TestParseExcludeFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "excludes.txt")
	content := "# comment line\nnode_modules/\n \n# another\n*.log\n dist/\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	got, err := fs.ParseExcludeFile(path)
	if err != nil {
		t.Fatalf("ParseExcludeFile returned error: %v", err)
	}

	want := []string{"node_modules/", "*.log", "dist/"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseExcludeFile() = %v, want %v", got, want)
	}
}

func TestExcludeListToRsyncArgs(t *testing.T) {
	list := &fs.ExcludeList{
		Patterns: []string{"node_modules/", "*.log"},
	}

	got := list.ToRsyncArgs()
	want := []string{"--exclude=node_modules/", "--exclude=*.log"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ToRsyncArgs() = %v, want %v", got, want)
	}
}

func TestExcludeListToTarArgs(t *testing.T) {
	list := &fs.ExcludeList{
		Patterns: []string{"node_modules/", "*.log"},
	}

	got := list.ToTarArgs()
	want := []string{"--exclude=*/node_modules/*", "--exclude=*.log"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ToTarArgs() = %v, want %v", got, want)
	}
}

func TestBuildExcludesForcePatterns(t *testing.T) {
	opts := &Options{
		SkipDefaultExcludes:  true,
		ExcludePatterns:      []string{"custom/"},
		ForceExcludePatterns: []string{"repos/"},
	}

	excludeList, err := opts.BuildExcludes()
	if err != nil {
		t.Fatalf("BuildExcludes returned error: %v", err)
	}

	hasRepos := false
	hasCustom := false
	for _, p := range excludeList.Patterns {
		if p == "repos/" {
			hasRepos = true
		}
		if p == "custom/" {
			hasCustom = true
		}
	}

	if !hasRepos {
		t.Error("Expected repos/ in excludes when forced")
	}
	if !hasCustom {
		t.Error("Expected custom/ in excludes")
	}
}

func TestBuildCloneScript(t *testing.T) {
	plans := []repoClonePlan{
		{
			Name:   "frontend",
			Path:   "repos/frontend",
			Remote: "git@github.com:acme/frontend.git",
		},
	}

	script, err := buildCloneScript("/remote/root", plans)
	if err != nil {
		t.Fatalf("buildCloneScript returned error: %v", err)
	}

	if !strings.Contains(script, "git clone") {
		t.Fatalf("expected git clone in script, got: %s", script)
	}
	if !strings.Contains(script, "repos/frontend") {
		t.Fatalf("expected repo path in script, got: %s", script)
	}
	if !strings.Contains(script, "dest=\"$root/$rel\"") {
		t.Fatalf("expected dest to be built from root and rel, got: %s", script)
	}
}

func TestParseCloneOutput(t *testing.T) {
	output := strings.Join([]string{
		"CLONED|frontend|/remote/root/repos/frontend|git@github.com:acme/frontend.git",
		"SKIP|backend|/remote/root/repos/backend|exists",
		"",
	}, "\n")

	results := parseCloneOutput(output)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	if results[0].Status != "cloned" || results[0].Name != "frontend" {
		t.Fatalf("unexpected first result: %+v", results[0])
	}
	if results[1].Status != "skipped" || results[1].Name != "backend" {
		t.Fatalf("unexpected second result: %+v", results[1])
	}
}

func TestBuildCloneScriptTildeRoot(t *testing.T) {
	plans := []repoClonePlan{
		{
			Name:   "frontend",
			Path:   "repos/frontend",
			Remote: "git@github.com:acme/frontend.git",
		},
	}

	script, err := buildCloneScript("~/Code/acme", plans)
	if err != nil {
		t.Fatalf("buildCloneScript returned error: %v", err)
	}

	if !strings.Contains(script, "case \"$root\" in") {
		t.Fatalf("expected tilde expansion case block, got: %s", script)
	}
	if !strings.Contains(script, "root=\"$HOME/${root#~/}\"") {
		t.Fatalf("expected tilde expansion to $HOME, got: %s", script)
	}
}

func TestResolveRepoClonesFallbackToReposDir(t *testing.T) {
	tmp := t.TempDir()
	repoPath := filepath.Join(tmp, "repos", "voxelax")
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}

	remote := "git@github.com:acme/voxelax.git"
	initGitRepo(t, repoPath, remote)

	project := &model.Project{Repos: []model.RepoSpec{}}
	plans, results, err := resolveRepoClones(tmp, project)
	if err != nil {
		t.Fatalf("resolveRepoClones returned error: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected no preflight results, got %d", len(results))
	}
	if len(plans) != 1 {
		t.Fatalf("expected 1 plan, got %d", len(plans))
	}
	if plans[0].Path != "repos/voxelax" {
		t.Fatalf("unexpected plan path: %s", plans[0].Path)
	}
	if plans[0].Remote != remote {
		t.Fatalf("unexpected remote: %s", plans[0].Remote)
	}
}

func TestExpandTildePath(t *testing.T) {
	cases := []struct {
		name    string
		root    string
		home    string
		want    string
		wantErr bool
	}{
		{name: "no-tilde", root: "/data/code", home: "/home/me", want: "/data/code"},
		{name: "tilde-only", root: "~", home: "/home/me", want: "/home/me"},
		{name: "tilde-path", root: "~/Code", home: "/home/me", want: "/home/me/Code"},
		{name: "tilde-user-unsupported", root: "~other/Code", home: "/home/me", wantErr: true},
		{name: "missing-home", root: "~/Code", home: "", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := expandTildePath(tc.root, tc.home)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("expandTildePath(%q, %q) = %q, want %q", tc.root, tc.home, got, tc.want)
			}
		})
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
