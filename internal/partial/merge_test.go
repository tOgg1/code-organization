package partial

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestMergeGitignore(t *testing.T) {
	existing := []byte("# comment\nnode_modules/\ndist/\n")
	partial := []byte("dist/\ncoverage/\n# comment\n")

	merged, err := MergeGitignore(existing, partial)
	require.NoError(t, err)

	expected := "# comment\nnode_modules/\ndist/\ncoverage/\n"
	assert.Equal(t, expected, string(merged))
}

func TestMergeGitignore_Empty(t *testing.T) {
	merged, err := MergeGitignore([]byte{}, []byte{})
	require.NoError(t, err)
	assert.Equal(t, "", string(merged))
}

func TestMergeFile_GitignoreVariants(t *testing.T) {
	dir := t.TempDir()

	existingPath := filepath.Join(dir, ".dockerignore")
	partialPath := filepath.Join(dir, "partial.dockerignore")

	require.NoError(t, os.WriteFile(existingPath, []byte("node_modules/\n"), 0644))
	require.NoError(t, os.WriteFile(partialPath, []byte("dist/\n"), 0644))

	require.NoError(t, MergeFile(existingPath, partialPath, existingPath))

	merged, err := os.ReadFile(existingPath)
	require.NoError(t, err)
	assert.Equal(t, "node_modules/\ndist/\n", string(merged))
}

func TestMergeJSON(t *testing.T) {
	existing := []byte(`{"a":1,"b":{"c":1,"d":[1]}}`)
	partial := []byte(`{"b":{"c":2,"e":3},"d":[2]}`)

	merged, err := MergeJSON(existing, partial)
	require.NoError(t, err)

	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(merged, &got))

	expected := map[string]interface{}{
		"a": float64(1),
		"b": map[string]interface{}{
			"c": float64(2),
			"d": []interface{}{float64(1)},
			"e": float64(3),
		},
		"d": []interface{}{float64(2)},
	}

	assert.Equal(t, expected, got)
}

func TestMergeJSON_Invalid(t *testing.T) {
	_, err := MergeJSON([]byte("{"), []byte(`{"a":1}`))
	assert.Error(t, err)
}

func TestMergeYAML(t *testing.T) {
	existing := []byte("a: 1\nb:\n  c: 1\n")
	partial := []byte("b:\n  c: 2\n---\nignored: true\n")

	merged, err := MergeYAML(existing, partial)
	require.NoError(t, err)

	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(mustJSON(t, merged), &got))

	expected := map[string]interface{}{
		"a": float64(1),
		"b": map[string]interface{}{
			"c": float64(2),
		},
	}

	assert.Equal(t, expected, got)
}

func TestDeepMerge(t *testing.T) {
	base := map[string]interface{}{
		"a": float64(1),
		"b": map[string]interface{}{
			"c": float64(1),
			"d": float64(2),
		},
	}
	overlay := map[string]interface{}{
		"b": map[string]interface{}{
			"c": float64(9),
		},
		"e": float64(3),
	}

	merged := deepMerge(base, overlay)

	expected := map[string]interface{}{
		"a": float64(1),
		"b": map[string]interface{}{
			"c": float64(9),
			"d": float64(2),
		},
		"e": float64(3),
	}

	assert.Equal(t, expected, merged)
}

func mustJSON(t *testing.T, yamlData []byte) []byte {
	t.Helper()
	var tmp map[string]interface{}
	require.NoError(t, yaml.Unmarshal(yamlData, &tmp))
	jsonData, err := json.Marshal(tmp)
	require.NoError(t, err)
	return jsonData
}
