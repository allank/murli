package murli

import (
	"strings"
	"testing"
)

func TestRunDoctor_AllPass(t *testing.T) {
	out := DescribeOutput{
		Name:          "riffle",
		SchemaVersion: SchemaVersion,
		Capabilities: Capabilities{
			OutputFormats: []string{"json", "ndjson", "text"},
		},
		Commands: []DescribeCommandSchema{
			{Name: "query", Summary: "Search the index"},
		},
	}
	report := RunDoctor(out)
	if report.Failed > 0 {
		for _, c := range report.Checks {
			if c.Status == "fail" {
				t.Errorf("unexpected failure: %s — %s", c.Name, c.Message)
			}
		}
	}
	if len(report.Checks) == 0 {
		t.Error("RunDoctor must return at least one check")
	}
}

func TestRunDoctor_WrongSchemaVersion(t *testing.T) {
	out := DescribeOutput{
		Name:          "riffle",
		SchemaVersion: "0.2", // wrong — should be "1.0"
		Capabilities:  Capabilities{OutputFormats: []string{"json", "ndjson", "text"}},
	}
	report := RunDoctor(out)
	var found bool
	for _, c := range report.Checks {
		if c.Name == "schema_version" && c.Status == "fail" {
			found = true
		}
	}
	if !found {
		t.Error("RunDoctor must report schema_version fail when version is wrong")
	}
}

func TestRunDoctor_EmptyOutputFormats(t *testing.T) {
	out := DescribeOutput{
		Name:          "riffle",
		SchemaVersion: SchemaVersion,
		Capabilities:  Capabilities{OutputFormats: nil},
	}
	report := RunDoctor(out)
	var found bool
	for _, c := range report.Checks {
		if c.Name == "output_formats" && c.Status == "fail" {
			found = true
		}
	}
	if !found {
		t.Error("RunDoctor must report output_formats fail when formats are empty")
	}
}

func TestRunDoctor_MissingCommandMetadata(t *testing.T) {
	out := DescribeOutput{
		Name:          "riffle",
		SchemaVersion: SchemaVersion,
		Capabilities:  Capabilities{OutputFormats: []string{"json", "ndjson", "text"}},
		Commands: []DescribeCommandSchema{
			{Name: "query"}, // no summary or agent_description
		},
	}
	report := RunDoctor(out)
	var found bool
	for _, c := range report.Checks {
		if c.Name == "command_metadata" && c.Status == "warn" {
			found = true
		}
	}
	if !found {
		t.Error("RunDoctor must warn when commands are missing agent_description and summary")
	}
}

func TestWriteDoctorTTY(t *testing.T) {
	report := DoctorReport{}
	report.add(CheckResult{Name: "schema_version", Status: "pass"})
	report.add(CheckResult{Name: "output_formats", Status: "warn", Message: "formats reduced"})
	report.add(CheckResult{Name: "command_metadata", Status: "fail", Message: "missing desc"})

	var buf strings.Builder
	WriteDoctorTTY(&buf, report)
	got := buf.String()

	if !strings.Contains(got, "✓ schema_version") {
		t.Errorf("missing pass icon: %s", got)
	}
	if !strings.Contains(got, "⚠ output_formats: formats reduced") {
		t.Errorf("missing warn icon: %s", got)
	}
	if !strings.Contains(got, "✗ command_metadata: missing desc") {
		t.Errorf("missing fail icon: %s", got)
	}
	if !strings.Contains(got, "1 passed, 1 warnings, 1 failed") {
		t.Errorf("missing summary: %s", got)
	}
}

func TestDoctorReportCounts(t *testing.T) {
	report := DoctorReport{}
	report.add(CheckResult{Name: "a", Status: "pass"})
	report.add(CheckResult{Name: "b", Status: "warn"})
	report.add(CheckResult{Name: "c", Status: "fail"})
	if report.Passed != 1 {
		t.Errorf("Passed: want 1, got %d", report.Passed)
	}
	if report.Warnings != 1 {
		t.Errorf("Warnings: want 1, got %d", report.Warnings)
	}
	if report.Failed != 1 {
		t.Errorf("Failed: want 1, got %d", report.Failed)
	}
}
