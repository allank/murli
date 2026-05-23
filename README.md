# murli 🎶

[![Go Reference](https://pkg.go.dev/badge/github.com/allank/murli.svg)](https://pkg.go.dev/github.com/allank/murli)
[![Go Report Card](https://goreportcard.com/badge/github.com/allank/murli)](https://goreportcard.com/report/github.com/allank/murli)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A pure-Go, zero-dependency middleware for [spf13/cobra](https://github.com/spf13/cobra) that makes CLI tools speak natively to AI agents.

`murli` — named after Krishna's sacred flute in Hindu tradition. The murli's music is said to enchant every listener — each feeling it was meant for them alone.

`murli` the library takes the same approach. Your commands don't change. But a human at a terminal gets clear, readable output, and an agent reading from a pipe gets structured JSON — each feeling the output was shaped for them.

---

## 💡 Core Philosophy

LLM-based agents interact with command-line tools differently than humans. While humans skim, agents tokenize, parse, and plan. `murli` acts as an automated adaptation layer for Cobra, ensuring seamless developer-agent integration.

*   **Mode Decoupling:** Automatic TTY checking. Human terminal users get pretty, formatted output; piped agent processes receive structured, clean JSON.
*   **Self-Documenting CLI:** Dynamically inspects commands, positional arguments, and flag trees to emit detailed schemas via a persistent global `--schema` flag.
*   **Actionable, Structured Errors:** Intercepts routing, validation, and execution errors, wrapping them in JSON envelopes with dedicated exit codes and recovery suggestions to allow single-retry self-correction.
*   **Token Efficiency:** Implements deferred logging that collapses consecutive duplicate log lines and telemetry progress indicators, saving LLM context window space. Telemetry is routed directly to `Stderr`, keeping `Stdout` clean.

---

## 🛠️ Installation

Initialize `murli` in your Go project:

```bash
go get github.com/allank/murli
```

---

## 🚀 Quick Start

Creating an agent-optimized CLI requires only minor additions to your standard Cobra codebase. Use `murli.Execute` as a drop-in replacement for your root command's runner:

```go
package main

import (
	"fmt"

	"github.com/allank/murli"
	"github.com/spf13/cobra"
)

type Result struct {
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

		// Progress messages are sent to Stderr (with deduplication in Agent mode)
		writer.Progress("Searching database index...")
		writer.Progress("Searching database index...") // Deduplicated automatically!

		if queryText == "" {
			return &murli.AgentError{
				Code:        murli.ExitUserError,
				ErrorType:   "empty_query",
				Message:     "Query string cannot be empty",
				Suggestion:  "Provide a conceptual search keyword.",
				Recoverable: true,
			}
		}

		results := []Result{
			{Path: "/docs/woodworking", Score: 0.95},
		}

		// Success routing based on TTY mode
		writer.WriteSuccess(
			fmt.Sprintf("Found %d matching folders", len(results)),
			results,
		)
		return nil
	},
}

func main() {
	var rootCmd = &cobra.Command{Use: "riffle"}
	rootCmd.AddCommand(queryCmd)

	queryCmd.Flags().Int("top", 5, "Maximum results to return")

	// Annotate with LLM parameters
	murli.Annotate(queryCmd, murli.Metadata{
		AgentDescription: "Searches the semantic index for directory conceptual matches.",
		WhenToUse:        "Use when looking for folders matching general topics.",
		Idempotent:       true,
		Returns: &murli.ReturnSchema{
			Type:        "json",
			Description: "Ranked list of vector similarity results",
			Shape: map[string]any{
				"path":  "string",
				"score": "float32",
			},
		},
	})

	// Wrap execution and register --schema / --agent flags
	_ = murli.Execute(rootCmd)
}
```

---

## 📖 Key Features

### 1. Dynamic JSON-RPC Schema (`--schema`)
Running `./yourtool query --schema` prints a highly detailed schema on `Stdout`. Positional argument bounds validation is automatically bypassed when generating schemas to prevent crash loops:

```json
{
  "name": "query",
  "summary": "Semantic query search",
  "when_to_use": "Use when looking for folders matching general topics.",
  "agent_description": "Searches the semantic index for directory conceptual matches.",
  "idempotent": true,
  "arguments": [
    {
      "name": "text",
      "type": "string",
      "required": true,
      "description": ""
    }
  ],
  "flags": [
    {
      "name": "top",
      "type": "int",
      "default": 5,
      "description": "Maximum results to return"
    }
  ],
  "returns": {
    "type": "json",
    "description": "Ranked list of vector similarity results",
    "shape": {
      "path": "string",
      "score": "float32"
    }
  }
}
```

### 2. Output & TTY Decoupling
*   **Human terminal mode** prints plain success/error lines:
    ```bash
    $ ./riffle query woodworking
    Found 1 matching folders
    ```
*   **Piped or Captured agent mode** (or using the `--agent` override) formats the response strictly in JSON:
    ```bash
    $ ./riffle query woodworking | cat
    {
      "status": "ok",
      "result": [
        {
          "path": "/docs/woodworking",
          "score": 0.95
        }
      ]
    }
    ```

### 3. Bulletproof Error Handling
Standard Go errors returned by custom commands, Cobra routing failures (e.g. unknown subcommands), and flag parsing errors are automatically captured and formatted. 
*   **TTY Mode:**
    ```bash
    $ ./riffle query woodworking --top abc
    Error: invalid argument "abc" for "--top" flag: strconv.ParseInt: parsing "abc": invalid syntax
    Hint:  Check command usage with --schema or --help.
    ```
*   **Agent Mode (non-TTY):**
    ```json
    {
      "code": 1,
      "error": "flag_error",
      "message": "invalid argument \"abc\" for \"--top\" flag: strconv.ParseInt: parsing \"abc\": invalid syntax",
      "suggestion": "Check command usage with --schema or --help.",
      "recoverable": true
    }
    ```

### 4. Exit Code Mapping

`murli` standardizes exit codes to tell agents how to handle command failures:

| Exit Code | Constant Name | Meaning | Agent Action |
|---|---|---|---|
| `0` | `ExitOK` | Successful execution | Proceed with next task. |
| `1` | `ExitUserError` | Bad input or argument configuration | Read `suggestion`, fix parameters, and retry. |
| `2` | `ExitToolError` | Environment, network, or filesystem crash | Surface crash to user; do not retry immediately. |
| `3` | `ExitPartial` | Some operations succeeded, some failed | Inspect response list, retry on subset if needed. |

---

## 🧪 Testing

`murli` is backed by a robust and comprehensive automated test suite. Run tests locally:

```bash
go test -v ./...
```

---

## 📄 License

Distributed under the MIT License. See [LICENSE](LICENSE) for details.
