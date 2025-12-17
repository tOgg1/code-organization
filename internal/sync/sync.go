package sync

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/tormodhaugland/co/internal/config"
	"github.com/tormodhaugland/co/internal/fs"
)

type Result struct {
	RemoteExists bool     `json:"remote_exists"`
	ActionTaken  string   `json:"action_taken"`
	BytesSent    int64    `json:"bytes_sent,omitempty"`
	DurationMs   int64    `json:"duration_ms"`
	Error        string   `json:"error,omitempty"`
	Excludes     []string `json:"excludes,omitempty"`
}

type Options struct {
	Force      bool
	DryRun     bool
	NoGit      bool
	IncludeEnv bool
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
}

func DefaultOptions() *Options {
	return &Options{
		Force:           false,
		DryRun:          false,
		NoGit:           false,
		IncludeEnv:      false,
		ExcludePatterns: nil,
		ExcludeFromFile: "",
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
		return &fs.ExcludeList{Patterns: cliPatterns}, nil
	}

	// Combine workspace and CLI additions (workspace first, then CLI)
	allAdditional := make([]string, 0, len(o.WorkspaceAdd)+len(cliPatterns))
	allAdditional = append(allAdditional, o.WorkspaceAdd...)
	allAdditional = append(allAdditional, cliPatterns...)

	return fs.BuildExcludeList(fs.ExcludeOptions{
		Additional: allAdditional,
		Remove:     o.WorkspaceRemove,
		NoGit:      o.NoGit,
		IncludeEnv: o.IncludeEnv,
	}), nil
}

func SyncWorkspace(localPath string, server *config.ServerConfig, slug string, opts *Options) (*Result, error) {
	start := time.Now()
	result := &Result{}

	remotePath := server.CodeRoot + "/" + slug

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
	sb.WriteString(fmt.Sprintf("Duration: %dms\n", r.DurationMs))
	if r.Error != "" {
		sb.WriteString(fmt.Sprintf("Error: %s\n", r.Error))
	}
	return sb.String()
}
