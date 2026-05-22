package main

import (
	"fmt"

	"github.com/allank/murli"
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
		writer := murli.NewWriter(cmd)
		queryText := args[0]

		// Log progress to demonstrate token-efficient collapsing
		writer.Progress("Searching similarity index...")
		writer.Progress("Searching similarity index...") // Repeated
		writer.Progress("Searching similarity index...") // Repeated
		writer.Progress("Reranking search candidates...")
		writer.Progress("Reranking search candidates...") // Repeated

		// Trigger various error scenarios for manual testing
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

		// Emit output dynamically
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

	// Configure flags
	queryCmd.Flags().Int("top", 5, "Maximum number of results to return")
	queryCmd.Flags().String("index", "", "Path to index root (auto-discovered if empty)")

	// Annotate leaf command
	murli.Annotate(queryCmd, murli.Metadata{
		AgentDescription: "Searches vector database for semantic directory matches. Performs cosine similarity searches across the directory summary index.",
		WhenToUse:        "Use when you need to locate folders containing specific conceptual topics. Do NOT use if you need precise word-matching.",
		Idempotent:       true,
		Returns: &murli.ReturnSchema{
			Type:        "json",
			Description: "Cosine similarity results ranked by score",
			Shape: map[string]any{
				"path":  "string",
				"score": "float32",
			},
		},
		Examples: []string{
			"riffle query 'rust database drivers'",
			"riffle query 'woodworking projects' --top 3",
		},
	})

	// Execute using murli
	_ = murli.Execute(rootCmd)
}
