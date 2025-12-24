package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/tormodhaugland/co/internal/config"
	"github.com/tormodhaugland/co/internal/fs"
	"github.com/tormodhaugland/co/internal/model"
)

var (
	tmpCleanDryRun bool
)

var tmpCmd = &cobra.Command{
	Use:   "tmp <name>",
	Short: "Create a temporary workspace",
	Long: `Creates a temporary workspace in ~/Code with the prefix "tmp--".

Temporary workspaces are lightweight workspaces for quick experiments,
spikes, or throwaway work. They exist at the same level as regular
workspaces but are marked with the "tmp--" prefix.

Examples:
  co tmp experiment     # Creates ~/Code/tmp--experiment
  co tmp test-idea      # Creates ~/Code/tmp--test-idea

Subcommands:
  co tmp ls             # List all temporary workspaces
  co tmp clean          # Remove tmp workspaces inactive for 30+ days
  co tmp rm <name>      # Remove a specific tmp workspace`,
	Args: cobra.ExactArgs(1),
	RunE: runTmpCreate,
}

var tmpLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List temporary workspaces",
	Long:  `Lists all temporary workspaces with their age and last activity.`,
	Args:  cobra.NoArgs,
	RunE:  runTmpList,
}

var tmpCleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove stale temporary workspaces",
	Long: `Removes temporary workspaces that have been inactive for longer
than the configured threshold (default: 30 days).

Use --dry-run to preview what would be removed without deleting.`,
	Args: cobra.NoArgs,
	RunE: runTmpClean,
}

var tmpRmCmd = &cobra.Command{
	Use:   "rm <name>",
	Short: "Remove a temporary workspace",
	Long: `Removes a specific temporary workspace.

The name should be without the "tmp--" prefix.

Example:
  co tmp rm experiment  # Removes ~/Code/tmp--experiment`,
	Args: cobra.ExactArgs(1),
	RunE: runTmpRemove,
}

func init() {
	rootCmd.AddCommand(tmpCmd)
	tmpCmd.AddCommand(tmpLsCmd)
	tmpCmd.AddCommand(tmpCleanCmd)
	tmpCmd.AddCommand(tmpRmCmd)

	tmpCleanCmd.Flags().BoolVar(&tmpCleanDryRun, "dry-run", false, "Preview what would be removed without deleting")
}

func runTmpCreate(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	name := strings.ToLower(args[0])

	// Validate name: alphanumeric and hyphens only
	if !isValidTmpName(name) {
		return fmt.Errorf("invalid tmp name: %s (must be lowercase alphanumeric with hyphens)", name)
	}

	slug := "tmp--" + name
	workspacePath := filepath.Join(cfg.CodeRoot, slug)

	if fs.WorkspaceExists(cfg.CodeRoot, slug) {
		return fmt.Errorf("tmp workspace already exists: %s", slug)
	}

	// Create workspace directory with repos subdirectory
	if err := os.MkdirAll(filepath.Join(workspacePath, "repos"), 0755); err != nil {
		return fmt.Errorf("failed to create workspace: %w", err)
	}

	// Create a minimal project.json
	proj := model.NewProject("tmp", name)
	proj.State = model.StateTmp
	proj.Slug = slug

	if err := proj.Save(workspacePath); err != nil {
		return fmt.Errorf("failed to save project.json: %w", err)
	}

	if jsonOut {
		result := map[string]interface{}{
			"slug": slug,
			"path": workspacePath,
			"name": name,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	fmt.Printf("Created tmp workspace: %s\n", workspacePath)
	return nil
}

func runTmpList(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	workspaces, err := fs.ListTmpWorkspaces(cfg.CodeRoot)
	if err != nil {
		return fmt.Errorf("failed to list tmp workspaces: %w", err)
	}

	if len(workspaces) == 0 {
		fmt.Println("No temporary workspaces found")
		return nil
	}

	type tmpInfo struct {
		Slug       string `json:"slug"`
		Name       string `json:"name"`
		Path       string `json:"path"`
		Created    string `json:"created,omitempty"`
		LastActive string `json:"last_active"`
		Age        string `json:"age"`
		AgeDays    int    `json:"age_days"`
	}

	var infos []tmpInfo
	now := time.Now()

	for _, slug := range workspaces {
		workspacePath := filepath.Join(cfg.CodeRoot, slug)
		name := strings.TrimPrefix(slug, "tmp--")

		info := tmpInfo{
			Slug: slug,
			Name: name,
			Path: workspacePath,
		}

		// Get last modification time
		lastMod, err := fs.GetLastModTime(workspacePath)
		if err == nil && lastMod > 0 {
			modTime := time.Unix(lastMod, 0)
			info.LastActive = modTime.Format("2006-01-02")
			days := int(now.Sub(modTime).Hours() / 24)
			info.AgeDays = days
			info.Age = formatAge(days)
		}

		// Try to load project.json for creation date
		proj, err := model.LoadProject(filepath.Join(workspacePath, "project.json"))
		if err == nil {
			info.Created = proj.Created
		}

		infos = append(infos, info)
	}

	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(infos)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tLAST ACTIVE\tAGE")
	for _, info := range infos {
		fmt.Fprintf(w, "%s\t%s\t%s\n", info.Name, info.LastActive, info.Age)
	}
	w.Flush()

	return nil
}

func runTmpClean(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	threshold := cfg.GetTmpConfig().CleanupDays
	workspaces, err := fs.ListTmpWorkspaces(cfg.CodeRoot)
	if err != nil {
		return fmt.Errorf("failed to list tmp workspaces: %w", err)
	}

	now := time.Now()
	var stale []string

	for _, slug := range workspaces {
		workspacePath := filepath.Join(cfg.CodeRoot, slug)
		lastMod, err := fs.GetLastModTime(workspacePath)
		if err != nil {
			continue
		}

		modTime := time.Unix(lastMod, 0)
		days := int(now.Sub(modTime).Hours() / 24)

		if days >= threshold {
			stale = append(stale, slug)
		}
	}

	if len(stale) == 0 {
		fmt.Printf("No tmp workspaces older than %d days\n", threshold)
		return nil
	}

	if tmpCleanDryRun {
		fmt.Printf("Would remove %d tmp workspace(s) (inactive for %d+ days):\n", len(stale), threshold)
		for _, slug := range stale {
			name := strings.TrimPrefix(slug, "tmp--")
			fmt.Printf("  %s\n", name)
		}
		return nil
	}

	if jsonOut {
		result := map[string]interface{}{
			"removed":        stale,
			"count":          len(stale),
			"threshold_days": threshold,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	fmt.Printf("Removing %d tmp workspace(s) inactive for %d+ days:\n", len(stale), threshold)
	for _, slug := range stale {
		workspacePath := filepath.Join(cfg.CodeRoot, slug)
		name := strings.TrimPrefix(slug, "tmp--")

		if err := os.RemoveAll(workspacePath); err != nil {
			fmt.Fprintf(os.Stderr, "  Warning: failed to remove %s: %v\n", name, err)
			continue
		}
		fmt.Printf("  Removed: %s\n", name)
	}

	return nil
}

func runTmpRemove(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	name := strings.ToLower(args[0])
	// Allow both "name" and "tmp--name" forms
	if strings.HasPrefix(name, "tmp--") {
		name = strings.TrimPrefix(name, "tmp--")
	}

	slug := "tmp--" + name
	workspacePath := filepath.Join(cfg.CodeRoot, slug)

	if !fs.WorkspaceExists(cfg.CodeRoot, slug) {
		return fmt.Errorf("tmp workspace does not exist: %s", name)
	}

	if err := os.RemoveAll(workspacePath); err != nil {
		return fmt.Errorf("failed to remove workspace: %w", err)
	}

	if jsonOut {
		result := map[string]interface{}{
			"removed": slug,
			"path":    workspacePath,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	fmt.Printf("Removed tmp workspace: %s\n", name)
	return nil
}

func isValidTmpName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
			return false
		}
	}
	// Don't allow starting or ending with hyphen
	if name[0] == '-' || name[len(name)-1] == '-' {
		return false
	}
	return true
}

func formatAge(days int) string {
	if days == 0 {
		return "today"
	}
	if days == 1 {
		return "1 day"
	}
	if days < 7 {
		return fmt.Sprintf("%d days", days)
	}
	if days < 30 {
		weeks := days / 7
		if weeks == 1 {
			return "1 week"
		}
		return fmt.Sprintf("%d weeks", weeks)
	}
	months := days / 30
	if months == 1 {
		return "1 month"
	}
	return fmt.Sprintf("%d months", months)
}
