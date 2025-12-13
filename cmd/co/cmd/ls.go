package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/tormodhaugland/co/internal/config"
	"github.com/tormodhaugland/co/internal/model"
)

var (
	lsOwner string
	lsState string
	lsTag   string
)

var lsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List workspaces",
	Long:  `Lists all workspaces with optional filtering by owner, state, or tag.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		idx, err := model.LoadIndex(cfg.IndexPath())
		if err != nil {
			return fmt.Errorf("failed to load index: %w", err)
		}

		records := idx.Records

		if lsOwner != "" {
			records = filterByOwner(records, lsOwner)
		}
		if lsState != "" {
			records = filterByState(records, model.ProjectState(lsState))
		}
		if lsTag != "" {
			records = filterByTag(records, lsTag)
		}

		if jsonOut {
			return outputJSON(records)
		}

		if len(records) == 0 {
			fmt.Println("No workspaces found")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "SLUG\tOWNER\tSTATE\tREPOS\tDIRTY")
		for _, r := range records {
			fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%d\n", r.Slug, r.Owner, r.State, r.RepoCount, r.DirtyRepos)
		}
		w.Flush()

		return nil
	},
}

func filterByOwner(records []*model.IndexRecord, owner string) []*model.IndexRecord {
	var result []*model.IndexRecord
	for _, r := range records {
		if r.Owner == owner {
			result = append(result, r)
		}
	}
	return result
}

func filterByState(records []*model.IndexRecord, state model.ProjectState) []*model.IndexRecord {
	var result []*model.IndexRecord
	for _, r := range records {
		if r.State == state {
			result = append(result, r)
		}
	}
	return result
}

func filterByTag(records []*model.IndexRecord, tag string) []*model.IndexRecord {
	var result []*model.IndexRecord
	for _, r := range records {
		for _, t := range r.Tags {
			if t == tag {
				result = append(result, r)
				break
			}
		}
	}
	return result
}

func outputJSON(records []*model.IndexRecord) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(records)
}

func init() {
	lsCmd.Flags().StringVar(&lsOwner, "owner", "", "filter by owner")
	lsCmd.Flags().StringVar(&lsState, "state", "", "filter by state (active, paused, archived, scratch)")
	lsCmd.Flags().StringVar(&lsTag, "tag", "", "filter by tag")
	rootCmd.AddCommand(lsCmd)
}
