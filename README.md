# murli 🎶

[![Go Reference](https://pkg.go.dev/badge/github.com/allank/murli.svg)](https://pkg.go.dev/github.com/allank/murli)
[![Go Report Card](https://goreportcard.com/badge/github.com/allank/murli)](https://goreportcard.com/report/github.com/allank/murli)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A pure-Go middleware for CLI tools that makes them speak natively to AI agents — with adapters for [spf13/cobra](https://github.com/spf13/cobra), [urfave/cli v2](https://github.com/urfave/cli/tree/main/docs/v2), and [urfave/cli v3](https://github.com/urfave/cli).

`murli` is named after Krishna's sacred flute. The murli's music enchants every listener — each feeling it was meant for them alone. This library takes the same approach: your commands don't change, but a human at a terminal gets clear readable output, and an agent reading from a pipe gets structured JSON. Each audience gets the experience shaped for them.

---

## Philosophy

Five principles guide everything murli does:

**One tool, two audiences.** Humans and agents call the same commands. murli routes output automatically — no `if agent { ... }` branches in your code.

**Discoverability is a first-class feature.** Agents shouldn't need documentation to use your tool. The tool describes itself.

**Errors are instructions, not messages.** A structured error tells an agent what went wrong, whether to retry, and what to do instead.

**Dangerous operations require explicit intent.** Mutations are rejected in non-interactive mode until the agent — or human — confirms they know what they're doing.

**Context windows are finite.** Log deduplication, clean stderr routing, and streaming results keep agent context consumption predictable.

---

## Installation

```bash
# Core types (Writer, Logger, AgentError, Metadata)
go get github.com/allank/murli

# Pick one adapter for your CLI framework:
go get github.com/allank/murli/cobra     # spf13/cobra
go get github.com/allank/murli/cli/v2   # urfave/cli v2
go get github.com/allank/murli/cli/v3   # urfave/cli v3
```

---

## Quick Start

The minimal change is one line at startup:

```go
// cobra
_ = murliCobra.Execute(rootCmd)   // replaces rootCmd.Execute()

// urfave/cli v2 or v3
_ = murliCLI.Run(app, os.Args)   // replaces app.Run(os.Args)
```

That single change gives your tool structured JSON output, `--schema` on every command, a `describe` subcommand, a `doctor` subcommand, a `profile` subcommand group, and automatic error handling. Everything below layers on top.

---

## Capabilities

### Dual-Audience Output

**What it is:** The same command produces plain text for humans and structured JSON for agents. The switch is automatic — murli checks whether stdout is a terminal.

**Principle:** One tool, two audiences.

---

**Zero effort — TTY detection is automatic.**

Run your command normally: human output. Pipe it: JSON.

```bash
$ ./riffle query woodworking
Found 3 matching folders

$ ./riffle query woodworking | cat
{
  "status": "ok",
  "schema_version": "1.0",
  "result": [...]
}
```

Use `--agent` to force JSON mode without piping (useful in scripts):

```bash
$ ./riffle query woodworking --agent
```

---

**Write your output once — murli routes it.**

Call `WriteSuccess` and `WriteError` in your command handlers. murli renders them appropriately for the audience.

```go
RunE: func(cmd *cobra.Command, args []string) error {
    w := murliCobra.NewWriter(cmd)

    results, err := search(args[0])
    if err != nil {
        return murli.NewToolError("search failed: " + err.Error())
    }

    w.WriteSuccess(
        fmt.Sprintf("Found %d results", len(results)), // human text
        results,                                         // agent payload
    )
    return nil
},
```

---

**Optional — stamp your tool version on every envelope.**

Set `murli.ToolVersion` once at startup (typically via a build-time ldflags variable) and every success envelope carries it:

```go
func main() {
    murli.ToolVersion = version // set via -ldflags "-X main.version=1.2.3"
    _ = murliCobra.Execute(rootCmd)
}
```

```json
{ "status": "ok", "schema_version": "1.0", "tool_version": "1.2.3", "result": [...] }
```

---

**Output format — `--output`.**

`--output` is auto-registered on every command. Pass it to control serialisation:

| Value | Behaviour |
|---|---|
| `json` | Pretty-printed JSON (default in agent mode) |
| `ndjson` | Minified single-line JSON |
| `text` | Plain text (same as TTY mode) |

```bash
$ ./riffle query woodworking --output ndjson
{"status":"ok","schema_version":"1.0","result":[...]}
```

---

### Command Introspection

**What it is:** Agents can discover everything about your tool — commands, flags, capabilities, and health — without reading documentation. murli auto-mounts three subcommands that do this work.

**Principle:** Discoverability is a first-class feature.

---

**Zero effort — `describe` is auto-mounted.**

`describe` dumps your complete command tree as a single JSON document. Agents call it once at startup to understand the full tool.

```bash
$ ./riffle describe
{
  "name": "riffle",
  "summary": "Riffle semantic search",
  "schema_version": "1.0",
  "capabilities": {
    "streaming": true,
    "output_formats": ["json", "ndjson", "text"]
  },
  "commands": [
    {
      "name": "query",
      "summary": "Semantic query search",
      "idempotent": true,
      "flags": [...]
    }
  ]
}
```

---

**Zero effort — `--schema` is auto-registered on every command.**

`--schema` on any command prints the full schema for that command — flags, arguments, return shape, safety block, and all metadata.

```bash
$ ./riffle query --schema
{
  "name": "query",
  "agent_description": "Searches the semantic index for directory conceptual matches.",
  "idempotent": true,
  "arguments": [{ "name": "text", "type": "string", "required": true }],
  "flags": [{ "name": "top", "type": "int", "default": 5 }],
  "safety": { "read_only": true }
}
```

Positional argument validation is automatically bypassed when generating schemas, so `--schema` never fails due to missing args.

---

**Zero effort — `doctor` is auto-mounted.**

`doctor` runs built-in self-checks and reports whether your murli integration is correctly configured. Agents can call it to verify the tool before use.

```bash
$ ./riffle doctor --agent
{
  "status": "ok",
  "result": {
    "checks": [
      { "name": "schema_version", "status": "pass" },
      { "name": "output_formats", "status": "pass" },
      { "name": "command_metadata", "status": "warn", "message": "commands missing description: [index]" }
    ],
    "passed": 2, "warnings": 1, "failed": 0
  }
}
```

`status` is `"ok"` when all checks pass or only warnings exist. `status` is `"plan"` when any check fails — signalling the tool needs attention before use.

---

**One flag — generate an AGENTS.md stub.**

Pass `--agents-md` to `describe` to generate a Markdown file ready to drop into your repository. Agents reading your repo get immediate tool context without running the binary.

```bash
$ ./riffle describe --agents-md > AGENTS.md
```

```markdown
# AGENTS.md

> Auto-generated from `riffle describe`. Edit to add project context.

## Tool: riffle

Riffle semantic search

## Introspection

```bash
riffle describe        # full JSON schema
riffle --help          # human-readable help
```

## Commands

### query

Searches the semantic index for directory conceptual matches.

```bash
riffle query --schema    # JSON schema for this command
```
```

---

### Rich Agent Metadata

**What it is:** Annotations layered onto commands and flags that give agents richer signal — what a command does, when to use it, which flags are sensitive or enumerable, and worked examples. None of it is required; add as much or as little as your tool needs.

**Principle:** Discoverability is a first-class feature.

---

**Zero effort — commands work unannotated.**

murli emits whatever it can infer from your command definitions automatically (name, usage string, flags, argument bounds). Annotation extends that, not replaces it.

---

**One call — annotate a command.**

```go
murliCobra.Annotate(queryCmd, murli.Metadata{
    AgentDescription: "Searches the semantic index for directory conceptual matches.",
    WhenToUse:        "Use when looking for folders matching general topics.",
    Idempotent:       true,
    Returns: &murli.ReturnSchema{
        Type:        "json",
        Description: "Ranked list of vector similarity results",
        Shape:       map[string]any{"path": "string", "score": "float32"},
    },
    Examples: []murli.Example{
        {Command: "riffle query woodworking", Description: "Find woodworking folders"},
        {Command: "riffle query --top 20 art", Description: "Return top 20 art matches"},
    },
})
```

All fields are optional. Set what is meaningful.

---

**Add code — annotate individual flags.**

`FlagAnnotations` adds per-flag metadata to `--schema` and `describe` output, giving agents richer signal for parameter construction:

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
            Sensitive: true,  // agents must not log this value
        },
        "top": {
            MutuallyExclusiveWith: []string{"all"},
            Pattern:               `^\d+$`,
        },
    },
})
```

| Field | Type | Purpose |
|---|---|---|
| `Env` | `string` | Environment variable that sets this flag |
| `Sensitive` | `bool` | Flag carries secrets; agents must not log its value |
| `Persistent` | `bool` | Flag applies to all subcommands |
| `Enum` | `[]string` | Exhaustive list of valid values |
| `Pattern` | `string` | Regex the value must match |
| `MutuallyExclusiveWith` | `[]string` | Other flags that cannot be set simultaneously |
| `Profileable` | `bool` | Flag can be saved in a profile (see [Saved Profiles](#saved-profiles)) |

---

**Naming convention advisory.**

In TTY mode murli emits advisory warnings to stderr when command or flag names deviate from conventional vocabulary:

```
[murli advisory] command "fetch": prefer "get" (conventional vocabulary)
[murli advisory] flag --format: prefer --output (conventional vocabulary)
```

Warnings are informational only — they never block execution and are suppressed entirely in agent mode. Common advisories: `get` over `fetch`; `list` over `ls`; `delete` over `remove`; `--output` over `--format`.

---

### Structured Errors

**What it is:** Every error — whether from flag parsing, routing, or your own handler — is intercepted and wrapped into a consistent JSON envelope with an exit code, error type, recovery suggestion, and retryability signal. Agents can act on errors without parsing message strings.

**Principle:** Errors are instructions, not messages.

---

**Zero effort — flag and routing errors are auto-wrapped.**

murli intercepts errors from the CLI framework before your code runs:

```bash
$ ./riffle query woodworking --top abc | cat
{
  "code": 1,
  "error": "flag_error",
  "message": "invalid argument \"abc\" for \"--top\" flag",
  "suggestion": "Check command usage with --schema or --help.",
  "recoverable": true,
  "schema_version": "1.0"
}
```

In TTY mode the same error prints as readable text.

---

**Return structured errors from your handlers.**

Use the convenience constructors for common cases:

```go
return murli.NewUserError("query string cannot be empty", "Provide a search keyword.")
return murli.NewToolError("database connection failed: timeout after 30s")
```

Or build the full `AgentError` when you need precise control:

```go
return &murli.AgentError{
    Code:        murli.ExitNotFound,
    ErrorType:   "index_missing",
    Message:     "Semantic index not found at ~/.riffle/index",
    Suggestion:  "Run `riffle index build` to create the index first.",
    Recoverable: false,
    DocURL:      "https://example.com/docs/indexing",
}
```

Extended fields (all optional):

| Field | Type | Purpose |
|---|---|---|
| `ValidValues` | `[]string` | Enumerable valid inputs when a bad value was supplied |
| `RetryAfterMs` | `int` | Milliseconds to wait before retrying (use with `ExitRateLimited`) |
| `DocURL` | `string` | Link to relevant documentation |
| `Field` | `string` | Name of the specific flag or argument that caused the error |

---

**Exit code taxonomy.**

murli standardises exit codes so agents know how to respond to any failure:

| Code | Constant | Meaning | Agent action |
|---|---|---|---|
| `0` | `ExitOK` | Success | Proceed |
| `1` | `ExitUserError` | Bad input or configuration | Read `suggestion`, fix parameters, retry |
| `2` | `ExitToolError` | Environment, network, or filesystem failure | Surface to user; do not retry immediately |
| `3` | `ExitPartial` | Some operations succeeded, some failed | Inspect response, retry on subset if appropriate |
| `4` | `ExitTimeout` | Operation timed out | Retry after a delay |
| `5` | `ExitNotFound` | Requested resource does not exist | Verify resource; do not retry blindly |
| `6` | `ExitPermission` | Caller lacks permission | Not retryable without an auth or config change |
| `7` | `ExitConflict` | State conflict (resource already exists, etc.) | Read current state before deciding to retry |
| `8` | `ExitRateLimited` | Rate limit hit | Wait at least `retry_after_ms` milliseconds |
| `9` | `ExitCancelled` | Operation cancelled by signal or context | Do not retry unless the parent operation resumes |

---

**Context cancellation is handled automatically.**

Return `ctx.Err()` (or any error wrapping it) and murli maps it to the correct exit code and error type — no extra code required:

```go
result, err := doWork(ctx)
if err != nil {
    return fmt.Errorf("work failed: %w", err) // wraps context.Canceled or DeadlineExceeded
}
```

| Returned error | Exit code | `error_type` | `recoverable` |
|---|---|---|---|
| `context.Canceled` | `9` (`ExitCancelled`) | `"cancelled"` | `false` |
| `context.DeadlineExceeded` | `4` (`ExitTimeout`) | `"timeout"` | `true` |

---

### Safety Rails

**What it is:** Commands that mutate state are automatically guarded in non-interactive mode. Agents cannot accidentally trigger destructive operations — they must explicitly signal intent. Dry-run support lets agents preview before executing.

**Principle:** Dangerous operations require explicit intent.

---

**Mark a command mutating — the guard is automatic.**

```go
murliCobra.Annotate(deleteCmd, murli.Metadata{
    Mutating:    true,
    Destructive: true,
})
```

When a mutating command runs in agent mode (non-TTY, no `--force`), murli rejects it before your handler runs:

```json
{
  "code": 1,
  "error": "confirmation_required",
  "message": "This command mutates state and requires explicit confirmation.",
  "suggestion": "Pass --force or --yes to proceed without a TTY.",
  "recoverable": true
}
```

`--force` and `--yes` are auto-registered on mutating commands. Either bypasses the guard.

---

**The safety block appears automatically in schema output.**

Every annotated command carries a `safety` block in `--schema` and `describe` output. Agents use it to reason about risk before calling:

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

`read_only` is derived from `!Mutating` automatically. Fields are omitted from JSON when false.

---

**Add code — support dry-run.**

Mark `DryRunnable: true` and handle the flag in your handler. murli auto-registers `--dry-run`:

```go
murliCobra.Annotate(deleteCmd, murli.Metadata{
    Mutating:    true,
    DryRunnable: true,
})

deleteCmd.RunE = func(cmd *cobra.Command, args []string) error {
    w := murliCobra.NewWriter(cmd)

    if w.IsDryRun() {
        w.WritePlan("Would delete "+args[0]+" (no changes made)",
            map[string]any{"would_delete": args[0]})
        return nil
    }

    // real deletion here
    w.WriteSuccess("Deleted "+args[0], map[string]any{"id": args[0]})
    return nil
}
```

`WritePlan` emits `"status": "plan"` — same envelope shape as `WriteSuccess` but signals "preview, not executed." Agents that see `"plan"` know to confirm before proceeding.

---

**Optional — check `IsForced()` for your own confirmation logic.**

```go
deleteCmd.RunE = func(cmd *cobra.Command, args []string) error {
    w := murliCobra.NewWriter(cmd)
    if !w.IsForced() && isHighRisk(args[0]) {
        return murli.NewUserError("high-risk operation", "Pass --force to confirm.")
    }
    // proceed
}
```

---

### Saved Profiles

**What it is:** Named sets of flag values that apply automatically on every invocation. Agents and humans stop repeating `--region us-east-1 --token abc` on every call — they save a profile once and use it by name.

**Principle:** One tool, two audiences (agents need persistent configuration too).

---

**Mark flags as profileable — the profile commands are auto-mounted.**

```go
// Annotate the root command (or app, for urfave/cli v2)
murliCobra.Annotate(rootCmd, murli.Metadata{
    FlagAnnotations: map[string]murli.FlagAnnotation{
        "region": {Profileable: true},
        "token":  {Profileable: true, Sensitive: true},
    },
})
```

murli auto-mounts `profile save`, `profile use`, `profile list`, `profile show`, and `profile delete`. No further code required.

---

**Save and activate a profile.**

```bash
# Save the current profileable flag values as "production"
$ mytool --region us-east-1 --token abc123 profile save production

# Set it as the default — all future calls use it automatically
$ mytool profile use production

# Now every invocation gets --region and --token without passing them
$ mytool query woodworking
```

Pass `--profile <name>` to override the default for a single invocation. An explicit flag on the command line always wins over a stored profile value. Profiles are stored in `~/.<toolname>/profiles.json`.

---

**Agents discover profiles via `describe`.**

```json
{
  "profiles": {
    "available": ["production", "staging"],
    "default": "production",
    "profileable_flags": ["region", "token"]
  }
}
```

`profileable_flags` tells agents which flags they can save. `available` and `default` tell agents what's already configured.

---

### Streaming & Progress

**What it is:** Long-running operations — indexing, batch processing, parallel fetches — can stream results and progress incrementally to their audience. Agents get NDJSON on stdout or structured progress objects on stderr; humans get in-place progress lines.

**Principle:** Context windows are finite.

---

**Stream incremental results with `WriteEvent`.**

`WriteEvent` is goroutine-safe. Call it as results become available, then close with `WriteSuccess` or `WriteError`:

```go
var wg sync.WaitGroup
for _, file := range files {
    wg.Add(1)
    go func(f string) {
        defer wg.Done()
        result := process(f)
        w.WriteEvent(result) // goroutine-safe; one minified JSON line per call
    }(file)
}
wg.Wait()
w.WriteSuccess("Processing complete", nil)
```

`WriteEvent` is a no-op in TTY mode — events are machine-only.

---

**Report progress with `WriteProgress`.**

For operations with measurable progress, `WriteProgress` writes to stderr (keeping stdout clean for results):

```go
w.WriteProgress(murli.ProgressEvent{
    Stage:   "indexing",
    Current: 500,
    Total:   2000,
    Percent: 25.0,
    EtaMs:   6000,
    Message: "Indexing files",
})
```

Agent mode: minified JSON on stderr. TTY mode: human-readable line with carriage return (overwrites in place). All fields are optional.

---

**Log with deduplication.**

`w.Progress()` and `w.Log()` write to stderr. In agent mode they produce NDJSON. Consecutive duplicate messages are collapsed with a `repeated` count, keeping agent context windows lean:

```go
w.Progress("Scanning /docs")
w.Progress("Scanning /docs") // deduplicated
w.Progress("Scanning /docs") // deduplicated
w.Flush()
```

Agent stderr output:

```
{"ts":"...","level":"progress","msg":"Scanning /docs","repeated":2}
```

ANSI escape codes in log messages are automatically stripped in agent mode, keeping JSON entries clean for parsers.

---

## Package Reference

| Package | Import path | Use when |
|---|---|---|
| Core | `github.com/allank/murli` | Always — `Writer`, `Logger`, `AgentError`, `Metadata`, schema types |
| cobra adapter | `github.com/allank/murli/cobra` | Your CLI uses [spf13/cobra](https://github.com/spf13/cobra) |
| cli/v2 adapter | `github.com/allank/murli/cli/v2` | Your CLI uses [urfave/cli v2](https://github.com/urfave/cli/tree/main/docs/v2) |
| cli/v3 adapter | `github.com/allank/murli/cli/v3` | Your CLI uses [urfave/cli v3](https://github.com/urfave/cli) |
| conformance | `github.com/allank/murli/conformance` | Verify your murli integration satisfies the 1.0 contract in CI |

Each adapter exposes the same surface:

| Purpose | cobra | cli/v2 | cli/v3 |
|---|---|---|---|
| Create writer | `cobra.NewWriter(cmd)` | `cli.NewWriter(ctx)` | `cli.NewWriter(cmd)` |
| Annotate command | `cobra.Annotate(cmd, meta)` | `cli.Annotate(cmd, meta)` | `cli.Annotate(cmd, meta)` |
| Annotate root flags | `cobra.Annotate(rootCmd, meta)` | `cli.AnnotateApp(app, meta)` | `cli.Annotate(app, meta)` |
| Enable + run | `cobra.Execute(rootCmd)` | `cli.Run(app, os.Args)` | `cli.Run(app, os.Args)` |
| Enable only | `cobra.Enable(rootCmd)` | `cli.Wrap(app)` | `cli.Wrap(app)` |
| Emit schema | `cobra.EmitSchema(cmd)` | `cli.EmitSchema(cmd, w)` | `cli.EmitSchema(cmd, w)` |

---

## Contract Compliance

The `murli/conformance` package lets downstream CLI maintainers verify their integration against the murli 1.0 contract in CI — without importing murli itself:

```go
func TestConformance(t *testing.T) {
    suite := conformance.NewSuite("/path/to/your/binary")
    suite.Check(t) // verifies describe output, schema_version, capabilities
}
```

---

## Testing

```bash
go test -race ./...
```

---

## License

Distributed under the MIT License. See [LICENSE](LICENSE) for details.
