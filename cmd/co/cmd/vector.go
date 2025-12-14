package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/tormodhaugland/co/internal/config"
	"github.com/tormodhaugland/co/internal/embedder"
	"github.com/tormodhaugland/co/internal/model"
	"github.com/tormodhaugland/co/internal/search"
	"github.com/tormodhaugland/co/internal/vectordb"
)

var (
	// Index flags
	vectorIndexAll     bool
	vectorIndexState   string
	vectorIndexVerbose bool

	// Search flags
	vectorSearchLimit     int
	vectorSearchCodebase  string
	vectorSearchMinScore  float64
	vectorSearchContent   bool
	vectorSearchPathsOnly bool
	vectorSearchFile      string

	// Stats flags
	vectorStatsCodebase string

	// Clear flags
	vectorClearAll bool
)

// vectorCmd is the parent command for vector search operations
var vectorCmd = &cobra.Command{
	Use:   "vector",
	Short: "Semantic code search using vector embeddings",
	Long: `Vector search indexes your codebases using AST-aware chunking and
generates embeddings via Ollama (local, free). This enables semantic code
search - finding similar code patterns and implementations.

Requires Ollama with nomic-embed-text model:
  ollama pull nomic-embed-text

Examples:
  co vector index acme--backend       # Index a specific codebase
  co vector index --all               # Index all active codebases
  co vector search "auth middleware"  # Search for similar code
  co vector stats                     # Show index statistics`,
}

// vectorIndexCmd indexes codebases
var vectorIndexCmd = &cobra.Command{
	Use:   "index [codebase...]",
	Short: "Index codebases for semantic search",
	Long: `Indexes one or more codebases, extracting semantic code chunks
and generating embeddings for vector similarity search.

Uses tree-sitter for AST-aware chunking (functions, classes, methods)
and Ollama's nomic-embed-text for embeddings.

Examples:
  co vector index acme--backend       # Index one codebase
  co vector index acme--api acme--web # Index multiple
  co vector index --all               # Index all active codebases
  co vector index --state=active      # Index by state filter`,
	RunE: runVectorIndex,
}

// vectorSearchCmd performs semantic search
var vectorSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search for similar code",
	Long: `Performs semantic search across indexed codebases.

The query can be:
  - Natural language: "function that validates JWT tokens"
  - Code snippet: "func handleError(err error)"
  - File content: --file=path/to/example.go

Examples:
  co vector search "authentication middleware"
  co vector search "error handling pattern"
  co vector search --codebase=acme--api "database connection"
  co vector search --file=example.go
  co vector search --json "api endpoint"`,
	Args: cobra.MaximumNArgs(1),
	RunE: runVectorSearch,
}

// vectorStatsCmd shows index statistics
var vectorStatsCmd = &cobra.Command{
	Use:   "stats [codebase]",
	Short: "Show vector index statistics",
	Long: `Displays statistics about the vector index including:
  - Total indexed files and code chunks
  - Per-codebase breakdown
  - Language distribution
  - Index freshness`,
	Args: cobra.MaximumNArgs(1),
	RunE: runVectorStats,
}

// vectorClearCmd clears the index
var vectorClearCmd = &cobra.Command{
	Use:   "clear [codebase]",
	Short: "Clear vector index",
	Long: `Removes vector index data for specified codebases.

Examples:
  co vector clear acme--backend  # Clear one codebase
  co vector clear --all          # Clear entire index`,
	Args: cobra.MaximumNArgs(1),
	RunE: runVectorClear,
}

func init() {
	// Index flags
	vectorIndexCmd.Flags().BoolVar(&vectorIndexAll, "all", false, "index all active codebases")
	vectorIndexCmd.Flags().StringVar(&vectorIndexState, "state", "active", "filter codebases by state")
	vectorIndexCmd.Flags().BoolVarP(&vectorIndexVerbose, "verbose", "v", false, "verbose output")

	// Search flags
	vectorSearchCmd.Flags().IntVarP(&vectorSearchLimit, "limit", "n", 10, "maximum results to return")
	vectorSearchCmd.Flags().StringVarP(&vectorSearchCodebase, "codebase", "c", "", "filter to specific codebase")
	vectorSearchCmd.Flags().Float64Var(&vectorSearchMinScore, "min-score", 0.0, "minimum similarity score (0-1, typically 0.1-0.2 for good matches)")
	vectorSearchCmd.Flags().BoolVar(&vectorSearchContent, "content", false, "include code content in output")
	vectorSearchCmd.Flags().BoolVar(&vectorSearchPathsOnly, "paths-only", false, "output only file paths")
	vectorSearchCmd.Flags().StringVarP(&vectorSearchFile, "file", "f", "", "use file content as query")

	// Stats flags
	vectorStatsCmd.Flags().StringVarP(&vectorStatsCodebase, "codebase", "c", "", "show stats for specific codebase")

	// Clear flags
	vectorClearCmd.Flags().BoolVar(&vectorClearAll, "all", false, "clear entire index")

	// Build command tree
	vectorCmd.AddCommand(vectorIndexCmd)
	vectorCmd.AddCommand(vectorSearchCmd)
	vectorCmd.AddCommand(vectorStatsCmd)
	vectorCmd.AddCommand(vectorClearCmd)
	rootCmd.AddCommand(vectorCmd)
}

func runVectorIndex(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Determine which codebases to index
	var codebases []string

	if vectorIndexAll || len(args) == 0 {
		// Load from index
		idx, err := model.LoadIndex(cfg.IndexPath())
		if err != nil {
			return fmt.Errorf("loading index (run 'co index' first): %w", err)
		}

		for _, r := range idx.Records {
			if vectorIndexState == "" || string(r.State) == vectorIndexState {
				codebases = append(codebases, r.Slug)
			}
		}
	} else {
		codebases = args
	}

	if len(codebases) == 0 {
		fmt.Println("No codebases to index")
		return nil
	}

	// Open vector database
	dbPath := filepath.Join(cfg.SystemDir(), "vectors.db")
	db, err := vectordb.Open(dbPath)
	if err != nil {
		return fmt.Errorf("opening vector database: %w", err)
	}
	defer db.Close()

	// Create embedder
	embCfg := embedder.DefaultConfig()
	emb, err := embedder.New(embCfg)
	if err != nil {
		return fmt.Errorf("creating embedder: %w", err)
	}

	// Test Ollama connection
	fmt.Println("Connecting to Ollama...")
	ollamaEmb := emb.(*embedder.OllamaEmbedder)
	if err := ollamaEmb.Ping(context.Background()); err != nil {
		fmt.Println("Ollama not available. Trying to pull model...")
		if pullErr := ollamaEmb.PullModel(context.Background()); pullErr != nil {
			return fmt.Errorf("ollama not available and could not pull model: %w\nEnsure Ollama is running: ollama serve", err)
		}
	}
	fmt.Printf("Using model: %s (%d dimensions)\n\n", emb.ModelName(), emb.Dimension())

	// Create indexer
	indexCfg := search.DefaultIndexConfig()
	indexCfg.Verbose = vectorIndexVerbose
	indexer := search.NewIndexer(db, emb, indexCfg)

	// Index each codebase
	totalStart := time.Now()
	for i, codebase := range codebases {
		fmt.Printf("[%d/%d] Indexing %s...\n", i+1, len(codebases), codebase)

		workspacePath := cfg.WorkspacePath(codebase)
		progress := make(chan search.IndexProgress, 100)

		// Run indexing in goroutine
		errChan := make(chan error, 1)
		go func() {
			errChan <- indexer.IndexCodebase(context.Background(), codebase, workspacePath, progress)
		}()

		// Display progress
		var lastPhase string
		for p := range progress {
			if p.Error != nil && vectorIndexVerbose {
				fmt.Printf("  Warning: %v\n", p.Error)
				continue
			}

			if p.Phase != lastPhase {
				if lastPhase != "" {
					fmt.Println()
				}
				lastPhase = p.Phase
			}

			switch p.Phase {
			case "scanning":
				fmt.Print("  Scanning files...")
			case "chunking":
				fmt.Printf("\r  Chunking: %d/%d files, %d chunks", p.FilesProcessed, p.FilesTotal, p.ChunksTotal)
			case "embedding":
				fmt.Printf("\r  Embedding: %d/%d chunks    ", p.ChunksEmbedded, p.ChunksTotal)
			case "storing":
				fmt.Print("\r  Storing in database...      ")
			case "complete":
				fmt.Printf("\r  ✓ Indexed %d files, %d chunks        \n", p.FilesProcessed, p.ChunksTotal)
			}
		}

		if err := <-errChan; err != nil {
			fmt.Printf("  ✗ Error: %v\n", err)
		}
	}

	duration := time.Since(totalStart)
	fmt.Printf("\nIndexing complete in %v\n", duration.Round(time.Millisecond))
	fmt.Printf("Database: %s\n", dbPath)

	return nil
}

func runVectorSearch(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Determine query
	var query string
	if vectorSearchFile != "" {
		content, err := os.ReadFile(vectorSearchFile)
		if err != nil {
			return fmt.Errorf("reading file: %w", err)
		}
		query = string(content)
	} else if len(args) > 0 {
		query = args[0]
	} else {
		return fmt.Errorf("provide a search query or use --file")
	}

	// Open vector database
	dbPath := filepath.Join(cfg.SystemDir(), "vectors.db")
	db, err := vectordb.Open(dbPath)
	if err != nil {
		return fmt.Errorf("opening vector database: %w", err)
	}
	defer db.Close()

	// Create embedder
	embCfg := embedder.DefaultConfig()
	emb, err := embedder.New(embCfg)
	if err != nil {
		return fmt.Errorf("creating embedder: %w", err)
	}

	// Create searcher
	searcher := search.NewSearcher(db, emb)

	// Perform search
	searchCfg := search.SearchConfig{
		Limit:          vectorSearchLimit,
		Codebase:       vectorSearchCodebase,
		MinScore:       vectorSearchMinScore,
		IncludeContent: vectorSearchContent || !vectorSearchPathsOnly,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results, err := searcher.Search(ctx, query, searchCfg)
	if err != nil {
		return fmt.Errorf("searching: %w", err)
	}

	if len(results) == 0 {
		fmt.Println("No results found")
		return nil
	}

	// Output results
	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(results)
	}

	if vectorSearchPathsOnly {
		for _, r := range results {
			fmt.Printf("%s/%s/repos/%s/%s:%d\n", cfg.CodeRoot, r.Codebase, r.Repo, r.FilePath, r.StartLine)
		}
		return nil
	}

	// Pretty print results
	fmt.Printf("Found %d results:\n\n", len(results))
	for i, r := range results {
		fmt.Printf("%d. %s\n", i+1, search.FormatResult(r, vectorSearchContent, cfg.CodeRoot))
		fmt.Println()
	}

	return nil
}

func runVectorStats(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Open vector database
	dbPath := filepath.Join(cfg.SystemDir(), "vectors.db")
	db, err := vectordb.Open(dbPath)
	if err != nil {
		return fmt.Errorf("opening vector database: %w", err)
	}
	defer db.Close()

	stats, err := db.GetStats()
	if err != nil {
		return fmt.Errorf("getting stats: %w", err)
	}

	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(stats)
	}

	// Filter by codebase if specified
	codebaseFilter := vectorStatsCodebase
	if len(args) > 0 {
		codebaseFilter = args[0]
	}

	fmt.Println("Vector Index Statistics")
	fmt.Println("=======================")
	fmt.Printf("Database: %s\n", dbPath)
	fmt.Printf("Total codebases: %d\n", stats.TotalCodebases)
	fmt.Printf("Total files: %d\n", stats.TotalFiles)
	fmt.Printf("Total chunks: %d\n", stats.TotalChunks)

	if !stats.OldestIndexed.IsZero() {
		fmt.Printf("Oldest indexed: %s\n", stats.OldestIndexed.Format("2006-01-02 15:04"))
		fmt.Printf("Newest indexed: %s\n", stats.NewestIndexed.Format("2006-01-02 15:04"))
	}
	fmt.Println()

	if len(stats.CodebaseStats) > 0 {
		fmt.Println("Per-Codebase Breakdown:")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "CODEBASE\tFILES\tCHUNKS\tREPOS\tLANGUAGES\tLAST INDEXED")

		for _, cs := range stats.CodebaseStats {
			if codebaseFilter != "" && cs.Codebase != codebaseFilter {
				continue
			}

			languages := "-"
			if len(cs.Languages) > 0 {
				languages = strings.Join(cs.Languages, ", ")
			}

			lastIndexed := "-"
			if !cs.LastIndexed.IsZero() {
				lastIndexed = cs.LastIndexed.Format("2006-01-02 15:04")
			}

			fmt.Fprintf(w, "%s\t%d\t%d\t%d\t%s\t%s\n",
				cs.Codebase, cs.FileCount, cs.ChunkCount, cs.RepoCount, languages, lastIndexed)
		}
		w.Flush()
	}

	return nil
}

func runVectorClear(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	dbPath := filepath.Join(cfg.SystemDir(), "vectors.db")

	if vectorClearAll {
		// Delete entire database file
		if err := os.Remove(dbPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("removing database: %w", err)
		}
		fmt.Println("Vector index cleared completely")
		return nil
	}

	if len(args) == 0 {
		return fmt.Errorf("specify a codebase or use --all")
	}

	codebase := args[0]

	// Open database and delete codebase
	db, err := vectordb.Open(dbPath)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	if err := db.DeleteCodebase(codebase); err != nil {
		return fmt.Errorf("deleting codebase: %w", err)
	}

	fmt.Printf("Cleared index for %s\n", codebase)
	return nil
}
