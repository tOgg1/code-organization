package model

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type ProjectState string

const (
	StateActive   ProjectState = "active"
	StatePaused   ProjectState = "paused"
	StateArchived ProjectState = "archived"
	StateScratch  ProjectState = "scratch"
)

type RepoSpec struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Remote string `json:"remote,omitempty"`
}

type Project struct {
	Schema       int               `json:"schema"`
	Slug         string            `json:"slug"`
	Owner        string            `json:"owner"`
	Name         string            `json:"name"`
	State        ProjectState      `json:"state"`
	Tags         []string          `json:"tags,omitempty"`
	Created      string            `json:"created"`
	Updated      string            `json:"updated"`
	Repos        []RepoSpec        `json:"repos"`
	Notes        string            `json:"notes,omitempty"`
	Template     string            `json:"template,omitempty"`      // Template used to create workspace
	TemplateVars map[string]string `json:"template_vars,omitempty"` // Variables used during creation
}

const CurrentProjectSchema = 1

func NewProject(owner, name string) *Project {
	now := time.Now().Format("2006-01-02")
	return &Project{
		Schema:  CurrentProjectSchema,
		Slug:    owner + "--" + name,
		Owner:   owner,
		Name:    name,
		State:   StateActive,
		Tags:    []string{},
		Created: now,
		Updated: now,
		Repos:   []RepoSpec{},
	}
}

func LoadProject(path string) (*Project, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var p Project
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}

	return &p, nil
}

func (p *Project) Save(workspacePath string) error {
	p.Updated = time.Now().Format("2006-01-02")

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}

	projectPath := filepath.Join(workspacePath, "project.json")
	return os.WriteFile(projectPath, append(data, '\n'), 0644)
}

func (p *Project) AddRepo(name, path, remote string) {
	p.Repos = append(p.Repos, RepoSpec{
		Name:   name,
		Path:   path,
		Remote: remote,
	})
}
