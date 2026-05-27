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
*   **Streaming Events:** Goroutine-safe NDJSON event streaming to `Stdout` for long-running operations that produce incremental results.
*   **Mutation Safety:** Commands marked `Mutating: true` are automatically rejected in non-interactive (agent) mode, preventing accidental state changes without human confirmation.

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
		writer.Flush()

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
*   **Piped or captured agent mode** (or using the `--agent` override) formats the response as a JSON envelope with `schema_version` and optional `tool_version`:
    ```bash
    $ ./riffle query woodworking | cat
    {
      "status": "ok",
      "schema_version": "0.2",
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
      "recoverable": true,
      "schema_version": "0.2"
    }
    ```

Return your own structured errors using the convenience constructors or a full `*murli.AgentError`:

```go
// Convenience constructors (v0.2+)
return murli.NewUserError("Query string cannot be empty", "Provide a conceptual search keyword.")
return murli.NewToolError("Database connection failed: timeout after 30s")

// Full control — set extended fields as needed
return &murli.AgentError{
    Code:         murli.ExitNotFound,
    ErrorType:    "index_missing",
    Message:      "Semantic index not found at ~/.riffle/index",
    Suggestion:   "Run `riffle index build` to create the index first.",
    Recoverable:  false,
    DocURL:       "https://example.com/docs/indexing",
}
```

`AgentError` extended fields (all optional):

| Field | Type | Purpose |
|---|---|---|
| `ValidValues` | `[]string` | Enumerable valid inputs when a bad value was supplied |
| `RetryAfterMs` | `int` | Milliseconds to wait before retrying (use with `ExitRateLimited`) |
| `DocURL` | `string` | Link to relevant documentation |
| `Field` | `string` | Name of the specific flag or argument that caused the error |

### 4. Exit Code Mapping

`murli` standardizes exit codes to tell agents how to handle command failures:

**Table-stakes (v0.1+)**

| Exit Code | Constant | Meaning | Agent Action |
|---|---|---|---|
| `0` | `ExitOK` | Successful execution | Proceed with next task. |
| `1` | `ExitUserError` | Bad input or argument configuration | Read `suggestion`, fix parameters, and retry. |
| `2` | `ExitToolError` | Environment, network, or filesystem crash | Surface to user; do not retry immediately. |
| `3` | `ExitPartial` | Some operations succeeded, some failed | Inspect response list, retry on subset if needed. |

**Extended taxonomy (v0.2+)**

| Exit Code | Constant | Meaning | Agent Action |
|---|---|---|---|
| `4` | `ExitTimeout` | Operation timed out | Retry after a delay; the operation may be retryable. |
| `5` | `ExitNotFound` | Requested resource does not exist | Verify the resource exists; do not retry blindly. |
| `6` | `ExitPermission` | Caller lacks permission | Not retryable without an auth or config change. |
| `7` | `ExitConflict` | State conflict (resource already exists, etc.) | Read current state before deciding whether to retry. |
| `8` | `ExitRateLimited` | Rate limit hit | Wait at least `retry_after_ms` milliseconds before retrying. |
| `9` | `ExitCancelled` | Operation cancelled by signal or context | Do not retry unless the parent operation resumes. |

### 5. NDJSON Log Output (v0.2+)

In agent mode, `w.Log()` and `w.Progress()` write newline-delimited JSON to `Stderr`. Consecutive duplicate messages are collapsed into a single entry with a `repeated` count, keeping agent context windows clean.

```bash
$ ./riffle index build | cat 2>logs.ndjson
# logs.ndjson contains:
{"ts":"2026-05-26T10:00:00.123Z","level":"info","msg":"Scanning /docs"}
{"ts":"2026-05-26T10:00:01.456Z","level":"progress","msg":"Indexed 500/2000 files","repeated":4}
{"ts":"2026-05-26T10:00:03.789Z","level":"info","msg":"Build complete"}
```

In TTY mode the same calls produce plain text on `Stderr`, with progress lines overwriting in-place (carriage return).

### 6. Structured Progress Events (v0.2+)

For operations with measurable progress, use `WriteProgress()` instead of `Progress()`:

```go
writer.WriteProgress(murli.ProgressEvent{
    Stage:   "indexing",
    Current: 500,
    Total:   2000,
    Percent: 25.0,
    EtaMs:   6000,
    Message: "Indexing files",
})
```

*   **Agent mode** — minified JSON on one line to `Stderr`:
    ```json
    {"stage":"indexing","current":500,"total":2000,"percent":25,"eta_ms":6000,"message":"Indexing files"}
    ```
*   **TTY mode** — human-readable line with carriage return to overwrite:
    ```
    [indexing] Indexing files (500/2000, 25%)
    ```

All `ProgressEvent` fields are optional — populate what is meaningful for your operation.

### 7. NDJSON Event Streaming (v0.2+)

Use `WriteEvent()` to stream incremental results to `Stdout` as they are produced. This is safe to call concurrently from multiple goroutines.

```go
var wg sync.WaitGroup
for _, file := range files {
    wg.Add(1)
    go func(f string) {
        defer wg.Done()
        result := process(f)
        writer.WriteEvent(result) // goroutine-safe
    }(file)
}
wg.Wait()
// Call WriteSuccess or WriteError only after all WriteEvent calls complete.
writer.WriteSuccess("Processing complete", nil)
```

Each event is written as a single minified JSON line on `Stdout`. `WriteEvent` is a no-op in TTY mode (events are machine-only).

### 8. Mutation Safety (v0.2+)

Mark commands that write, delete, or otherwise change state with `Mutating: true`:

```go
murliCobra.Annotate(deleteCmd, murli.Metadata{
    AgentDescription: "Permanently deletes an index.",
    WhenToUse:        "Use to remove a stale or corrupt index.",
    Mutating:         true,
})
```

When a mutating command runs in non-interactive (agent) mode — i.e. piped output — the adapter automatically rejects it before executing any business logic:

```json
{
  "code": 1,
  "error": "confirmation_required",
  "message": "This command mutates state and requires explicit confirmation.",
  "suggestion": "Mutation requires confirmation. Use a TTY (interactive terminal) to run this command, or wait for --force support in a future release.",
  "recoverable": true,
  "schema_version": "0.2"
}
```

This prevents agents from accidentally deleting or modifying state without human oversight. An interactive bypass (`--force` / `--yes`) is planned for v0.4.

### 9. Version Stamps (v0.2+)

All output envelopes carry `schema_version`. The success envelope also carries `tool_version` when set, so consumers know exactly which version of your tool produced the output.

Set `murli.ToolVersion` in your `main()` using a build-time variable:

```go
// In main.go
var version = "dev" // overridden by -ldflags at build time

func main() {
    murli.ToolVersion = version
    // ...
}
```

```bash
go build -ldflags "-X main.version=1.2.3" -o riffle .
```

Or inject directly into the murli package at build time:

```bash
go build -ldflags "-X github.com/allank/murli.ToolVersion=1.2.3" -o riffle .
```

When set, the success envelope includes `tool_version`:

```json
{
  "status": "ok",
  "schema_version": "0.2",
  "tool_version": "1.2.3",
  "result": [...]
}
```

### 10. Whole-Tool Introspection — `describe` (v0.3+)

The `describe` subcommand is auto-mounted on the root command by `Enable()`/`Wrap()`. It dumps the complete command tree as a single JSON document — zero engineer effort required.

```bash
$ ./riffle describe
{
  "name": "riffle",
  "summary": "Riffle semantic search",
  "schema_version": "0.2",
  "capabilities": {
    "streaming": true,
    "dry_run": false,
    "output_formats": ["json", "ndjson", "yaml", "text"],
    "schema_version": "0.2"
  },
  "conventions": {
    "vocabulary": {
      "get": "preferred verb for read operations (over fetch, info, retrieve)",
      "list": "preferred verb for enumeration (over show-all, ls, enumerate)"
    }
  },
  "commands": [
    {
      "name": "query",
      "summary": "Semantic query search",
      "idempotent": true,
      "flags": [...],
      "returns": {...}
    }
  ]
}
```

Agents can call `describe` once at startup to discover all commands, their metadata, capabilities, and recommended vocabulary — without parsing help text.

### 11. Output Format Routing — `--output` (v0.3+)

`--output` is a persistent flag auto-registered on every command. Supported values:

| Value | Behavior |
|---|---|
| `json` (default in agent mode) | Pretty-printed JSON envelope |
| `ndjson` | Minified single-line JSON envelope |
| `yaml` | YAML-encoded envelope |
| `text` | Plain human-readable text (same as TTY mode) |

```bash
$ ./riffle query woodworking --output yaml
status: ok
schema_version: "0.2"
result:
  - path: /docs/woodworking
    score: 0.95
```

### 12. Protocol Version Negotiation — `--protocol-version` (v0.3+)

`--protocol-version` is a persistent flag that adjusts the envelope schema for older consumers. Valid values: `0.1`, `0.2` (default).

With `--protocol-version=0.1`, all envelopes omit `schema_version` and `tool_version` — useful when connecting murli-powered tools to older agent frameworks that don't expect those fields.

### 13. Rich Flag Contracts — `FlagAnnotation` (v0.3+)

Provide per-flag extended metadata via `Metadata.FlagAnnotations`. These fields appear in `--schema` and `describe` output, giving agents richer signal for parameter construction:

```go
murliCobra.Annotate(queryCmd, murli.Metadata{
    FlagAnnotations: map[string]murli.FlagAnnotation{
        "region": {
            Env:        "AWS_REGION",
            Enum:       []string{"us-east-1", "eu-west-1", "ap-southeast-1"},
            Persistent: true,
        },
        "token": {
            Env:       "RIFFLE_TOKEN",
            Sensitive: true,
        },
        "top": {
            MutuallyExclusiveWith: []string{"all"},
            Pattern:               `^\d+$`,
        },
    },
})
```

Available annotation fields:

| Field | Type | Purpose |
|---|---|---|
| `Env` | `string` | Environment variable that sets this flag |
| `Sensitive` | `bool` | Flag carries secrets; agents should not log its value |
| `Persistent` | `bool` | Flag applies to all subcommands |
| `MutuallyExclusiveWith` | `[]string` | Other flag names that cannot be set at the same time |
| `Enum` | `[]string` | Exhaustive list of valid values |
| `Pattern` | `string` | Regex pattern the value must match |

### 14. Typed Examples (v0.3+)

`Metadata.Examples` is now `[]murli.Example` (changed from `[]string` in v0.2). Each example carries a command string, optional description, and expected exit code:

```go
Examples: []murli.Example{
    {
        Command:     "riffle query woodworking",
        Description: "Find woodworking folders",
    },
    {
        Command:          "riffle query --top 20 art",
        Description:      "Return top 20 art matches",
        ExpectedExitCode: 0,
    },
},
```

`ExpectedExitCode` defaults to `0` (success) and is omitted from JSON output when zero.

### 15. Naming Convention Advisory (v0.3+)

In TTY mode, murli emits advisory warnings to stderr when non-conventional command or flag names are detected:

```
[murli advisory] command "fetch": prefer "get" (conventional vocabulary)
[murli advisory] flag --format: prefer --output (conventional vocabulary)
```

Warnings are informational only — they never block execution and are suppressed entirely in agent mode. The advisory system checks against conventional vocabulary derived from Cloudflare CLI guidelines:

- Commands: `get` over `fetch`/`info`/`retrieve`; `list` over `show-all`/`ls`/`enumerate`; `delete` over `remove`/`rm`; `create` over `add`/`new`/`make`; `update` over `edit`/`modify`/`set`
- Flags: `--force` over `--skip-confirmations`/`--no-confirm`; `--quiet` over `--silent`/`--no-output`; `--dry-run` over `--preview`/`--what-if`; `--output` over `--format`/`--output-format`

### 16. ANSI Stripping in Agent Mode (v0.3+)

ANSI escape codes in log messages (e.g. colour output from upstream libraries) are automatically stripped when writing to `Stderr` in agent mode, keeping NDJSON log entries clean for parsers.

```go
writer.Log("\x1b[32mSuccess\x1b[0m") // TTY: green "Success"; agent: plain "Success" in JSON
```

### 17. Dry-Run Support (v0.4+)

Opt in by setting `DryRunnable: true` in `Annotate()`. Murli auto-registers `--dry-run` on that command. Engineers check `IsDryRun()` at the start of their action and return a plan via `WritePlan()` if true. Non-DryRunnable commands do not receive the flag.

```go
murliCobra.Annotate(deleteCmd, murli.Metadata{
    Mutating:    true,
    Destructive: true,
    DryRunnable: true,
})

deleteCmd.RunE = func(cmd *cobra.Command, args []string) error {
    w := murliCobra.NewWriter(cmd)

    if w.IsDryRun() {
        plan := map[string]any{"would_delete": args[0]}
        w.WritePlan("Would delete "+args[0]+" (dry run — no changes made)", plan)
        return nil
    }

    // real deletion here ...
    w.WriteSuccess("Deleted "+args[0], map[string]any{"id": args[0]})
    return nil
}
```

`WritePlan` outputs `{"status": "plan", "result": <plan>, "schema_version": "0.2"}` in agent mode — same envelope shape as `WriteSuccess` but `"status"` is `"plan"` instead of `"ok"`. In TTY mode it prints `humanText` as plain text.

**Honouring the contract is the engineer's responsibility.** Declaring `DryRunnable: true` signals to agents that `--dry-run` produces a plan rather than executing. If you don't call `IsDryRun()`, the command runs normally.

### 18. --force / --yes Guard Bypass (v0.4+)

Commands marked `Mutating: true` automatically receive `--force` and `--yes` flags. When either is passed, the non-interactive mutation guard is bypassed and the action runs. The two flags are aliases — either one activates the bypass.

```go
murliCobra.Annotate(deleteCmd, murli.Metadata{Mutating: true})
// --force and --yes are now available on deleteCmd; no other changes needed.
```

Engineers who need to suppress their own confirmation prompts can call `w.IsForced()`:

```go
deleteCmd.RunE = func(cmd *cobra.Command, args []string) error {
    w := murliCobra.NewWriter(cmd)
    if !w.IsForced() && someCustomRiskCheck() {
        return murli.NewUserError("high-risk operation", "Pass --force to confirm")
    }
    // proceed ...
    return nil
}
```

`--force` and `--yes` are excluded from `--schema` and `describe` output (infrastructure flags). Agents infer force support from `safety.read_only: false` — if a command is not read-only, force/yes are always available.

### 19. Context Cancellation → Structured Errors (v0.4+)

All three adapters automatically detect context errors in the error-wrapping path:

| Returned error | Exit code | `error_type` | `recoverable` |
|---|---|---|---|
| `context.Canceled` (or wrapped) | 9 (`ExitCancelled`) | `"cancelled"` | `false` |
| `context.DeadlineExceeded` (or wrapped) | 4 (`ExitTimeout`) | `"timeout"` | `true` |

Engineers write standard Go — return `ctx.Err()` or any error that wraps either sentinel via `fmt.Errorf("...: %w", err)`. The adapter detects it automatically.

```go
func (cmd *cobra.Command, args []string) error {
    result, err := doWork(ctx)
    if err != nil {
        return fmt.Errorf("work failed: %w", err) // wraps context.Canceled or DeadlineExceeded
    }
    // ...
}
```

Murli does not register signal handlers. Signal handling, context creation, and shutdown logic are the engineer's responsibility.

### 20. SafetyBlock in Schema Output (v0.4+)

Every command in `--schema` and `describe` output includes a `safety` block assembled from `Metadata`:

```json
{
  "name": "delete",
  "safety": {
    "read_only": false,
    "idempotent": false,
    "destructive": true,
    "dry_run_supported": true
  }
}
```

Set the fields in `Annotate()`:

```go
murliCobra.Annotate(deleteCmd, murli.Metadata{
    Mutating:    true,
    Destructive: true,
    DryRunnable: true,
    Reversible:  false, // default, omitted from JSON
})
```

`read_only` is derived as `!Mutating` at assembly time; you never set it directly. `idempotent`, `destructive`, `reversible`, and `dry_run_supported` use `omitempty` — they are absent from JSON when false.

Agents use the block to reason about risk:
- `destructive: true, reversible: false` → require explicit confirmation or `--force`
- `dry_run_supported: true` → probe with `--dry-run` first
- `read_only: true` → safe to call without confirmation

---

## 🧪 Testing

```bash
go test -race ./...
```

---

## 📄 License

Distributed under the MIT License. See [LICENSE](LICENSE) for details.
