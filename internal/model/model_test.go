package model

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewProject(t *testing.T) {
	p := NewProject("myowner", "myproject")

	if p.Schema != CurrentProjectSchema {
		t.Errorf("Schema = %d, want %d", p.Schema, CurrentProjectSchema)
	}
	if p.Slug != "myowner--myproject" {
		t.Errorf("Slug = %q, want %q", p.Slug, "myowner--myproject")
	}
	if p.Owner != "myowner" {
		t.Errorf("Owner = %q, want %q", p.Owner, "myowner")
	}
	if p.Name != "myproject" {
		t.Errorf("Name = %q, want %q", p.Name, "myproject")
	}
	if p.State != StateActive {
		t.Errorf("State = %q, want %q", p.State, StateActive)
	}
	if p.Tags == nil {
		t.Error("Tags should not be nil")
	}
	if p.Repos == nil {
		t.Error("Repos should not be nil")
	}
}

func TestProjectAddRepo(t *testing.T) {
	p := NewProject("owner", "project")
	p.AddRepo("main", "repos/main", "git@github.com:owner/project.git")

	if len(p.Repos) != 1 {
		t.Fatalf("len(Repos) = %d, want 1", len(p.Repos))
	}
	if p.Repos[0].Name != "main" {
		t.Errorf("Repos[0].Name = %q, want %q", p.Repos[0].Name, "main")
	}
	if p.Repos[0].Path != "repos/main" {
		t.Errorf("Repos[0].Path = %q, want %q", p.Repos[0].Path, "repos/main")
	}
	if p.Repos[0].Remote != "git@github.com:owner/project.git" {
		t.Errorf("Repos[0].Remote = %q, want remote URL", p.Repos[0].Remote)
	}
}

func TestProjectJSONRoundTrip(t *testing.T) {
	original := &Project{
		Schema:  1,
		Slug:    "owner--project",
		Owner:   "owner",
		Name:    "project",
		State:   StatePaused,
		Tags:    []string{"golang", "cli"},
		Created: "2024-01-01",
		Updated: "2024-06-01",
		Repos: []RepoSpec{
			{Name: "main", Path: "repos/main", Remote: "git@example.com:repo.git"},
		},
		Notes: "Some notes here",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded Project
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.Slug != original.Slug {
		t.Errorf("Slug = %q, want %q", decoded.Slug, original.Slug)
	}
	if decoded.State != original.State {
		t.Errorf("State = %q, want %q", decoded.State, original.State)
	}
	if len(decoded.Tags) != len(original.Tags) {
		t.Errorf("len(Tags) = %d, want %d", len(decoded.Tags), len(original.Tags))
	}
	if len(decoded.Repos) != len(original.Repos) {
		t.Errorf("len(Repos) = %d, want %d", len(decoded.Repos), len(original.Repos))
	}
}

func TestProjectSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()

	original := NewProject("testowner", "testproj")
	original.Tags = []string{"test"}
	original.AddRepo("main", "repos/main", "")

	if err := original.Save(tmpDir); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	projectPath := filepath.Join(tmpDir, "project.json")
	loaded, err := LoadProject(projectPath)
	if err != nil {
		t.Fatalf("LoadProject error: %v", err)
	}

	if loaded.Slug != original.Slug {
		t.Errorf("Slug = %q, want %q", loaded.Slug, original.Slug)
	}
	if loaded.Owner != original.Owner {
		t.Errorf("Owner = %q, want %q", loaded.Owner, original.Owner)
	}
	if len(loaded.Repos) != 1 {
		t.Errorf("len(Repos) = %d, want 1", len(loaded.Repos))
	}
}

func TestLoadProjectNotFound(t *testing.T) {
	_, err := LoadProject("/nonexistent/path/project.json")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestLoadProjectInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "project.json")
	os.WriteFile(path, []byte("not valid json"), 0644)

	_, err := LoadProject(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestNewIndexRecord(t *testing.T) {
	r := NewIndexRecord("owner--project", "/path/to/workspace")

	if r.Schema != CurrentIndexSchema {
		t.Errorf("Schema = %d, want %d", r.Schema, CurrentIndexSchema)
	}
	if r.Slug != "owner--project" {
		t.Errorf("Slug = %q, want %q", r.Slug, "owner--project")
	}
	if r.Path != "/path/to/workspace" {
		t.Errorf("Path = %q, want %q", r.Path, "/path/to/workspace")
	}
	if !r.Valid {
		t.Error("Valid should be true by default")
	}
}

func TestIndexRecordToJSON(t *testing.T) {
	r := NewIndexRecord("owner--project", "/path")
	r.Owner = "owner"
	r.State = StateActive

	data, err := r.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON error: %v", err)
	}

	var decoded IndexRecord
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.Slug != r.Slug {
		t.Errorf("Slug = %q, want %q", decoded.Slug, r.Slug)
	}
}

func TestIndexFindBySlug(t *testing.T) {
	idx := NewIndex()
	idx.Add(&IndexRecord{Slug: "a--one", Owner: "a"})
	idx.Add(&IndexRecord{Slug: "b--two", Owner: "b"})
	idx.Add(&IndexRecord{Slug: "c--three", Owner: "c"})

	found := idx.FindBySlug("b--two")
	if found == nil {
		t.Fatal("FindBySlug returned nil")
	}
	if found.Owner != "b" {
		t.Errorf("Owner = %q, want %q", found.Owner, "b")
	}

	notFound := idx.FindBySlug("nonexistent")
	if notFound != nil {
		t.Error("FindBySlug should return nil for nonexistent slug")
	}
}

func TestIndexFilterByOwner(t *testing.T) {
	idx := NewIndex()
	idx.Add(&IndexRecord{Slug: "alice--one", Owner: "alice"})
	idx.Add(&IndexRecord{Slug: "bob--two", Owner: "bob"})
	idx.Add(&IndexRecord{Slug: "alice--three", Owner: "alice"})

	aliceRecords := idx.FilterByOwner("alice")
	if len(aliceRecords) != 2 {
		t.Errorf("len(aliceRecords) = %d, want 2", len(aliceRecords))
	}

	bobRecords := idx.FilterByOwner("bob")
	if len(bobRecords) != 1 {
		t.Errorf("len(bobRecords) = %d, want 1", len(bobRecords))
	}

	noRecords := idx.FilterByOwner("charlie")
	if len(noRecords) != 0 {
		t.Errorf("len(noRecords) = %d, want 0", len(noRecords))
	}
}

func TestIndexFilterByState(t *testing.T) {
	idx := NewIndex()
	idx.Add(&IndexRecord{Slug: "a--one", State: StateActive})
	idx.Add(&IndexRecord{Slug: "b--two", State: StatePaused})
	idx.Add(&IndexRecord{Slug: "c--three", State: StateActive})
	idx.Add(&IndexRecord{Slug: "d--four", State: StateArchived})

	active := idx.FilterByState(StateActive)
	if len(active) != 2 {
		t.Errorf("len(active) = %d, want 2", len(active))
	}

	paused := idx.FilterByState(StatePaused)
	if len(paused) != 1 {
		t.Errorf("len(paused) = %d, want 1", len(paused))
	}

	scratch := idx.FilterByState(StateScratch)
	if len(scratch) != 0 {
		t.Errorf("len(scratch) = %d, want 0", len(scratch))
	}
}

func TestIndexFilterByTag(t *testing.T) {
	idx := NewIndex()
	idx.Add(&IndexRecord{Slug: "a--one", Tags: []string{"golang", "cli"}})
	idx.Add(&IndexRecord{Slug: "b--two", Tags: []string{"python"}})
	idx.Add(&IndexRecord{Slug: "c--three", Tags: []string{"golang", "web"}})
	idx.Add(&IndexRecord{Slug: "d--four", Tags: nil})

	golang := idx.FilterByTag("golang")
	if len(golang) != 2 {
		t.Errorf("len(golang) = %d, want 2", len(golang))
	}

	python := idx.FilterByTag("python")
	if len(python) != 1 {
		t.Errorf("len(python) = %d, want 1", len(python))
	}

	rust := idx.FilterByTag("rust")
	if len(rust) != 0 {
		t.Errorf("len(rust) = %d, want 0", len(rust))
	}
}

func TestIndexSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	indexPath := filepath.Join(tmpDir, "index.jsonl")

	original := NewIndex()
	now := time.Now()
	original.Add(&IndexRecord{
		Schema:       1,
		Slug:         "owner--project",
		Path:         "/path/to/workspace",
		Owner:        "owner",
		State:        StateActive,
		Tags:         []string{"test"},
		RepoCount:    2,
		LastCommitAt: &now,
		Valid:        true,
	})

	if err := original.Save(indexPath); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	loaded, err := LoadIndex(indexPath)
	if err != nil {
		t.Fatalf("LoadIndex error: %v", err)
	}

	if len(loaded.Records) != 1 {
		t.Fatalf("len(Records) = %d, want 1", len(loaded.Records))
	}

	r := loaded.Records[0]
	if r.Slug != "owner--project" {
		t.Errorf("Slug = %q, want %q", r.Slug, "owner--project")
	}
	if r.Owner != "owner" {
		t.Errorf("Owner = %q, want %q", r.Owner, "owner")
	}
}

func TestLoadIndexNotFound(t *testing.T) {
	idx, err := LoadIndex("/nonexistent/index.jsonl")
	if err != nil {
		t.Fatalf("LoadIndex should not error for nonexistent file: %v", err)
	}
	if len(idx.Records) != 0 {
		t.Errorf("len(Records) = %d, want 0", len(idx.Records))
	}
}
