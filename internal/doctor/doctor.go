package doctor

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/tormodhaugland/co/internal/fs"
	"github.com/tormodhaugland/co/internal/git"
	"github.com/tormodhaugland/co/internal/model"
)

type MissingProject struct {
	Slug  string
	Path  string
	Owner string
	Name  string
}

func FindMissingProjects(codeRoot string) ([]MissingProject, error) {
	workspaces, err := fs.ListWorkspaces(codeRoot)
	if err != nil {
		return nil, err
	}

	missing := make([]MissingProject, 0)
	for _, slug := range workspaces {
		workspacePath := filepath.Join(codeRoot, slug)
		if fs.HasProjectJSON(workspacePath) {
			continue
		}

		owner, name, ok := ParseSlug(slug)
		if !ok {
			return nil, fmt.Errorf("invalid workspace slug: %s", slug)
		}

		missing = append(missing, MissingProject{
			Slug:  slug,
			Path:  workspacePath,
			Owner: owner,
			Name:  name,
		})
	}

	return missing, nil
}

func CreateProjectJSON(slug, workspacePath string) (*model.Project, error) {
	project, err := BuildProject(slug, workspacePath)
	if err != nil {
		return nil, err
	}

	if err := project.Save(workspacePath); err != nil {
		return nil, err
	}

	return project, nil
}

func BuildProject(slug, workspacePath string) (*model.Project, error) {
	owner, name, ok := ParseSlug(slug)
	if !ok {
		return nil, fmt.Errorf("invalid workspace slug: %s", slug)
	}

	project := model.NewProject(owner, name)
	project.Slug = slug

	repos, err := fs.ListRepos(workspacePath)
	if err != nil {
		return nil, err
	}

	for _, repoName := range repos {
		repoPath := filepath.Join(workspacePath, "repos", repoName)

		remote := ""
		if git.IsRepo(repoPath) {
			if info, err := git.GetInfo(repoPath); err == nil {
				remote = info.Remote
			}
		}

		project.AddRepo(repoName, "repos/"+repoName, remote)
	}

	return project, nil
}

func ParseSlug(slug string) (string, string, bool) {
	parts := strings.Split(slug, "--")
	if len(parts) < 2 {
		return "", "", false
	}

	owner := strings.TrimSpace(parts[0])
	name := strings.TrimSpace(strings.Join(parts[1:], "--"))
	if owner == "" || name == "" {
		return "", "", false
	}

	return owner, name, true
}
