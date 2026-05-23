# murli 🎶

[![Go Reference](https://pkg.go.dev/badge/github.com/allank/murli.svg)](https://pkg.go.dev/github.com/allank/murli)
[![Go Report Card](https://goreportcard.com/badge/github.com/allank/murli)](https://goreportcard.com/report/github.com/allank/murli)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A pure-Go middleware for CLI tools that makes them speak natively to AI agents — with adapters for [spf13/cobra](https://github.com/spf13/cobra), [urfave/cli v2](https://github.com/urfave/cli/tree/main/docs/v2), and [urfave/cli v3](https://github.com/urfave/cli).

`murli` — named after Krishna's sacred flute in Hindu tradition. The murli's music is said to enchant every listener — each feeling it was meant for them alone.

`murli` the library takes the same approach. Your commands don't change. But a human at a terminal gets clear, readable output, and an agent reading from a pipe gets structured JSON — each feeling the output was shaped for them.

---

## 💡 Core Philosophy

LLM-based agents interact with command-line tools differently than humans. While humans skim, agents tokenize, parse, and plan. `murli` acts as an automated adaptation layer, ensuring seamless developer-agent integration.

*   **Mode Decoupling:** Automatic TTY checking. Human terminal users get pretty, formatted output; piped agent processes receive structured, clean JSON.
*   **Self-Documenting CLI:** Dynamically inspects commands, positional arguments, and flag trees to emit detailed schemas via a persistent global `--schema` flag.
*   **Actionable, Structured Errors:** Intercepts routing, validation, and execution errors, wrapping them in JSON envelopes with dedicated exit codes and recovery suggestions to allow single-retry self-correction.
*   **Token Efficiency:** Implements deferred logging that collapses consecutive duplicate log lines and telemetry progress indicators, saving LLM context window space. Telemetry is routed directly to `Stderr`, keeping `Stdout` clean.

---

## 🛠️ Installation

Install the core package plus the adapter for your CLI framework:

```bash
# Core types (Writer, Logger, AgentError, Metadata)
go get github.com/allank/murli

# Pick one adapter:
go get github.com/allank/murli/cobra     # spf13/cobra
go get github.com/allank/murli/cli/v2   # urfave/cli v2
go get github.com/allank/murli/cli/v3   # urfave/cli v3
```

---

## 🚀 Quick Start

### cobra

```go
package main

import (
	"fmt"

	"github.com/allank/murli"
	murliCobra "github.com/allank/murli/cobra"
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
		writer := murliCobra.NewWriter(cmd)

		writer.Progress("Searching database index...")
		writer.Progress("Searching database index...") // Deduplicated automatically

		results := []Result{{Path: "/docs/woodworking", Score: 0.95}}

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

	murliCobra.Annotate(queryCmd, murli.Metadata{
		AgentDescription: "Searches the semantic index for directory conceptual matches.",
		WhenToUse:        "Use when looking for folders matching general topics.",
		Idempotent:       true,
		Returns: &murli.ReturnSchema{
			Type:        "json",
			Description: "Ranked list of vector similarity results",
			Shape:       map[string]any{"path": "string", "score": "float32"},
		},
	})

	_ = murliCobra.Execute(rootCmd)
}
```

### urfave/cli v2

```go
package main

import (
	"fmt"
	"os"

	"github.com/allank/murli"
	murliCLI "github.com/allank/murli/cli/v2"
	"github.com/urfave/cli/v2"
)

func main() {
	queryCmd := &cli.Command{
		Name:  "query",
		Usage: "Semantic query search",
		Flags: []cli.Flag{
			&cli.IntFlag{Name: "top", Value: 5, Usage: "Maximum results to return"},
		},
		Action: func(ctx *cli.Context) error {
			writer := murliCLI.NewWriter(ctx)

			results := []map[string]any{{"path": "/docs/woodworking", "score": 0.95}}

			writer.WriteSuccess(
				fmt.Sprintf("Found %d matching folders", len(results)),
				results,
			)
			return nil
		},
	}

	murliCLI.Annotate(queryCmd, murli.Metadata{
		AgentDescription: "Searches the semantic index for directory conceptual matches.",
		WhenToUse:        "Use when looking for folders matching general topics.",
		Idempotent:       true,
	})

	app := &cli.App{
		Name:     "riffle",
		Commands: []*cli.Command{queryCmd},
	}

	_ = murliCLI.Run(app, os.Args)
}
```

### urfave/cli v3

```go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/allank/murli"
	murliCLI "github.com/allank/murli/cli/v3"
	"github.com/urfave/cli/v3"
)

func main() {
	queryCmd := &cli.Command{
		Name:  "query",
		Usage: "Semantic query search",
		Flags: []cli.Flag{
			&cli.IntFlag{Name: "top", Value: 5, Usage: "Maximum results to return"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			writer := murliCLI.NewWriter(cmd)

			results := []map[string]any{{"path": "/docs/woodworking", "score": 0.95}}

			writer.WriteSuccess(
				fmt.Sprintf("Found %d matching folders", len(results)),
				results,
			)
			return nil
		},
	}

	murliCLI.Annotate(queryCmd, murli.Metadata{
		AgentDescription: "Searches the semantic index for directory conceptual matches.",
		WhenToUse:        "Use when looking for folders matching general topics.",
		Idempotent:       true,
	})

	app := &cli.Command{
		Name:     "riffle",
		Commands: []*cli.Command{queryCmd},
	}

	_ = murliCLI.Run(app, os.Args)
}
```

---

## 📦 Package Structure

| Package | Import path | Use when |
|---|---|---|
| Core | `github.com/allank/murli` | Always — provides `Writer`, `Logger`, `AgentError`, `Metadata`, and schema types |
| cobra adapter | `github.com/allank/murli/cobra` | Your CLI uses [spf13/cobra](https://github.com/spf13/cobra) |
| cli/v2 adapter | `github.com/allank/murli/cli/v2` | Your CLI uses [urfave/cli v2](https://github.com/urfave/cli/tree/main/docs/v2) |
| cli/v3 adapter | `github.com/allank/murli/cli/v3` | Your CLI uses [urfave/cli v3](https://github.com/urfave/cli) |

Each adapter provides the same surface:

| Function | cobra | cli/v2 | cli/v3 |
|---|---|---|---|
| Create writer | `cobra.NewWriter(cmd)` | `cli.NewWriter(ctx)` | `cli.NewWriter(cmd)` |
| Annotate command | `cobra.Annotate(cmd, meta)` | `cli.Annotate(cmd, meta)` | `cli.Annotate(cmd, meta)` |
| Enable + run | `cobra.Execute(rootCmd)` | `cli.Run(app, os.Args)` | `cli.Run(app, os.Args)` |
| Enable only | `cobra.Enable(rootCmd)` | `cli.Wrap(app)` | `cli.Wrap(app)` |
| Emit schema | `cobra.EmitSchema(cmd)` | `cli.EmitSchema(cmd, w)` | `cli.EmitSchema(cmd, w)` |

---

## 📖 Key Features

### 1. Dynamic JSON Schema (`--schema`)
Running `./yourtool query --schema` prints a detailed schema on `Stdout`. Positional argument bounds validation is automatically bypassed when generating schemas:

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
*   **Piped or captured agent mode** (or using the `--agent` override) formats the response strictly in JSON:
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
Standard Go errors, routing failures, and flag parsing errors are automatically captured and formatted.

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

Return your own structured errors with `*murli.AgentError` to supply a `suggestion` and signal recoverability:

```go
return &murli.AgentError{
    Code:        murli.ExitUserError,
    ErrorType:   "empty_query",
    Message:     "Query string cannot be empty",
    Suggestion:  "Provide a conceptual search keyword.",
    Recoverable: true,
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

```bash
go test -v ./...
```

---

## 📄 License

Distributed under the MIT License. See [LICENSE](LICENSE) for details.
