package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
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

func TestV3SafetyBlockInDescribeOutput(t *testing.T) {
	outBuf := &bytes.Buffer{}
	app := &cli.Command{
		Name:   "app",
		Writer: outBuf,
		Commands: []*cli.Command{
			{
				Name:   "delete",
				Action: func(ctx context.Context, c *cli.Command) error { return nil },
			},
		},
	}
	murliCLI.Annotate(app.Commands[0], murli.Metadata{
		Mutating:    true,
		Destructive: true,
		DryRunnable: true,
	})
	murliCLI.Wrap(app)

	if err := app.Run(context.Background(), []string{"app", "describe"}); err != nil {
		t.Fatalf("Run describe: %v", err)
	}

	var out murli.DescribeOutput
	if err := json.Unmarshal(outBuf.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, outBuf.String())
	}

	var deleteFound *murli.DescribeCommandSchema
	for i := range out.Commands {
		if out.Commands[i].Name == "delete" {
			deleteFound = &out.Commands[i]
			break
		}
	}
	if deleteFound == nil {
		t.Fatal("delete command not found in describe output")
	}
	if deleteFound.Safety.ReadOnly {
		t.Error("delete safety.read_only must be false")
	}
	if !deleteFound.Safety.Destructive {
		t.Error("delete safety.destructive must be true")
	}
	if !deleteFound.Safety.DryRunnable {
		t.Error("delete safety.dry_run_supported must be true")
	}
}

func TestV3ProfileFlagRegistered(t *testing.T) {
	app := &cli.Command{Name: "myapp", Writer: &bytes.Buffer{}}
	murliCLI.Wrap(app)
	for _, f := range app.Flags {
		if names := f.Names(); len(names) > 0 && names[0] == "profile" {
			return
		}
	}
	t.Error("--profile flag should be registered on app.Flags by Wrap()")
}

func TestV3ProfileFlagAbsentFromSchema(t *testing.T) {
	outBuf := &bytes.Buffer{}
	app := &cli.Command{
		Name:   "myapp",
		Writer: outBuf,
		Commands: []*cli.Command{
			{Name: "get", Usage: "get something",
				Action: func(ctx context.Context, c *cli.Command) error { return nil }},
		},
	}
	murliCLI.Wrap(app)
	if err := app.Run(context.Background(), []string{"myapp", "get", "--schema"}); err != nil {
		t.Fatalf("run: %v", err)
	}
	var schema murli.CommandSchema
	if err := json.Unmarshal(outBuf.Bytes(), &schema); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, outBuf.String())
	}
	for _, f := range schema.Flags {
		if f.Name == "profile" {
			t.Error("--profile must not appear in command schema flags")
		}
	}
}

func TestV3ProfileSavesChangedProfileableFlags(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	outBuf := &bytes.Buffer{}
	app := &cli.Command{
		Name:   "myapp",
		Writer: outBuf,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "region"},
			&cli.StringFlag{Name: "token"},
		},
	}
	murliCLI.Annotate(app, murli.Metadata{
		FlagAnnotations: map[string]murli.FlagAnnotation{
			"region": {Profileable: true},
			"token":  {Profileable: true},
		},
	})
	murliCLI.Wrap(app)

	if err := app.Run(context.Background(), []string{"myapp", "--region", "us-east-1", "--token", "abc", "--agent", "profile", "save", "prod"}); err != nil {
		t.Fatalf("run: %v", err)
	}

	store, err := murli.LoadProfileStore("myapp")
	if err != nil {
		t.Fatalf("LoadProfileStore: %v", err)
	}
	p, ok := store.Get("prod")
	if !ok {
		t.Fatal("profile prod not found")
	}
	if p.Flags["region"] != "us-east-1" {
		t.Errorf("expected region=us-east-1, got %q", p.Flags["region"])
	}
	if p.Flags["token"] != "abc" {
		t.Errorf("expected token=abc, got %q", p.Flags["token"])
	}
}

func TestV3ProfileApplicationAppliesDefaultProfile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	store := &murli.ProfileStore{
		Default: "staging",
		Profiles: map[string]murli.Profile{
			"staging": {Flags: map[string]string{"region": "eu-west-1"}},
		},
	}
	_ = store.Save("myapp")

	var capturedRegion string
	app := &cli.Command{
		Name:   "myapp",
		Writer: &bytes.Buffer{},
		Flags:  []cli.Flag{&cli.StringFlag{Name: "region"}},
		Commands: []*cli.Command{
			{Name: "list", Action: func(ctx context.Context, c *cli.Command) error {
				capturedRegion = c.Root().String("region")
				return nil
			}},
		},
	}
	murliCLI.Annotate(app, murli.Metadata{
		FlagAnnotations: map[string]murli.FlagAnnotation{"region": {Profileable: true}},
	})
	murliCLI.Wrap(app)

	if err := app.Run(context.Background(), []string{"myapp", "list"}); err != nil {
		t.Fatalf("run: %v", err)
	}
	if capturedRegion != "eu-west-1" {
		t.Errorf("expected eu-west-1 from default profile, got %q", capturedRegion)
	}
}

func TestV3ProfileApplicationExplicitFlagBeatsProfile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	store := &murli.ProfileStore{
		Default: "staging",
		Profiles: map[string]murli.Profile{
			"staging": {Flags: map[string]string{"region": "eu-west-1"}},
		},
	}
	_ = store.Save("myapp")

	var capturedRegion string
	app := &cli.Command{
		Name:   "myapp",
		Writer: &bytes.Buffer{},
		Flags:  []cli.Flag{&cli.StringFlag{Name: "region"}},
		Commands: []*cli.Command{
			{Name: "list", Action: func(ctx context.Context, c *cli.Command) error {
				capturedRegion = c.Root().String("region")
				return nil
			}},
		},
	}
	murliCLI.Annotate(app, murli.Metadata{
		FlagAnnotations: map[string]murli.FlagAnnotation{"region": {Profileable: true}},
	})
	murliCLI.Wrap(app)

	_ = app.Run(context.Background(), []string{"myapp", "--region", "ap-southeast-1", "list"})
	if capturedRegion != "ap-southeast-1" {
		t.Errorf("expected ap-southeast-1 (explicit beats profile), got %q", capturedRegion)
	}
}

func TestV3ProfileExplicitMissingProfileStopsExecution(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	origExit := murli.ExitFunc
	var capturedExit int
	murli.ExitFunc = func(code int) { capturedExit = code }
	defer func() { murli.ExitFunc = origExit }()

	actionCalled := false
	app := &cli.Command{
		Name:   "myapp",
		Writer: &bytes.Buffer{},
		Commands: []*cli.Command{
			{Name: "list", Action: func(ctx context.Context, c *cli.Command) error {
				actionCalled = true
				return nil
			}},
		},
	}
	murliCLI.Wrap(app)

	_ = app.Run(context.Background(), []string{"myapp", "--profile", "ghost", "list"})

	if capturedExit != murli.ExitNotFound {
		t.Errorf("expected ExitNotFound, got %d", capturedExit)
	}
	if actionCalled {
		t.Error("command action must not run when --profile names a missing profile")
	}
}

func TestV3DescribeIncludesProfilesInfo(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	store := &murli.ProfileStore{
		Default:  "prod",
		Profiles: map[string]murli.Profile{"prod": {Flags: map[string]string{"region": "us-east-1"}}},
	}
	_ = store.Save("myapp")

	outBuf := &bytes.Buffer{}
	app := &cli.Command{
		Name:   "myapp",
		Writer: outBuf,
		Flags:  []cli.Flag{&cli.StringFlag{Name: "region"}},
	}
	murliCLI.Annotate(app, murli.Metadata{
		FlagAnnotations: map[string]murli.FlagAnnotation{"region": {Profileable: true}},
	})
	murliCLI.Wrap(app)

	if err := app.Run(context.Background(), []string{"myapp", "describe"}); err != nil {
		t.Fatalf("run: %v", err)
	}
	var out murli.DescribeOutput
	if err := json.Unmarshal(outBuf.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, outBuf.String())
	}
	if !out.Capabilities.Profiles {
		t.Error("capabilities.profiles should be true")
	}
	if out.Profiles == nil {
		t.Fatal("profiles field should be present")
	}
	if len(out.Profiles.ProfileableFlags) == 0 || out.Profiles.ProfileableFlags[0] != "region" {
		t.Errorf("profileable_flags should contain region, got %v", out.Profiles.ProfileableFlags)
	}
	if out.Profiles.Default != "prod" {
		t.Errorf("expected default=prod, got %q", out.Profiles.Default)
	}
}

func TestV3DescribeAgentsMD(t *testing.T) {
	app := &cli.Command{
		Name:  "riffle",
		Usage: "Riffle semantic search",
		Commands: []*cli.Command{
			{Name: "query", Usage: "Search"},
		},
	}
	var buf bytes.Buffer
	app.Writer = &buf

	murliCLI.Wrap(app)

	err := app.Run(context.Background(), []string{"riffle", "describe", "--agents-md"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "# AGENTS.md") {
		t.Errorf("expected # AGENTS.md in output, got:\n%s", got)
	}
	if !strings.Contains(got, "## Tool: riffle") {
		t.Errorf("expected ## Tool: riffle in output, got:\n%s", got)
	}
	if strings.Contains(got, `"schema_version"`) {
		t.Error("--agents-md output must not contain JSON")
	}
}

