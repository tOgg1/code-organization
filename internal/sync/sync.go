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
	RemoteExists bool   `json:"remote_exists"`
	ActionTaken  string `json:"action_taken"`
	BytesSent    int64  `json:"bytes_sent,omitempty"`
	DurationMs   int64  `json:"duration_ms"`
	Error        string `json:"error,omitempty"`
}

type Options struct {
	Force    bool
	DryRun   bool
	NoGit    bool
	Excludes []string
}

func DefaultOptions() *Options {
	return &Options{
		Force:    false,
		DryRun:   false,
		NoGit:    false,
		Excludes: fs.DefaultExcludes(),
	}
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
	args := []string{"-az", "--partial", "--progress"}

	for _, exclude := range opts.Excludes {
		args = append(args, "--exclude="+exclude)
	}

	if opts.NoGit {
		args = append(args, "--exclude=.git/")
	}

	args = append(args, localPath+"/")
	args = append(args, sshHost+":"+remotePath+"/")

	cmd := exec.Command("rsync", args...)
	return cmd.Run()
}

func tarSyncWorkspace(localPath, sshHost, remotePath string, opts *Options) error {
	excludeArgs := []string{}
	for _, exclude := range opts.Excludes {
		excludeArgs = append(excludeArgs, "--exclude="+exclude)
	}
	if opts.NoGit {
		excludeArgs = append(excludeArgs, "--exclude=.git")
	}

	tarArgs := append([]string{"-czf", "-"}, excludeArgs...)
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

func BuildExcludeList(noGit bool) []string {
	excludes := fs.DefaultExcludes()
	if noGit {
		excludes = append(excludes, ".git")
	}
	return excludes
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
