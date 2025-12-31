package partial

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// MergeGitignore merges ignore-style files, preserving existing order and adding new unique lines.
func MergeGitignore(existing, partial []byte) ([]byte, error) {
	existingLines := normalizeLines(existing)
	partialLines := normalizeLines(partial)

	seen := make(map[string]bool, len(existingLines))
	merged := make([]string, 0, len(existingLines)+len(partialLines))

	for _, line := range existingLines {
		key := strings.TrimSpace(line)
		if key == "" {
			merged = append(merged, line)
			continue
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		merged = append(merged, line)
	}

	for _, line := range partialLines {
		key := strings.TrimSpace(line)
		if key == "" {
			continue
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		merged = append(merged, line)
	}

	if len(merged) == 0 {
		return []byte{}, nil
	}

	return []byte(strings.Join(merged, "\n") + "\n"), nil
}

// MergeJSON deep merges two JSON objects. Overlay values take precedence.
func MergeJSON(existing, partial []byte) ([]byte, error) {
	base, err := decodeJSONMap(existing)
	if err != nil {
		return nil, err
	}
	overlay, err := decodeJSONMap(partial)
	if err != nil {
		return nil, err
	}

	merged := deepMerge(base, overlay)
	return json.MarshalIndent(merged, "", "  ")
}

// MergeYAML deep merges two YAML documents (first document only).
func MergeYAML(existing, partial []byte) ([]byte, error) {
	base, err := decodeYAMLMap(existing)
	if err != nil {
		return nil, err
	}
	overlay, err := decodeYAMLMap(partial)
	if err != nil {
		return nil, err
	}

	merged := deepMerge(base, overlay)
	return yaml.Marshal(merged)
}

// MergeFile merges a partial file into an existing file and writes to destPath.
func MergeFile(existingPath, partialPath, destPath string) error {
	existing, err := os.ReadFile(existingPath)
	if err != nil {
		return fmt.Errorf("reading existing file: %w", err)
	}
	partial, err := os.ReadFile(partialPath)
	if err != nil {
		return fmt.Errorf("reading partial file: %w", err)
	}

	var merged []byte
	switch {
	case IsGitignoreFile(destPath):
		merged, err = MergeGitignore(existing, partial)
	case IsJSONFile(destPath):
		merged, err = MergeJSON(existing, partial)
	case IsYAMLFile(destPath):
		merged, err = MergeYAML(existing, partial)
	default:
		return fmt.Errorf("unsupported merge format: %s", filepath.Base(destPath))
	}
	if err != nil {
		return err
	}

	info, err := os.Stat(existingPath)
	if err != nil {
		return fmt.Errorf("stat existing file: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("creating destination directory: %w", err)
	}

	if err := os.WriteFile(destPath, merged, info.Mode()); err != nil {
		return fmt.Errorf("writing merged file: %w", err)
	}

	return nil
}

func normalizeLines(data []byte) []string {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func decodeJSONMap(data []byte) (map[string]interface{}, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return map[string]interface{}{}, nil
	}

	var m map[string]interface{}
	if err := json.Unmarshal(trimmed, &m); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	if m == nil {
		return nil, fmt.Errorf("expected JSON object")
	}
	return m, nil
}

func decodeYAMLMap(data []byte) (map[string]interface{}, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return map[string]interface{}{}, nil
	}

	dec := yaml.NewDecoder(bytes.NewReader(trimmed))
	var m map[string]interface{}
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}
	if m == nil {
		return nil, fmt.Errorf("expected YAML object")
	}
	return m, nil
}

func deepMerge(base, overlay map[string]interface{}) map[string]interface{} {
	merged := make(map[string]interface{}, len(base)+len(overlay))
	for k, v := range base {
		merged[k] = v
	}

	for k, ov := range overlay {
		if bv, ok := merged[k]; ok {
			bm, bok := bv.(map[string]interface{})
			om, ook := ov.(map[string]interface{})
			if bok && ook {
				merged[k] = deepMerge(bm, om)
				continue
			}
		}
		merged[k] = ov
	}

	return merged
}
