package cobra_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/allank/murli"
	murliCobra "github.com/allank/murli/cobra"
	"github.com/spf13/cobra"
)

func TestSchemaGeneration(t *testing.T) {
	root := &cobra.Command{
		Use:   "demo <target> [option]",
		Short: "A lovely demonstration CLI command",
	}
	root.Flags().Int("limit", 10, "Page limit")
	root.Flags().Bool("verbose", false, "Enable verbose logs")

	sub := &cobra.Command{Use: "subcmd", Short: "Child command"}
	root.AddCommand(sub)

	murliCobra.Annotate(root, murli.Metadata{
		AgentDescription: "Use this command to demonstrate schema generation capabilities.",
		WhenToUse:        "When needing to verify the schema generation engine is healthy.",
		Idempotent:       true,
		Arguments: []murli.ArgumentMetadata{
			{Name: "target", Type: "string", Description: "The focal element under inspection"},
		},
		Returns: &murli.ReturnSchema{
			Type:        "json",
			Description: "Operation metadata and timestamps",
			Shape:       map[string]any{"id": "string", "timestamp": "int64"},
		},
		Examples: []murli.Example{{Command: "demo server-123 --limit 5"}},
	})

	buf := &bytes.Buffer{}
	root.SetOut(buf)

	if err := murliCobra.EmitSchema(root); err != nil {
		t.Fatalf("EmitSchema failed: %v", err)
	}

	var schema murli.CommandSchema
	if err := json.Unmarshal(buf.Bytes(), &schema); err != nil {
		t.Fatalf("failed to parse schema JSON: %v", err)
	}

	if schema.Name != "demo" {
		t.Errorf("expected name 'demo', got %q", schema.Name)
	}
	if schema.AgentDescription != "Use this command to demonstrate schema generation capabilities." {
		t.Errorf("incorrect agent description")
	}
	if !schema.Idempotent {
		t.Errorf("expected idempotent = true")
	}
	if len(schema.Arguments) != 2 {
		t.Fatalf("expected 2 arguments, got %d", len(schema.Arguments))
	}
	if schema.Arguments[0].Name != "target" || !schema.Arguments[0].Required || schema.Arguments[0].Description != "The focal element under inspection" {
		t.Errorf("target argument incorrect: %+v", schema.Arguments[0])
	}
	if schema.Arguments[1].Name != "option" || schema.Arguments[1].Required {
		t.Errorf("option argument incorrect: %+v", schema.Arguments[1])
	}
	var flagLimit, flagVerbose bool
	for _, f := range schema.Flags {
		if f.Name == "limit" {
			flagLimit = true
			if f.Type != "int" || f.Default.(float64) != 10 {
				t.Errorf("limit flag incorrect: %+v", f)
			}
		}
		if f.Name == "verbose" {
			flagVerbose = true
			if f.Type != "bool" || f.Default.(bool) != false {
				t.Errorf("verbose flag incorrect: %+v", f)
			}
		}
	}
	if !flagLimit || !flagVerbose {
		t.Errorf("missing expected flags")
	}
	if schema.Returns == nil || schema.Returns.Shape["id"] != "string" {
		t.Errorf("incorrect return schema: %+v", schema.Returns)
	}
	if len(schema.Subcommands) != 1 || schema.Subcommands[0].Name != "subcmd" {
		t.Errorf("subcommand incorrect: %+v", schema.Subcommands)
	}
}

func TestFlagAnnotationInSchema(t *testing.T) {
	root := &cobra.Command{Use: "demo", Short: "Demo"}
	root.Flags().String("region", "", "AWS region")
	root.Flags().String("format", "", "Output format")

	murliCobra.Annotate(root, murli.Metadata{
		AgentDescription: "Demo command",
		FlagAnnotations: map[string]murli.FlagAnnotation{
			"region": {
				Env:        "AWS_REGION",
				Enum:       []string{"us-east-1", "eu-west-1"},
				Persistent: true,
			},
			"format": {
				MutuallyExclusiveWith: []string{"json"},
			},
		},
	})

	buf := &bytes.Buffer{}
	root.SetOut(buf)
	if err := murliCobra.EmitSchema(root); err != nil {
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
		t.Errorf("Env: got %q, want %q", regionFlag.Env, "AWS_REGION")
	}
	if len(regionFlag.Enum) != 2 {
		t.Errorf("Enum: got %v, want 2 items", regionFlag.Enum)
	}
	if !regionFlag.Persistent {
		t.Errorf("Persistent: expected true")
	}
}

func TestMutatingGuardBlocksInAgentMode(t *testing.T) {
	var capturedExit int
	murli.ExitFunc = func(code int) { capturedExit = code }
	defer func() { murli.ExitFunc = func(code int) {} }()

	capturedExit = -999
	errBuf := &bytes.Buffer{}

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a resource",
		RunE: func(c *cobra.Command, args []string) error {
			t.Error("RunE must not be called when guard fires")
			return nil
		},
	}
	murliCobra.Annotate(cmd, murli.Metadata{
		Mutating: true,
	})
	murliCobra.Enable(cmd)
	cmd.SetErr(errBuf)
	_ = cmd.Execute()

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
		t.Error("confirmation_required must be Recoverable = true")
	}
}

func TestMutatingGuardAllowsNonMutating(t *testing.T) {
	ran := false
	cmd := &cobra.Command{
		Use:  "list",
		RunE: func(c *cobra.Command, args []string) error { ran = true; return nil },
	}
	murliCobra.Annotate(cmd, murli.Metadata{Mutating: false})
	murliCobra.Enable(cmd)
	outBuf, errBuf := &bytes.Buffer{}, &bytes.Buffer{}
	cmd.SetOut(outBuf)
	cmd.SetErr(errBuf)
	_ = cmd.Execute()

	if !ran {
		t.Error("non-mutating command RunE must run in agent mode")
	}
}

func TestDescribeCommand(t *testing.T) {
	root := &cobra.Command{Use: "riffle", Short: "Riffle semantic search"}
	queryCmd := &cobra.Command{Use: "query <text>", Short: "Semantic query"}
	deleteCmd := &cobra.Command{Use: "delete <id>", Short: "Delete index"}
	root.AddCommand(queryCmd, deleteCmd)

	murliCobra.Annotate(queryCmd, murli.Metadata{
		AgentDescription: "Searches the semantic index.",
		Idempotent:       true,
	})
	murliCobra.Annotate(deleteCmd, murli.Metadata{
		AgentDescription: "Deletes an index.",
		Mutating:         true,
	})

	murliCobra.Enable(root)

	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{"describe"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var out murli.DescribeOutput
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, buf.String())
	}

	if out.Name != "riffle" {
		t.Errorf("name: got %q, want %q", out.Name, "riffle")
	}
	if out.SchemaVersion == "" {
		t.Error("schema_version must be present")
	}
	if !out.Capabilities.Streaming {
		t.Error("capabilities.streaming must be true")
	}
	if len(out.Capabilities.OutputFormats) != 4 {
		t.Errorf("output_formats: got %v", out.Capabilities.OutputFormats)
	}
	if len(out.Commands) < 2 {
		t.Fatalf("expected at least 2 commands, got %d", len(out.Commands))
	}

	var queryFound, deleteFound bool
	for _, cmd := range out.Commands {
		if cmd.Name == "query" {
			queryFound = true
			if !cmd.Idempotent {
				t.Error("query: expected Idempotent=true")
			}
		}
		if cmd.Name == "delete" {
			deleteFound = true
			if !cmd.Mutating {
				t.Error("delete: expected Mutating=true")
			}
		}
	}
	if !queryFound {
		t.Error("query command not found in describe output")
	}
	if !deleteFound {
		t.Error("delete command not found in describe output")
	}

	if out.Conventions == nil || len(out.Conventions.Vocabulary) == 0 {
		t.Error("conventions.vocabulary must be present and non-empty")
	}
}

func TestMiddlewareInterception(t *testing.T) {
	var capturedExit int
	murli.ExitFunc = func(code int) { capturedExit = code }
	defer func() { murli.ExitFunc = func(code int) {} }()

	t.Run("wraps standard error as ExitToolError", func(t *testing.T) {
		capturedExit = -999
		cmd := &cobra.Command{
			Use: "failcmd",
			RunE: func(c *cobra.Command, args []string) error {
				return errors.New("underlying socket closed")
			},
		}
		murliCobra.Enable(cmd)
		bufErr := &bytes.Buffer{}
		cmd.SetErr(bufErr)
		_ = cmd.Execute()

		if capturedExit != murli.ExitToolError {
			t.Errorf("expected exit %d, got %d", murli.ExitToolError, capturedExit)
		}
		var resp murli.AgentError
		if err := json.Unmarshal(bufErr.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse error JSON: %v", err)
		}
		if resp.ErrorType != "execution_error" || resp.Message != "underlying socket closed" {
			t.Errorf("incorrect error fields: %+v", resp)
		}
	})

	t.Run("passes through AgentError fields", func(t *testing.T) {
		capturedExit = -999
		cmd := &cobra.Command{
			Use: "customfail",
			RunE: func(c *cobra.Command, args []string) error {
				return &murli.AgentError{
					Code:        murli.ExitUserError,
					ErrorType:   "file_not_found",
					Message:     "config.json is missing",
					Suggestion:  "Create config.json under project root.",
					Recoverable: true,
				}
			},
		}
		murliCobra.Enable(cmd)
		bufErr := &bytes.Buffer{}
		cmd.SetErr(bufErr)
		_ = cmd.Execute()

		if capturedExit != murli.ExitUserError {
			t.Errorf("expected exit %d, got %d", murli.ExitUserError, capturedExit)
		}
		var resp murli.AgentError
		if err := json.Unmarshal(bufErr.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse: %v", err)
		}
		if resp.ErrorType != "file_not_found" || resp.Suggestion != "Create config.json under project root." {
			t.Errorf("incorrect fields: %+v", resp)
		}
	})

	t.Run("flag parse error becomes ExitUserError", func(t *testing.T) {
		capturedExit = -999
		cmd := &cobra.Command{
			Use:  "flagtest",
			RunE: func(c *cobra.Command, args []string) error { return nil },
		}
		cmd.Flags().Int("num", 0, "A number")
		murliCobra.Enable(cmd)
		bufErr := &bytes.Buffer{}
		cmd.SetErr(bufErr)
		cmd.SetArgs([]string{"--num", "not-a-number"})
		_ = cmd.Execute()

		if capturedExit != murli.ExitUserError {
			t.Errorf("expected exit %d, got %d", murli.ExitUserError, capturedExit)
		}
		var resp murli.AgentError
		if err := json.Unmarshal(bufErr.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse: %v", err)
		}
		if resp.ErrorType != "flag_error" || !strings.Contains(resp.Message, "not-a-number") {
			t.Errorf("incorrect flag error: %+v", resp)
		}
	})
}

func TestDryRunFlagRegisteredOnDryRunnableCommand(t *testing.T) {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a resource",
		RunE:  func(c *cobra.Command, args []string) error { return nil },
	}
	murliCobra.Annotate(cmd, murli.Metadata{
		Mutating:    true,
		DryRunnable: true,
	})
	murliCobra.Enable(cmd)

	if cmd.Flags().Lookup("dry-run") == nil {
		t.Error("--dry-run must be registered on DryRunnable commands")
	}
}

func TestDryRunFlagNotRegisteredOnNonDryRunnableCommand(t *testing.T) {
	cmd := &cobra.Command{
		Use:  "delete",
		RunE: func(c *cobra.Command, args []string) error { return nil },
	}
	murliCobra.Annotate(cmd, murli.Metadata{Mutating: true}) // DryRunnable not set
	murliCobra.Enable(cmd)

	if cmd.Flags().Lookup("dry-run") != nil {
		t.Error("--dry-run must NOT be registered on non-DryRunnable commands")
	}
}

func TestForceFlagsRegisteredOnMutatingCommand(t *testing.T) {
	cmd := &cobra.Command{
		Use:  "delete",
		RunE: func(c *cobra.Command, args []string) error { return nil },
	}
	murliCobra.Annotate(cmd, murli.Metadata{Mutating: true})
	murliCobra.Enable(cmd)

	if cmd.Flags().Lookup("force") == nil {
		t.Error("--force must be registered on Mutating commands")
	}
	if cmd.Flags().Lookup("yes") == nil {
		t.Error("--yes must be registered on Mutating commands")
	}
}

func TestForceFlagsNotRegisteredOnReadOnlyCommand(t *testing.T) {
	cmd := &cobra.Command{
		Use:  "list",
		RunE: func(c *cobra.Command, args []string) error { return nil },
	}
	murliCobra.Annotate(cmd, murli.Metadata{Mutating: false})
	murliCobra.Enable(cmd)

	if cmd.Flags().Lookup("force") != nil {
		t.Error("--force must NOT be registered on read-only commands")
	}
}

func TestMutatingGuardBypassedWithForceFlag(t *testing.T) {
	ran := false
	cmd := &cobra.Command{
		Use:  "delete",
		RunE: func(c *cobra.Command, args []string) error { ran = true; return nil },
	}
	murliCobra.Annotate(cmd, murli.Metadata{Mutating: true})
	murliCobra.Enable(cmd)
	outBuf, errBuf := &bytes.Buffer{}, &bytes.Buffer{}
	cmd.SetOut(outBuf)
	cmd.SetErr(errBuf)
	cmd.SetArgs([]string{"--force"})
	_ = cmd.Execute()

	if !ran {
		t.Error("RunE must run when --force is passed (guard bypassed)")
	}
}

func TestMutatingGuardBypassedWithYesFlag(t *testing.T) {
	ran := false
	cmd := &cobra.Command{
		Use:  "delete",
		RunE: func(c *cobra.Command, args []string) error { ran = true; return nil },
	}
	murliCobra.Annotate(cmd, murli.Metadata{Mutating: true})
	murliCobra.Enable(cmd)
	outBuf, errBuf := &bytes.Buffer{}, &bytes.Buffer{}
	cmd.SetOut(outBuf)
	cmd.SetErr(errBuf)
	cmd.SetArgs([]string{"--yes"})
	_ = cmd.Execute()

	if !ran {
		t.Error("RunE must run when --yes is passed (guard bypassed)")
	}
}

func TestContextCancelledMapsToExitCancelled(t *testing.T) {
	var capturedExit int
	murli.ExitFunc = func(code int) { capturedExit = code }
	defer func() { murli.ExitFunc = func(code int) {} }()

	capturedExit = -999
	errBuf := &bytes.Buffer{}

	cmd := &cobra.Command{
		Use: "work",
		RunE: func(c *cobra.Command, args []string) error {
			return context.Canceled
		},
	}
	murliCobra.Enable(cmd)
	cmd.SetErr(errBuf)
	_ = cmd.Execute()

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

func TestWrappedContextCancelledDetected(t *testing.T) {
	var capturedExit int
	murli.ExitFunc = func(code int) { capturedExit = code }
	defer func() { murli.ExitFunc = func(code int) {} }()

	capturedExit = -999
	errBuf := &bytes.Buffer{}

	cmd := &cobra.Command{
		Use: "work",
		RunE: func(c *cobra.Command, args []string) error {
			return fmt.Errorf("operation failed: %w", context.Canceled)
		},
	}
	murliCobra.Enable(cmd)
	cmd.SetErr(errBuf)
	_ = cmd.Execute()

	if capturedExit != murli.ExitCancelled {
		t.Errorf("exit code: want %d (ExitCancelled), got %d", murli.ExitCancelled, capturedExit)
	}
}

func TestDeadlineExceededMapsToExitTimeout(t *testing.T) {
	var capturedExit int
	murli.ExitFunc = func(code int) { capturedExit = code }
	defer func() { murli.ExitFunc = func(code int) {} }()

	capturedExit = -999
	errBuf := &bytes.Buffer{}

	cmd := &cobra.Command{
		Use: "work",
		RunE: func(c *cobra.Command, args []string) error {
			return context.DeadlineExceeded
		},
	}
	murliCobra.Enable(cmd)
	cmd.SetErr(errBuf)
	_ = cmd.Execute()

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

func TestSafetyBlockInSchema(t *testing.T) {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a resource",
		RunE:  func(c *cobra.Command, args []string) error { return nil },
	}
	murliCobra.Annotate(cmd, murli.Metadata{
		Mutating:    true,
		Destructive: true,
		DryRunnable: true,
	})

	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	if err := murliCobra.EmitSchema(cmd); err != nil {
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

func TestSafetyBlockReadOnlyInSchema(t *testing.T) {
	cmd := &cobra.Command{
		Use:  "list",
		RunE: func(c *cobra.Command, args []string) error { return nil },
	}
	murliCobra.Annotate(cmd, murli.Metadata{Mutating: false, Idempotent: true})

	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	if err := murliCobra.EmitSchema(cmd); err != nil {
		t.Fatalf("EmitSchema: %v", err)
	}

	var schema murli.CommandSchema
	if err := json.Unmarshal(buf.Bytes(), &schema); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, buf.String())
	}

	if !schema.Safety.ReadOnly {
		t.Error("safety.read_only must be true for non-Mutating command")
	}
	if !schema.Safety.Idempotent {
		t.Error("safety.idempotent must be true")
	}
}

func TestInfraFlagsAbsentFromSchema(t *testing.T) {
	cmd := &cobra.Command{
		Use:  "delete",
		RunE: func(c *cobra.Command, args []string) error { return nil },
	}
	murliCobra.Annotate(cmd, murli.Metadata{Mutating: true, DryRunnable: true})
	murliCobra.Enable(cmd)

	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	if err := murliCobra.EmitSchema(cmd); err != nil {
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

func TestSafetyBlockInDescribeOutput(t *testing.T) {
	root := &cobra.Command{Use: "app", Short: "Test app"}
	deleteCmd := &cobra.Command{
		Use:  "delete",
		RunE: func(c *cobra.Command, args []string) error { return nil },
	}
	murliCobra.Annotate(deleteCmd, murli.Metadata{
		Mutating:    true,
		Destructive: true,
		DryRunnable: true,
	})
	root.AddCommand(deleteCmd)
	murliCobra.Enable(root)

	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{"describe"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var out murli.DescribeOutput
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, buf.String())
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
