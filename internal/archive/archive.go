package archive

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/tormodhaugland/co/internal/config"
	"github.com/tormodhaugland/co/internal/fs"
	"github.com/tormodhaugland/co/internal/git"
)

type ArchiveMeta struct {
	Schema      int       `json:"schema"`
	Slug        string    `json:"slug"`
	ArchivedAt  time.Time `json:"archived_at"`
	Reason      string    `json:"reason,omitempty"`
	BundleCount int       `json:"bundle_count"`
	FullArchive bool      `json:"full_archive,omitempty"`
}

type Result struct {
	ArchivePath string `json:"archive_path"`
	BundleCount int    `json:"bundle_count"`
	FullArchive bool   `json:"full_archive,omitempty"`
	Deleted     bool   `json:"deleted"`
	Error       string `json:"error,omitempty"`
}

type Options struct {
	Reason      string
	DeleteAfter bool
	Full        bool
}

func ArchiveWorkspace(cfg *config.Config, slug string, opts Options) (*Result, error) {
	workspacePath := cfg.WorkspacePath(slug)
	if !fs.WorkspaceExists(cfg.CodeRoot, slug) {
		return nil, fmt.Errorf("workspace not found: %s", slug)
	}

	now := time.Now()
	year := now.Format("2006")
	timestamp := now.Format("20060102-150405")

	archiveDir := filepath.Join(cfg.ArchiveDir(), year)
	if err := fs.EnsureDir(archiveDir); err != nil {
		return nil, fmt.Errorf("failed to create archive directory: %w", err)
	}

	if opts.Full {
		return archiveFullWorkspace(cfg, slug, workspacePath, archiveDir, timestamp, now, opts)
	}

	return archiveBundlesOnly(cfg, slug, workspacePath, archiveDir, timestamp, now, opts)
}

func archiveFullWorkspace(cfg *config.Config, slug, workspacePath, archiveDir, timestamp string, now time.Time, opts Options) (*Result, error) {
	result := &Result{FullArchive: true}

	archiveName := fmt.Sprintf("%s--%s--full.tar.gz", slug, timestamp)
	archivePath := filepath.Join(archiveDir, archiveName)

	if err := createTarGz(workspacePath, archivePath); err != nil {
		return nil, fmt.Errorf("failed to create full archive: %w", err)
	}

	result.ArchivePath = archivePath

	if opts.DeleteAfter {
		if err := os.RemoveAll(workspacePath); err != nil {
			return nil, fmt.Errorf("failed to delete workspace: %w", err)
		}
		result.Deleted = true
	}

	return result, nil
}

func archiveBundlesOnly(cfg *config.Config, slug, workspacePath, archiveDir, timestamp string, now time.Time, opts Options) (*Result, error) {
	result := &Result{}

	archiveName := fmt.Sprintf("%s--%s.tar.gz", slug, timestamp)

	tmpDir, err := os.MkdirTemp("", "co-archive-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	projectSrc := filepath.Join(workspacePath, "project.json")
	projectDst := filepath.Join(tmpDir, "project.json")
	if err := copyFile(projectSrc, projectDst); err != nil {
		return nil, fmt.Errorf("failed to copy project.json: %w", err)
	}

	repos, err := fs.ListRepos(workspacePath)
	if err != nil {
		return nil, fmt.Errorf("failed to list repos: %w", err)
	}

	bundleCount := 0
	for _, repoName := range repos {
		repoPath := filepath.Join(workspacePath, "repos", repoName)
		if !git.IsRepo(repoPath) {
			continue
		}

		bundleName := fmt.Sprintf("repos__%s.bundle", repoName)
		bundlePath := filepath.Join(tmpDir, bundleName)

		if err := git.CreateBundle(repoPath, bundlePath); err != nil {
			return nil, fmt.Errorf("failed to create bundle for %s: %w", repoName, err)
		}
		bundleCount++
	}

	meta := ArchiveMeta{
		Schema:      1,
		Slug:        slug,
		ArchivedAt:  now,
		Reason:      opts.Reason,
		BundleCount: bundleCount,
	}

	metaPath := filepath.Join(tmpDir, "archive-meta.json")
	metaData, _ := json.MarshalIndent(meta, "", "  ")
	if err := os.WriteFile(metaPath, metaData, 0644); err != nil {
		return nil, fmt.Errorf("failed to write archive-meta.json: %w", err)
	}

	archivePath := filepath.Join(archiveDir, archiveName)
	if err := createTarGz(tmpDir, archivePath); err != nil {
		return nil, fmt.Errorf("failed to create archive: %w", err)
	}

	result.ArchivePath = archivePath
	result.BundleCount = bundleCount

	if opts.DeleteAfter {
		if err := os.RemoveAll(workspacePath); err != nil {
			return nil, fmt.Errorf("failed to delete workspace: %w", err)
		}
		result.Deleted = true
	}

	return result, nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

func createTarGz(srcDir, dstPath string) error {
	cmd := exec.Command("tar", "-czf", dstPath, "-C", srcDir, ".")
	return cmd.Run()
}
