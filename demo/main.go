package main

import (
	"fmt"

	"github.com/allank/murli"
	murliCobra "github.com/allank/murli/cobra"
	"github.com/spf13/cobra"
)

type QueryResult struct {
	Path  string  `json:"path"`
	Score float32 `json:"score"`
}

var queryCmd = &cobra.Command{
	Use:   "query <text>",
	Short: "Semantic query search",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		writer := murliCobra.NewWriter(cmd)
		queryText := args[0]

		writer.Progress("Searching similarity index...")
		writer.Progress("Searching similarity index...")
		writer.Progress("Searching similarity index...")
		writer.Progress("Reranking search candidates...")
		writer.Progress("Reranking search candidates...")

		if queryText == "error" {
			return fmt.Errorf("simulated backend storage timeout")
		}

		if queryText == "agent-error" {
			return &murli.AgentError{
				Code:        murli.ExitUserError,
				ErrorType:   "invalid_query",
				Message:     "The query contains illegal symbols",
				Suggestion:  "Try searching without punctuation characters.",
				Recoverable: true,
			}
		}

		results := []QueryResult{
			{Path: "/users/allank/projects/woodwork", Score: 0.94},
			{Path: "/users/allank/docs/cabinetry", Score: 0.81},
		}

		writer.WriteSuccess(
			fmt.Sprintf("Found %d matching folders", len(results)),
			results,
		)
		return nil
	},
}

var rootCmd = &cobra.Command{
	Use:   "riffle",
	Short: "Riffle is a tool for taming directories",
}

func main() {
	rootCmd.AddCommand(queryCmd)

	queryCmd.Flags().Int("top", 5, "Maximum number of results to return")
	queryCmd.Flags().String("index", "", "Path to index root (auto-discovered if empty)")

	murliCobra.Annotate(queryCmd, murli.Metadata{
		AgentDescription: "Searches vector database for semantic directory matches.",
		WhenToUse:        "Use when you need to locate folders containing specific conceptual topics.",
		Idempotent:       true,
		Returns: &murli.ReturnSchema{
			Type:        "json",
			Description: "Cosine similarity results ranked by score",
			Shape:       map[string]any{"path": "string", "score": "float32"},
		},
		Examples: []murli.Example{
			{Command: "riffle query 'rust database drivers'"},
			{Command: "riffle query 'woodworking projects' --top 3"},
		},
	})

	_ = murliCobra.Execute(rootCmd)
}
