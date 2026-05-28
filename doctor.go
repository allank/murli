package murli

import "fmt"

// CheckResult is the outcome of one doctor self-check.
type CheckResult struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // "pass", "warn", or "fail"
	Message string `json:"message,omitempty"`
}

// DoctorReport is the aggregate result of all doctor checks.
// Embedded in the WriteSuccess result payload by the doctor command.
type DoctorReport struct {
	Checks   []CheckResult `json:"checks"`
	Passed   int           `json:"passed"`
	Warnings int           `json:"warnings"`
	Failed   int           `json:"failed"`
}

// add appends a check and updates the pass/warn/fail counters.
func (r *DoctorReport) add(c CheckResult) {
	r.Checks = append(r.Checks, c)
	switch c.Status {
	case "pass":
		r.Passed++
	case "warn":
		r.Warnings++
	case "fail":
		r.Failed++
	}
}

// RunDoctor runs all self-checks against a DescribeOutput and returns a DoctorReport.
// Adapters call this after building the DescribeOutput in the doctor command handler.
func RunDoctor(out DescribeOutput) DoctorReport {
	var report DoctorReport
	report.add(checkSchemaVersion(out))
	report.add(checkOutputFormats(out))
	report.add(checkCommandMetadata(out))
	return report
}

func checkSchemaVersion(out DescribeOutput) CheckResult {
	if out.SchemaVersion == SchemaVersion {
		return CheckResult{Name: "schema_version", Status: "pass"}
	}
	return CheckResult{
		Name:    "schema_version",
		Status:  "fail",
		Message: fmt.Sprintf("schema_version is %q, want %q", out.SchemaVersion, SchemaVersion),
	}
}

func checkOutputFormats(out DescribeOutput) CheckResult {
	if len(out.Capabilities.OutputFormats) == 0 {
		return CheckResult{
			Name:    "output_formats",
			Status:  "fail",
			Message: "capabilities.output_formats is empty",
		}
	}
	return CheckResult{Name: "output_formats", Status: "pass"}
}

func checkCommandMetadata(out DescribeOutput) CheckResult {
	var missing []string
	for _, cmd := range out.Commands {
		if cmd.AgentDescription == "" && cmd.Summary == "" {
			missing = append(missing, cmd.Name)
		}
	}
	if len(missing) > 0 {
		return CheckResult{
			Name:    "command_metadata",
			Status:  "warn",
			Message: fmt.Sprintf("commands missing description: %v", missing),
		}
	}
	return CheckResult{Name: "command_metadata", Status: "pass"}
}
