package model

import (
	"bufio"
	"encoding/json"
	"os"
	"time"
)

type IndexRepoInfo struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Remote string `json:"remote,omitempty"`
	Head   string `json:"head,omitempty"`
	Branch string `json:"branch,omitempty"`
	Dirty  bool   `json:"dirty"`
}

type IndexRecord struct {
	Schema         int             `json:"schema"`
	Slug           string          `json:"slug"`
	Path           string          `json:"path"`
	Owner          string          `json:"owner"`
	State          ProjectState    `json:"state"`
	Tags           []string        `json:"tags,omitempty"`
	RepoCount      int             `json:"repo_count"`
	LastCommitAt   *time.Time      `json:"last_commit_at,omitempty"`
	LastFSChangeAt *time.Time      `json:"last_fs_change_at,omitempty"`
	DirtyRepos     int             `json:"dirty_repos"`
	SizeBytes      int64           `json:"size_bytes"`
	Repos          []IndexRepoInfo `json:"repos"`
	Valid          bool            `json:"valid"`
	Error          string          `json:"error,omitempty"`
}

const CurrentIndexSchema = 1

func NewIndexRecord(slug, path string) *IndexRecord {
	return &IndexRecord{
		Schema: CurrentIndexSchema,
		Slug:   slug,
		Path:   path,
		Tags:   []string{},
		Repos:  []IndexRepoInfo{},
		Valid:  true,
	}
}

func (r *IndexRecord) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}

type Index struct {
	Records []*IndexRecord
}

func NewIndex() *Index {
	return &Index{
		Records: []*IndexRecord{},
	}
}

func LoadIndex(path string) (*Index, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NewIndex(), nil
		}
		return nil, err
	}
	defer file.Close()

	index := NewIndex()
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		var record IndexRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			continue
		}
		index.Records = append(index.Records, &record)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return index, nil
}

func (idx *Index) Save(path string) error {
	tmpPath := path + ".tmp"
	file, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	for _, record := range idx.Records {
		data, err := record.ToJSON()
		if err != nil {
			file.Close()
			os.Remove(tmpPath)
			return err
		}
		file.Write(data)
		file.Write([]byte("\n"))
	}

	if err := file.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}

	return os.Rename(tmpPath, path)
}

func (idx *Index) Add(record *IndexRecord) {
	idx.Records = append(idx.Records, record)
}

// Remove removes a record by slug. Returns true if a record was removed.
func (idx *Index) Remove(slug string) bool {
	for i, r := range idx.Records {
		if r.Slug == slug {
			idx.Records = append(idx.Records[:i], idx.Records[i+1:]...)
			return true
		}
	}
	return false
}

func (idx *Index) FindBySlug(slug string) *IndexRecord {
	for _, r := range idx.Records {
		if r.Slug == slug {
			return r
		}
	}
	return nil
}

func (idx *Index) FilterByOwner(owner string) []*IndexRecord {
	var result []*IndexRecord
	for _, r := range idx.Records {
		if r.Owner == owner {
			result = append(result, r)
		}
	}
	return result
}

func (idx *Index) FilterByState(state ProjectState) []*IndexRecord {
	var result []*IndexRecord
	for _, r := range idx.Records {
		if r.State == state {
			result = append(result, r)
		}
	}
	return result
}

func (idx *Index) FilterByTag(tag string) []*IndexRecord {
	var result []*IndexRecord
	for _, r := range idx.Records {
		for _, t := range r.Tags {
			if t == tag {
				result = append(result, r)
				break
			}
		}
	}
	return result
}
