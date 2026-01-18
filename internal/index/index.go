package index

import (
	"path/filepath"
	"sync"
	"time"

	"github.com/tormodhaugland/co/internal/config"
	"github.com/tormodhaugland/co/internal/fs"
	"github.com/tormodhaugland/co/internal/git"
	"github.com/tormodhaugland/co/internal/model"
)

type Builder struct {
	cfg              *config.Config
	workers          int
	syncProjectRepos bool
	progress         func(done, total int)
}

func NewBuilder(cfg *config.Config) *Builder {
	return &Builder{
		cfg:              cfg,
		workers:          4,
		syncProjectRepos: true,
	}
}

func (b *Builder) SetSyncProjectRepos(enabled bool) {
	b.syncProjectRepos = enabled
}

func (b *Builder) SetProgress(fn func(done, total int)) {
	b.progress = fn
}

func (b *Builder) Build() (*model.Index, error) {
	workspaces, err := fs.ListWorkspaces(b.cfg.CodeRoot)
	if err != nil {
		return nil, err
	}

	index := model.NewIndex()
	results := make(chan *model.IndexRecord, len(workspaces))

	var wg sync.WaitGroup
	sem := make(chan struct{}, b.workers)

	for _, slug := range workspaces {
		wg.Add(1)
		go func(slug string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			record := b.buildRecord(slug)
			results <- record
		}(slug)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	total := len(workspaces)
	done := 0

	for record := range results {
		index.Add(record)
		done++
		if b.progress != nil {
			b.progress(done, total)
		}
	}

	return index, nil
}

func (b *Builder) buildRecord(slug string) *model.IndexRecord {
	workspacePath := b.cfg.WorkspacePath(slug)
	record := model.NewIndexRecord(slug, workspacePath)

	if !fs.HasProjectJSON(workspacePath) {
		record.Valid = false
		record.Error = "missing project.json"
		return record
	}

	proj, err := model.LoadProject(filepath.Join(workspacePath, "project.json"))
	if err != nil {
		record.Valid = false
		record.Error = "invalid project.json: " + err.Error()
		return record
	}

	record.Owner = proj.Owner
	record.State = proj.State
	record.Tags = proj.Tags

	repos, err := fs.ListRepos(workspacePath)
	if err == nil {
		record.RepoCount = len(repos)

		var latestCommit time.Time
		var dirtyCount int
		repoSpecs := make([]model.RepoSpec, 0, len(repos))

		for _, repoName := range repos {
			repoPath := filepath.Join(workspacePath, "repos", repoName)

			var repoInfo model.IndexRepoInfo
			repoInfo.Name = repoName
			repoInfo.Path = "repos/" + repoName

			repoSpec := model.RepoSpec{
				Name: repoName,
				Path: "repos/" + repoName,
			}

			if git.IsRepo(repoPath) {
				info, err := git.GetInfo(repoPath)
				if err == nil {
					repoInfo.Head = info.Head
					repoInfo.Branch = info.Branch
					repoInfo.Dirty = info.Dirty
					repoInfo.Remote = info.Remote
					repoSpec.Remote = info.Remote

					if info.Dirty {
						dirtyCount++
					}

					if info.LastCommit.After(latestCommit) {
						latestCommit = info.LastCommit
					}
				}
			}

			record.Repos = append(record.Repos, repoInfo)
			repoSpecs = append(repoSpecs, repoSpec)
		}

		record.DirtyRepos = dirtyCount
		if !latestCommit.IsZero() {
			record.LastCommitAt = &latestCommit
		}

		if b.syncProjectRepos {
			if !repoSpecsEqual(proj.Repos, repoSpecs) {
				proj.Repos = repoSpecs
				_ = proj.Save(workspacePath)
			}
		}
	}

	size, err := fs.CalculateSize(workspacePath)
	if err == nil {
		record.SizeBytes = size
	}

	lastMod, err := fs.GetLastModTime(workspacePath)
	if err == nil && lastMod > 0 {
		t := time.Unix(lastMod, 0)
		record.LastFSChangeAt = &t
	}

	return record
}

func repoSpecsEqual(left, right []model.RepoSpec) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i].Name != right[i].Name {
			return false
		}
		if left[i].Path != right[i].Path {
			return false
		}
		if left[i].Remote != right[i].Remote {
			return false
		}
	}
	return true
}

func (b *Builder) Save(index *model.Index) error {
	if err := fs.EnsureDir(b.cfg.SystemDir()); err != nil {
		return err
	}
	return index.Save(b.cfg.IndexPath())
}
