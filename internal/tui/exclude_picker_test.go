package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadChildrenSkipsExcludedDir(t *testing.T) {
	tmp := t.TempDir()
	skipDir := filepath.Join(tmp, "skip")
	if err := os.Mkdir(skipDir, 0o755); err != nil {
		t.Fatalf("mkdir skip: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skipDir, "nested.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write nested: %v", err)
	}

	m := newExcludePickerModel(tmp, []string{"skip/"})
	var skipNode *fileNode
	for _, n := range m.root.Children {
		if n.Name == "skip" {
			skipNode = n
			break
		}
	}
	if skipNode == nil {
		t.Fatalf("skip node not found")
	}
	if !skipNode.IsExcluded {
		t.Fatalf("expected skip to be excluded by default")
	}

	// Expanding an excluded node should not traverse it.
	m.loadChildren(skipNode)
	if len(skipNode.Children) != 0 {
		t.Fatalf("expected no children to be loaded for excluded dir, got %d", len(skipNode.Children))
	}
}

func TestLoadChildrenMarksSymlinkAndTruncates(t *testing.T) {
	origMax := maxDirEntries
	maxDirEntries = 10
	defer func() { maxDirEntries = origMax }()

	tmp := t.TempDir()
	if err := os.Mkdir(filepath.Join(tmp, "a"), 0o755); err != nil {
		t.Fatalf("mkdir a: %v", err)
	}
	if err := os.Symlink(filepath.Join(tmp, "a"), filepath.Join(tmp, "alink")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	for i := 0; i < 12; i++ {
		if err := os.WriteFile(filepath.Join(tmp, "file"+fmt.Sprint(i)), []byte("x"), 0o644); err != nil {
			t.Fatalf("write file%d: %v", i, err)
		}
	}

	m := newExcludePickerModel(tmp, nil)

	var symlinkNode *fileNode
	for _, n := range m.root.Children {
		if n.Name == "alink" {
			symlinkNode = n
			break
		}
	}
	if symlinkNode == nil {
		t.Fatalf("symlink node not found")
	}
	if symlinkNode.IsDir {
		t.Fatalf("symlink should not be treated as dir")
	}
	if !symlinkNode.IsSymlink {
		t.Fatalf("symlink flag not set")
	}

	origMax = maxDirEntries
	maxDirEntries = 2
	defer func() { maxDirEntries = origMax }()

	m2 := newExcludePickerModel(tmp, nil)
	if len(m2.root.Children) == 0 {
		t.Fatalf("expected children on root")
	}
	last := m2.root.Children[len(m2.root.Children)-1]
	if !last.IsPlaceholder {
		t.Fatalf("expected last node to be placeholder when truncated")
	}
}
