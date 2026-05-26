package cli_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/allank/murli"
	murliCLI "github.com/allank/murli/cli/v2"
	"github.com/urfave/cli/v2"
)

func TestV2SchemaEmit(t *testing.T) {
	cmd := &cli.Command{
		Name:  "query",
		Usage: "Semantic query search",
		Flags: []cli.Flag{
			&cli.IntFlag{Name: "top", Value: 5, Usage: "max results"},
			&cli.StringFlag{Name: "index", Usage: "index path"},
		},
		Action: func(ctx *cli.Context) error { return nil },
	}

	murliCLI.Annotate(cmd, murli.Metadata{
		AgentDescription: "Searches for semantic matches.",
		WhenToUse:        "Use for conceptual directory search.",
		Idempotent:       true,
		Arguments: []murli.ArgumentMetadata{
			{Name: "text", Type: "string", Required: true, Description: "Search query"},
		},
		Returns: &murli.ReturnSchema{
			Type:  "json",
			Shape: map[string]any{"path": "string", "score": "float32"},
		},
	})

	buf := &bytes.Buffer{}
	if err := murliCLI.EmitSchema(cmd, buf); err != nil {
		t.Fatalf("EmitSchema failed: %v", err)
	}

	var schema murli.CommandSchema
	if err := json.Unmarshal(buf.Bytes(), &schema); err != nil {
		t.Fatalf("failed to parse schema: %v", err)
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
	if len(schema.Arguments) != 1 || schema.Arguments[0].Name != "text" || !schema.Arguments[0].Required {
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
		t.Error("missing top flag in schema")
	}
}

func TestFlagAnnotationInSchemaV2(t *testing.T) {
	cmd := &cli.Command{
		Name:  "query",
		Usage: "Query",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "region", Usage: "AWS region"},
		},
		Action: func(ctx *cli.Context) error { return nil },
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

func TestV2MiddlewareWrapsError(t *testing.T) {
	var capturedExit int
	murli.ExitFunc = func(code int) { capturedExit = code }
	defer func() { murli.ExitFunc = func(code int) {} }()

	capturedExit = -999
	errBuf := &bytes.Buffer{}

	app := &cli.App{
		Name:      "testapp",
		ErrWriter: errBuf,
		Commands: []*cli.Command{
			{
				Name: "fail",
				Action: func(ctx *cli.Context) error {
					return errors.New("something broke")
				},
			},
		},
	}

	murliCLI.Wrap(app)
	err := app.Run([]string{"testapp", "fail"})
	_ = err

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

func TestV2MiddlewarePassesThroughAgentError(t *testing.T) {
	var capturedExit int
	murli.ExitFunc = func(code int) { capturedExit = code }
	defer func() { murli.ExitFunc = func(code int) {} }()

	capturedExit = -999
	errBuf := &bytes.Buffer{}

	app := &cli.App{
		Name:      "testapp",
		ErrWriter: errBuf,
		Commands: []*cli.Command{
			{
				Name: "customfail",
				Action: func(ctx *cli.Context) error {
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
	_ = app.Run([]string{"testapp", "customfail"})

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

func TestV2MutatingGuardBlocksInAgentMode(t *testing.T) {
	var capturedExit int
	murli.ExitFunc = func(code int) { capturedExit = code }
	defer func() { murli.ExitFunc = func(code int) {} }()

	capturedExit = -999
	errBuf := &bytes.Buffer{}

	app := &cli.App{
		Name:      "testapp",
		ErrWriter: errBuf,
		Commands: []*cli.Command{
			{
				Name: "delete",
				Action: func(ctx *cli.Context) error {
					t.Error("Action must not be called when guard fires")
					return nil
				},
			},
		},
	}
	murliCLI.Annotate(app.Commands[0], murli.Metadata{Mutating: true})
	murliCLI.Wrap(app)
	_ = app.Run([]string{"testapp", "delete"})

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

func TestV2SchemaFlag(t *testing.T) {
	buf := &bytes.Buffer{}

	app := &cli.App{
		Name:   "testapp",
		Writer: buf,
		Commands: []*cli.Command{
			{
				Name:   "query",
				Usage:  "run a query",
				Action: func(ctx *cli.Context) error { return nil },
				Flags:  []cli.Flag{&cli.IntFlag{Name: "top", Value: 3}},
			},
		},
	}

	murliCLI.Wrap(app)
	_ = app.Run([]string{"testapp", "query", "--schema"})

	var schema murli.CommandSchema
	if err := json.Unmarshal(buf.Bytes(), &schema); err != nil {
		t.Fatalf("schema flag did not produce valid JSON: %v\nOutput: %s", err, buf.String())
	}
	if schema.Name != "query" {
		t.Errorf("expected name 'query', got %q", schema.Name)
	}

	_ = os.Stdout
}
