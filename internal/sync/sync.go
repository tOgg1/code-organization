package sync

import (
	"fmt"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/tormodhaugland/co/internal/config"
	"github.com/tormodhaugland/co/internal/fs"
	"github.com/tormodhaugland/co/internal/git"
	"github.com/tormodhaugland/co/internal/model"
)

type Result struct {
	RemoteExists bool         `json:"remote_exists"`
	ActionTaken  string       `json:"action_taken"`
	BytesSent    int64        `json:"bytes_sent,omitempty"`
	DurationMs   int64        `json:"duration_ms"`
	Error        string       `json:"error,omitempty"`
	Excludes     []string     `json:"excludes,omitempty"`
	RepoResults  []RepoResult `json:"repo_results,omitempty"`
}

type RepoResult struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Remote  string `json:"remote,omitempty"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type Options struct {
	Force      bool
	DryRun     bool
	NoGit      bool
	IncludeEnv bool
	// ForceExcludePatterns are always excluded, even in interactive mode.
	ForceExcludePatterns []string
	// Additional exclude patterns from CLI --exclude flags
	ExcludePatterns []string
	// File to read additional exclude patterns from (--exclude-from)
	ExcludeFromFile string
	// SkipDefaultExcludes when true, only use ExcludePatterns without builtin defaults.
	// Used by interactive mode where user has explicitly selected what to exclude.
	SkipDefaultExcludes bool
	// WorkspaceAdd contains exclude patterns from project.json sync.excludes.add
	WorkspaceAdd []string
	// WorkspaceRemove contains patterns to remove from defaults via project.json sync.excludes.remove
	WorkspaceRemove []string
	// Project provides workspace metadata for clone-based sync.
	Project *model.Project
}

func DefaultOptions() *Options {
	return &Options{
		Force:                false,
		DryRun:               false,
		NoGit:                false,
		IncludeEnv:           false,
		ForceExcludePatterns: []string{"repos/"},
		ExcludePatterns:      nil,
		ExcludeFromFile:      "",
	}
}

// BuildExcludes constructs the effective exclude list from options.
// Precedence (low to high): BuiltinDefaults -> WorkspaceConfig -> CLI flags
func (o *Options) BuildExcludes() (*fs.ExcludeList, error) {
	// Collect CLI patterns
	cliPatterns := make([]string, 0, len(o.ExcludePatterns))
	cliPatterns = append(cliPatterns, o.ExcludePatterns...)

	// Read patterns from file if specified
	if o.ExcludeFromFile != "" {
		filePatterns, err := fs.ParseExcludeFile(o.ExcludeFromFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read exclude file: %w", err)
		}
		cliPatterns = append(cliPatterns, filePatterns...)
	}

	// When SkipDefaultExcludes is set (interactive mode), use only the provided patterns
	if o.SkipDefaultExcludes {
		patterns := append([]string{}, o.ForceExcludePatterns...)
		patterns = append(patterns, cliPatterns...)
		return &fs.ExcludeList{Patterns: patterns}, nil
	}

	// Combine workspace and CLI additions (workspace first, then CLI)
	allAdditional := make([]string, 0, len(o.WorkspaceAdd)+len(cliPatterns))
	allAdditional = append(allAdditional, o.WorkspaceAdd...)
	allAdditional = append(allAdditional, cliPatterns...)

	excludeList := fs.BuildExcludeList(fs.ExcludeOptions{
		Additional: allAdditional,
		Remove:     o.WorkspaceRemove,
		NoGit:      o.NoGit,
		IncludeEnv: o.IncludeEnv,
	})

	excludeList.Patterns = appendMissing(excludeList.Patterns, o.ForceExcludePatterns)
	return excludeList, nil
}

func SyncWorkspace(localPath string, server *config.ServerConfig, slug string, opts *Options) (*Result, error) {
	start := time.Now()
	result := &Result{}

	remotePath := server.CodeRoot + "/" + slug
	project, err := loadProjectForSync(localPath, opts.Project)
	if err != nil {
		result.Error = err.Error()
		return result, err
	}

	plans, preflightResults, err := resolveRepoClones(localPath, project)
	if err != nil {
		result.Error = err.Error()
		return result, err
	}
	result.RepoResults = append(result.RepoResults, preflightResults...)

	exists, err := remotePathExists(server.SSH, remotePath)
	if err != nil {
		result.Error = err.Error()
		return result, err
	}
	result.RemoteExists = exists

	if exists && !opts.Force {
		result.ActionTaken = "skipped"
		result.DurationMs = time.Since(start).Milliseconds()
		return result, nil
	}

	if opts.DryRun {
		result.ActionTaken = "dry_run"
		result.DurationMs = time.Since(start).Milliseconds()
		return result, nil
	}

	if err := createRemoteDir(server.SSH, remotePath); err != nil {
		result.Error = err.Error()
		return result, err
	}

	if err := rsyncWorkspace(localPath, server.SSH, remotePath, opts); err != nil {
		if err := tarSyncWorkspace(localPath, server.SSH, remotePath, opts); err != nil {
			result.Error = err.Error()
			return result, err
		}
	}

	cloneResults, err := cloneRepos(server.SSH, remotePath, plans)
	result.RepoResults = append(result.RepoResults, cloneResults...)
	if err != nil {
		result.Error = err.Error()
		return result, err
	}

	if exists {
		result.ActionTaken = "forced_sync"
	} else {
		result.ActionTaken = "synced"
	}

	result.DurationMs = time.Since(start).Milliseconds()
	return result, nil
}

func remotePathExists(sshHost, remotePath string) (bool, error) {
	cmd := exec.Command("ssh", sshHost, fmt.Sprintf("test -d %s", remotePath))
	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 1 {
				return false, nil
			}
		}
		return false, err
	}
	return true, nil
}

func createRemoteDir(sshHost, remotePath string) error {
	cmd := exec.Command("ssh", sshHost, fmt.Sprintf("mkdir -p %s", remotePath))
	return cmd.Run()
}

func rsyncWorkspace(localPath, sshHost, remotePath string, opts *Options) error {
	excludeList, err := opts.BuildExcludes()
	if err != nil {
		return err
	}

	args := []string{"-az", "--partial", "--progress"}
	args = append(args, excludeList.ToRsyncArgs()...)
	args = append(args, localPath+"/")
	args = append(args, sshHost+":"+remotePath+"/")

	cmd := exec.Command("rsync", args...)
	return cmd.Run()
}

func tarSyncWorkspace(localPath, sshHost, remotePath string, opts *Options) error {
	excludeList, err := opts.BuildExcludes()
	if err != nil {
		return err
	}

	tarArgs := []string{"-czf", "-"}
	tarArgs = append(tarArgs, excludeList.ToTarArgs()...)
	tarArgs = append(tarArgs, "-C", localPath, ".")

	tarCmd := exec.Command("tar", tarArgs...)

	sshCmd := exec.Command("ssh", sshHost, fmt.Sprintf("tar -xzf - -C %s", remotePath))

	pipe, err := tarCmd.StdoutPipe()
	if err != nil {
		return err
	}
	sshCmd.Stdin = pipe

	if err := tarCmd.Start(); err != nil {
		return err
	}

	if err := sshCmd.Start(); err != nil {
		return err
	}

	if err := tarCmd.Wait(); err != nil {
		return err
	}

	return sshCmd.Wait()
}

func FormatResult(r *Result) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Action: %s\n", r.ActionTaken))
	sb.WriteString(fmt.Sprintf("Remote existed: %v\n", r.RemoteExists))
	if len(r.RepoResults) > 0 {
		cloned := 0
		skipped := 0
		for _, repo := range r.RepoResults {
			switch repo.Status {
			case "cloned":
				cloned++
			case "skipped":
				skipped++
			}
		}
		sb.WriteString(fmt.Sprintf("Repos cloned: %d, skipped: %d\n", cloned, skipped))
	}
	sb.WriteString(fmt.Sprintf("Duration: %dms\n", r.DurationMs))
	if r.Error != "" {
		sb.WriteString(fmt.Sprintf("Error: %s\n", r.Error))
	}
	return sb.String()
}

type repoClonePlan struct {
	Name   string
	Path   string
	Remote string
}

func loadProjectForSync(localPath string, project *model.Project) (*model.Project, error) {
	if project != nil {
		return project, nil
	}
	projectPath := filepath.Join(localPath, "project.json")
	loaded, err := model.LoadProject(projectPath)
	if err != nil {
		return nil, fmt.Errorf("project.json required for sync: %w", err)
	}
	return loaded, nil
}

func resolveRepoClones(localPath string, project *model.Project) ([]repoClonePlan, []RepoResult, error) {
	plans := make([]repoClonePlan, 0, len(project.Repos))
	results := make([]RepoResult, 0, len(project.Repos))

	for _, repo := range project.Repos {
		if strings.TrimSpace(repo.Path) == "" {
			results = append(results, RepoResult{
				Name:    repo.Name,
				Path:    repo.Path,
				Status:  "skipped",
				Message: "missing repo path",
			})
			continue
		}

		cleanPath, err := safeRepoPath(repo.Path)
		if err != nil {
			results = append(results, RepoResult{
				Name:    repo.Name,
				Path:    repo.Path,
				Status:  "skipped",
				Message: err.Error(),
			})
			continue
		}

		remote := strings.TrimSpace(repo.Remote)
		if remote == "" {
			localRepoPath := filepath.Join(localPath, cleanPath)
			if info, err := git.GetInfo(localRepoPath); err == nil {
				remote = strings.TrimSpace(info.Remote)
			}
		}

		if remote == "" {
			results = append(results, RepoResult{
				Name:    repo.Name,
				Path:    cleanPath,
				Status:  "skipped",
				Message: "missing remote",
			})
			continue
		}

		plans = append(plans, repoClonePlan{
			Name:   repo.Name,
			Path:   cleanPath,
			Remote: remote,
		})
	}

	return plans, results, nil
}

func safeRepoPath(repoPath string) (string, error) {
	clean := path.Clean(strings.TrimSpace(repoPath))
	if clean == "." || clean == "" {
		return "", fmt.Errorf("invalid repo path")
	}
	if path.IsAbs(clean) || strings.HasPrefix(clean, "../") || clean == ".." {
		return "", fmt.Errorf("invalid repo path: %s", repoPath)
	}
	return clean, nil
}

func cloneRepos(sshHost, remoteRoot string, plans []repoClonePlan) ([]RepoResult, error) {
	if len(plans) == 0 {
		return nil, nil
	}

	script, err := buildCloneScript(remoteRoot, plans)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command("ssh", sshHost, "sh", "-s")
	cmd.Stdin = strings.NewReader(script)
	output, err := cmd.CombinedOutput()

	results := parseCloneOutput(string(output))
	if err != nil {
		return results, fmt.Errorf("clone repos failed: %w: %s", err, strings.TrimSpace(string(output)))
	}

	return results, nil
}

func buildCloneScript(remoteRoot string, plans []repoClonePlan) (string, error) {
	var sb strings.Builder
	sb.WriteString("set -e\n")
	sb.WriteString("root=" + shellEscape(remoteRoot) + "\n")
	sb.WriteString("mkdir -p \"$root\"\n")
	sb.WriteString("mkdir -p \"$root/repos\"\n")

	for _, plan := range plans {
		dest := path.Join(remoteRoot, plan.Path)
		sb.WriteString("name=" + shellEscape(plan.Name) + "\n")
		sb.WriteString("url=" + shellEscape(plan.Remote) + "\n")
		sb.WriteString("dest=" + shellEscape(dest) + "\n")
		sb.WriteString("if [ -d \"$dest\" ]; then\n")
		sb.WriteString("  echo \"SKIP|$name|$dest|exists\"\n")
		sb.WriteString("else\n")
		sb.WriteString("  mkdir -p \"$(dirname \\\"$dest\\\")\"\n")
		sb.WriteString("  git clone \"$url\" \"$dest\"\n")
		sb.WriteString("  echo \"CLONED|$name|$dest|$url\"\n")
		sb.WriteString("fi\n")
	}

	return sb.String(), nil
}

func parseCloneOutput(output string) []RepoResult {
	lines := strings.Split(output, "\n")
	results := make([]RepoResult, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "CLONED|") {
			parts := strings.SplitN(line, "|", 4)
			if len(parts) == 4 {
				results = append(results, RepoResult{
					Name:   parts[1],
					Path:   parts[2],
					Remote: parts[3],
					Status: "cloned",
				})
			}
			continue
		}
		if strings.HasPrefix(line, "SKIP|") {
			parts := strings.SplitN(line, "|", 4)
			if len(parts) == 4 {
				results = append(results, RepoResult{
					Name:    parts[1],
					Path:    parts[2],
					Status:  "skipped",
					Message: parts[3],
				})
			}
		}
	}

	return results
}

func appendMissing(patterns []string, additional []string) []string {
	seen := make(map[string]struct{}, len(patterns))
	for _, pattern := range patterns {
		seen[pattern] = struct{}{}
	}
	for _, pattern := range additional {
		if _, ok := seen[pattern]; ok {
			continue
		}
		patterns = append(patterns, pattern)
		seen[pattern] = struct{}{}
	}
	return patterns
}

func shellEscape(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}
