package cobra_test

import (
	"bytes"
	"encoding/json"
	"errors"
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
