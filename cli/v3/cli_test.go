package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/allank/murli"
	murliCLI "github.com/allank/murli/cli/v3"
	"github.com/urfave/cli/v3"
)

func TestV3SchemaEmit(t *testing.T) {
	cmd := &cli.Command{
		Name:  "query",
		Usage: "Semantic query search",
		Flags: []cli.Flag{
			&cli.IntFlag{Name: "top", Value: 5, Usage: "max results"},
		},
		Action: func(ctx context.Context, c *cli.Command) error { return nil },
	}

	murliCLI.Annotate(cmd, murli.Metadata{
		AgentDescription: "Searches for semantic matches.",
		WhenToUse:        "Use for conceptual directory search.",
		Idempotent:       true,
		Arguments: []murli.ArgumentMetadata{
			{Name: "text", Type: "string", Required: true, Description: "Search query"},
		},
	})

	buf := &bytes.Buffer{}
	if err := murliCLI.EmitSchema(cmd, buf); err != nil {
		t.Fatalf("EmitSchema failed: %v", err)
	}

	var schema murli.CommandSchema
	if err := json.Unmarshal(buf.Bytes(), &schema); err != nil {
		t.Fatalf("parse failed: %v\nOutput: %s", err, buf.String())
	}

	if schema.Name != "query" {
		t.Errorf("expected name 'query', got %q", schema.Name)
	}
	if schema.AgentDescription != "Searches for semantic matches." {
		t.Errorf("incorrect agent description")
	}
	if !schema.Idempotent {
		t.Error("expected idempotent = true")
	}
	if len(schema.Arguments) != 1 || !schema.Arguments[0].Required {
		t.Errorf("incorrect arguments: %+v", schema.Arguments)
	}
	var foundTop bool
	for _, f := range schema.Flags {
		if f.Name == "top" {
			foundTop = true
			if f.Type != "int" || f.Default.(float64) != 5 {
				t.Errorf("top flag incorrect: %+v", f)
			}
		}
	}
	if !foundTop {
		t.Error("missing top flag")
	}
}

func TestFlagAnnotationInSchemaV3(t *testing.T) {
	cmd := &cli.Command{
		Name:  "query",
		Usage: "Query",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "region", Usage: "AWS region"},
		},
		Action: func(ctx context.Context, c *cli.Command) error { return nil },
	}

	murliCLI.Annotate(cmd, murli.Metadata{
		FlagAnnotations: map[string]murli.FlagAnnotation{
			"region": {
				Env:  "AWS_REGION",
				Enum: []string{"us-east-1", "eu-west-1"},
			},
		},
	})

	buf := &bytes.Buffer{}
	if err := murliCLI.EmitSchema(cmd, buf); err != nil {
		t.Fatalf("EmitSchema: %v", err)
	}

	var schema murli.CommandSchema
	if err := json.Unmarshal(buf.Bytes(), &schema); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	var regionFlag *murli.FlagSchema
	for i := range schema.Flags {
		if schema.Flags[i].Name == "region" {
			regionFlag = &schema.Flags[i]
			break
		}
	}
	if regionFlag == nil {
		t.Fatal("region flag not found in schema")
	}
	if regionFlag.Env != "AWS_REGION" {
		t.Errorf("Env: got %q, want AWS_REGION", regionFlag.Env)
	}
	if len(regionFlag.Enum) != 2 {
		t.Errorf("Enum: got %v, want 2 items", regionFlag.Enum)
	}
}

func TestV3MiddlewareWrapsError(t *testing.T) {
	var capturedExit int
	origExit := murli.ExitFunc
	murli.ExitFunc = func(code int) { capturedExit = code }
	defer func() { murli.ExitFunc = origExit }()

	capturedExit = -999
	errBuf := &bytes.Buffer{}

	app := &cli.Command{
		Name:      "testapp",
		ErrWriter: errBuf,
		Commands: []*cli.Command{
			{
				Name: "fail",
				Action: func(ctx context.Context, c *cli.Command) error {
					return errors.New("something broke")
				},
			},
		},
	}

	murliCLI.Wrap(app)
	_ = app.Run(context.Background(), []string{"testapp", "fail"})

	if capturedExit != murli.ExitToolError {
		t.Errorf("expected exit %d, got %d", murli.ExitToolError, capturedExit)
	}

	var resp murli.AgentError
	if err := json.Unmarshal(errBuf.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse error JSON: %v", err)
	}
	if resp.ErrorType != "execution_error" || resp.Message != "something broke" {
		t.Errorf("incorrect error fields: %+v", resp)
	}
}

func TestV3MiddlewarePassesThroughAgentError(t *testing.T) {
	var capturedExit int
	origExit := murli.ExitFunc
	murli.ExitFunc = func(code int) { capturedExit = code }
	defer func() { murli.ExitFunc = origExit }()

	capturedExit = -999
	errBuf := &bytes.Buffer{}

	app := &cli.Command{
		Name:      "testapp",
		ErrWriter: errBuf,
		Commands: []*cli.Command{
			{
				Name: "customfail",
				Action: func(ctx context.Context, c *cli.Command) error {
					return &murli.AgentError{
						Code:        murli.ExitUserError,
						ErrorType:   "bad_input",
						Message:     "query was empty",
						Suggestion:  "Provide a non-empty search term.",
						Recoverable: true,
					}
				},
			},
		},
	}

	murliCLI.Wrap(app)
	_ = app.Run(context.Background(), []string{"testapp", "customfail"})

	if capturedExit != murli.ExitUserError {
		t.Errorf("expected exit %d, got %d", murli.ExitUserError, capturedExit)
	}

	var resp murli.AgentError
	if err := json.Unmarshal(errBuf.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	if resp.ErrorType != "bad_input" || resp.Suggestion != "Provide a non-empty search term." {
		t.Errorf("incorrect fields: %+v", resp)
	}
}

func TestV3MutatingGuardBlocksInAgentMode(t *testing.T) {
	var capturedExit int
	origExit := murli.ExitFunc
	murli.ExitFunc = func(code int) { capturedExit = code }
	defer func() { murli.ExitFunc = origExit }()

	capturedExit = -999
	errBuf := &bytes.Buffer{}

	deleteCmd := &cli.Command{
		Name: "delete",
		Action: func(ctx context.Context, c *cli.Command) error {
			t.Error("Action must not be called when guard fires")
			return nil
		},
	}
	murliCLI.Annotate(deleteCmd, murli.Metadata{Mutating: true})

	app := &cli.Command{
		Name:      "testapp",
		ErrWriter: errBuf,
		Commands:  []*cli.Command{deleteCmd},
	}
	murliCLI.Wrap(app)
	_ = app.Run(context.Background(), []string{"testapp", "delete"})

	if capturedExit != murli.ExitUserError {
		t.Errorf("exit code: want %d, got %d", murli.ExitUserError, capturedExit)
	}
	var resp murli.AgentError
	if err := json.Unmarshal(errBuf.Bytes(), &resp); err != nil {
		t.Fatalf("error envelope not valid JSON: %v\nOutput: %s", err, errBuf.String())
	}
	if resp.ErrorType != "confirmation_required" {
		t.Errorf("ErrorType: want %q, got %q", "confirmation_required", resp.ErrorType)
	}
	if !resp.Recoverable {
		t.Error("confirmation_required error must be Recoverable = true")
	}
}

func TestDescribeCommandV3(t *testing.T) {
	queryCmd := &cli.Command{
		Name:   "query",
		Usage:  "Semantic query",
		Action: func(ctx context.Context, c *cli.Command) error { return nil },
	}
	murliCLI.Annotate(queryCmd, murli.Metadata{
		AgentDescription: "Searches the semantic index.",
		Idempotent:       true,
	})

	buf := &bytes.Buffer{}
	app := &cli.Command{
		Name:     "riffle",
		Usage:    "Riffle semantic search",
		Writer:   buf,
		Commands: []*cli.Command{queryCmd},
	}

	murliCLI.Wrap(app)

	err := app.Run(context.Background(), []string{"riffle", "describe"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	var out murli.DescribeOutput
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, buf.String())
	}

	if out.Name != "riffle" {
		t.Errorf("name: got %q, want %q", out.Name, "riffle")
	}
	if !out.Capabilities.Streaming {
		t.Error("capabilities.streaming must be true")
	}
	if len(out.Commands) < 1 {
		t.Fatalf("expected at least 1 command, got %d", len(out.Commands))
	}
	var queryFound bool
	for _, cmd := range out.Commands {
		if cmd.Name == "query" {
			queryFound = true
			if !cmd.Idempotent {
				t.Error("query: expected Idempotent=true")
			}
		}
	}
	if !queryFound {
		t.Error("query command not found in describe output")
	}
}

func TestV3SchemaFlag(t *testing.T) {
	buf := &bytes.Buffer{}

	app := &cli.Command{
		Name:   "testapp",
		Writer: buf,
		Commands: []*cli.Command{
			{
				Name:   "query",
				Usage:  "run a query",
				Action: func(ctx context.Context, c *cli.Command) error { return nil },
				Flags:  []cli.Flag{&cli.IntFlag{Name: "top", Value: 3}},
			},
		},
	}

	murliCLI.Wrap(app)
	_ = app.Run(context.Background(), []string{"testapp", "query", "--schema"})

	var schema murli.CommandSchema
	if err := json.Unmarshal(buf.Bytes(), &schema); err != nil {
		t.Fatalf("schema flag did not produce valid JSON: %v\nOutput: %s", err, buf.String())
	}
	if schema.Name != "query" {
		t.Errorf("expected name 'query', got %q", schema.Name)
	}
}

func TestV3DryRunFlagRegisteredOnDryRunnableCommand(t *testing.T) {
	deleteCmd := &cli.Command{
		Name:   "delete",
		Action: func(ctx context.Context, c *cli.Command) error { return nil },
	}
	murliCLI.Annotate(deleteCmd, murli.Metadata{Mutating: true, DryRunnable: true})

	app := &cli.Command{Name: "testapp", Commands: []*cli.Command{deleteCmd}}
	murliCLI.Wrap(app)

	var found bool
	for _, f := range deleteCmd.Flags {
		for _, name := range f.Names() {
			if name == "dry-run" {
				found = true
			}
		}
	}
	if !found {
		t.Error("--dry-run must be registered on DryRunnable commands")
	}
}

func TestV3DryRunFlagNotRegisteredOnNonDryRunnableCommand(t *testing.T) {
	deleteCmd := &cli.Command{
		Name:   "delete",
		Action: func(ctx context.Context, c *cli.Command) error { return nil },
	}
	murliCLI.Annotate(deleteCmd, murli.Metadata{Mutating: true})

	app := &cli.Command{Name: "testapp", Commands: []*cli.Command{deleteCmd}}
	murliCLI.Wrap(app)

	for _, f := range deleteCmd.Flags {
		for _, name := range f.Names() {
			if name == "dry-run" {
				t.Error("--dry-run must NOT be registered on non-DryRunnable commands")
			}
		}
	}
}

func TestV3ForceFlagsRegisteredOnMutatingCommand(t *testing.T) {
	deleteCmd := &cli.Command{
		Name:   "delete",
		Action: func(ctx context.Context, c *cli.Command) error { return nil },
	}
	murliCLI.Annotate(deleteCmd, murli.Metadata{Mutating: true})

	app := &cli.Command{Name: "testapp", Commands: []*cli.Command{deleteCmd}}
	murliCLI.Wrap(app)

	var forceFound, yesFound bool
	for _, f := range deleteCmd.Flags {
		for _, name := range f.Names() {
			if name == "force" {
				forceFound = true
			}
			if name == "yes" {
				yesFound = true
			}
		}
	}
	if !forceFound {
		t.Error("--force must be registered on Mutating commands")
	}
	if !yesFound {
		t.Error("--yes must be registered on Mutating commands")
	}
}

func TestV3MutatingGuardBypassedWithForceFlag(t *testing.T) {
	ran := false
	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}

	deleteCmd := &cli.Command{
		Name:   "delete",
		Action: func(ctx context.Context, c *cli.Command) error { ran = true; return nil },
	}
	murliCLI.Annotate(deleteCmd, murli.Metadata{Mutating: true})

	app := &cli.Command{
		Name:      "testapp",
		Writer:    outBuf,
		ErrWriter: errBuf,
		Commands:  []*cli.Command{deleteCmd},
	}
	murliCLI.Wrap(app)
	_ = app.Run(context.Background(), []string{"testapp", "delete", "--force"})

	if !ran {
		t.Error("Action must run when --force is passed (guard bypassed)")
	}
}

func TestV3MutatingGuardBypassedWithYesFlag(t *testing.T) {
	ran := false
	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}

	deleteCmd := &cli.Command{
		Name:   "delete",
		Action: func(ctx context.Context, c *cli.Command) error { ran = true; return nil },
	}
	murliCLI.Annotate(deleteCmd, murli.Metadata{Mutating: true})

	app := &cli.Command{
		Name:      "testapp",
		Writer:    outBuf,
		ErrWriter: errBuf,
		Commands:  []*cli.Command{deleteCmd},
	}
	murliCLI.Wrap(app)
	_ = app.Run(context.Background(), []string{"testapp", "delete", "--yes"})

	if !ran {
		t.Error("Action must run when --yes is passed (guard bypassed)")
	}
}

func TestV3ContextCancelledMapsToExitCancelled(t *testing.T) {
	var capturedExit int
	origExit := murli.ExitFunc
	murli.ExitFunc = func(code int) { capturedExit = code }
	defer func() { murli.ExitFunc = origExit }()

	capturedExit = -999
	errBuf := &bytes.Buffer{}

	app := &cli.Command{
		Name:      "testapp",
		ErrWriter: errBuf,
		Commands: []*cli.Command{
			{
				Name: "work",
				Action: func(ctx context.Context, c *cli.Command) error {
					return context.Canceled
				},
			},
		},
	}
	murliCLI.Wrap(app)
	_ = app.Run(context.Background(), []string{"testapp", "work"})

	if capturedExit != murli.ExitCancelled {
		t.Errorf("exit code: want %d (ExitCancelled), got %d", murli.ExitCancelled, capturedExit)
	}
	var resp murli.AgentError
	if err := json.Unmarshal(errBuf.Bytes(), &resp); err != nil {
		t.Fatalf("not valid JSON: %v\nraw: %s", err, errBuf.String())
	}
	if resp.ErrorType != "cancelled" {
		t.Errorf("error_type: want %q, got %q", "cancelled", resp.ErrorType)
	}
	if resp.Recoverable {
		t.Error("cancelled error must not be Recoverable")
	}
}

func TestV3WrappedContextCancelledDetected(t *testing.T) {
	var capturedExit int
	origExit := murli.ExitFunc
	murli.ExitFunc = func(code int) { capturedExit = code }
	defer func() { murli.ExitFunc = origExit }()

	capturedExit = -999
	errBuf := &bytes.Buffer{}

	app := &cli.Command{
		Name:      "testapp",
		ErrWriter: errBuf,
		Commands: []*cli.Command{
			{
				Name: "work",
				Action: func(ctx context.Context, c *cli.Command) error {
					return fmt.Errorf("operation failed: %w", context.Canceled)
				},
			},
		},
	}
	murliCLI.Wrap(app)
	_ = app.Run(context.Background(), []string{"testapp", "work"})

	if capturedExit != murli.ExitCancelled {
		t.Errorf("exit code: want %d (ExitCancelled), got %d", murli.ExitCancelled, capturedExit)
	}
}

func TestV3DeadlineExceededMapsToExitTimeout(t *testing.T) {
	var capturedExit int
	origExit := murli.ExitFunc
	murli.ExitFunc = func(code int) { capturedExit = code }
	defer func() { murli.ExitFunc = origExit }()

	capturedExit = -999
	errBuf := &bytes.Buffer{}

	app := &cli.Command{
		Name:      "testapp",
		ErrWriter: errBuf,
		Commands: []*cli.Command{
			{
				Name: "work",
				Action: func(ctx context.Context, c *cli.Command) error {
					return context.DeadlineExceeded
				},
			},
		},
	}
	murliCLI.Wrap(app)
	_ = app.Run(context.Background(), []string{"testapp", "work"})

	if capturedExit != murli.ExitTimeout {
		t.Errorf("exit code: want %d (ExitTimeout), got %d", murli.ExitTimeout, capturedExit)
	}
	var resp murli.AgentError
	if err := json.Unmarshal(errBuf.Bytes(), &resp); err != nil {
		t.Fatalf("not valid JSON: %v\nraw: %s", err, errBuf.String())
	}
	if resp.ErrorType != "timeout" {
		t.Errorf("error_type: want %q, got %q", "timeout", resp.ErrorType)
	}
	if !resp.Recoverable {
		t.Error("timeout error must be Recoverable")
	}
}

func TestV3SafetyBlockInSchema(t *testing.T) {
	cmd := &cli.Command{
		Name:   "delete",
		Usage:  "Delete a resource",
		Action: func(ctx context.Context, c *cli.Command) error { return nil },
	}
	murliCLI.Annotate(cmd, murli.Metadata{
		Mutating:    true,
		Destructive: true,
		DryRunnable: true,
	})

	buf := &bytes.Buffer{}
	if err := murliCLI.EmitSchema(cmd, buf); err != nil {
		t.Fatalf("EmitSchema: %v", err)
	}

	var schema murli.CommandSchema
	if err := json.Unmarshal(buf.Bytes(), &schema); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, buf.String())
	}

	if schema.Safety.ReadOnly {
		t.Error("safety.read_only must be false for Mutating command")
	}
	if !schema.Safety.Destructive {
		t.Error("safety.destructive must be true")
	}
	if !schema.Safety.DryRunnable {
		t.Error("safety.dry_run_supported must be true")
	}
}

func TestV3InfraFlagsAbsentFromSchema(t *testing.T) {
	deleteCmd := &cli.Command{
		Name:   "delete",
		Action: func(ctx context.Context, c *cli.Command) error { return nil },
	}
	murliCLI.Annotate(deleteCmd, murli.Metadata{Mutating: true, DryRunnable: true})
	app := &cli.Command{Name: "testapp", Commands: []*cli.Command{deleteCmd}}
	murliCLI.Wrap(app)

	buf := &bytes.Buffer{}
	if err := murliCLI.EmitSchema(deleteCmd, buf); err != nil {
		t.Fatalf("EmitSchema: %v", err)
	}

	var schema murli.CommandSchema
	if err := json.Unmarshal(buf.Bytes(), &schema); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, buf.String())
	}

	infraFlags := map[string]bool{"force": true, "yes": true, "dry-run": true,
		"schema": true, "agent": true, "output": true, "protocol-version": true}
	for _, f := range schema.Flags {
		if infraFlags[f.Name] {
			t.Errorf("infrastructure flag %q must not appear in schema flags list", f.Name)
		}
	}
}

func TestV3RunEntryPointStructuredErrorOnFlagParse(t *testing.T) {
	var capturedExit int
	origExit := murli.ExitFunc
	murli.ExitFunc = func(code int) { capturedExit = code }
	defer func() { murli.ExitFunc = origExit }()

	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	app := &cli.Command{
		Name:      "myapp",
		Writer:    outBuf,
		ErrWriter: errBuf,
		Commands: []*cli.Command{
			{
				Name:  "hello",
				Flags: []cli.Flag{&cli.StringFlag{Name: "name"}},
				Action: func(ctx context.Context, c *cli.Command) error {
					return nil
				},
			},
		},
	}

	// Run returns nil (swallowed), error written to stderr via rootWriter.
	result := murliCLI.Run(app, []string{"myapp", "hello", "--unknown-xyz"})
	if result != nil {
		t.Errorf("Run() should return nil after writing structured error, got: %v", result)
	}
	// Verify that an exit was called due to the flag parse error.
	if capturedExit == 0 {
		t.Errorf("Run() should have called ExitFunc with non-zero code on flag parse error, got: %d", capturedExit)
	}
}
