package murli

import (
	"fmt"
	"io"
)

// nonConventionalVerbs maps non-standard command verb → preferred alternative.
var nonConventionalVerbs = map[string]string{
	"fetch":     "get",
	"info":      "get",
	"retrieve":  "get",
	"show-all":  "list",
	"ls":        "list",
	"enumerate": "list",
	"remove":    "delete",
	"rm":        "delete",
	"add":       "create",
	"new":       "create",
	"make":      "create",
	"edit":      "update",
	"modify":    "update",
	"set":       "update",
}

// nonConventionalFlags maps non-standard flag name → preferred alternative.
var nonConventionalFlags = map[string]string{
	"skip-confirmations": "force",
	"no-confirm":         "force",
	"silent":             "quiet",
	"no-output":          "quiet",
	"preview":            "dry-run",
	"what-if":            "dry-run",
	"format":             "output",
	"output-format":      "output",
}

// CheckConventions checks command names and flag names against the murli
// conventional vocabulary. Advisory warnings are written to w. Returns the number
// of warnings emitted. Never panics; suppressed output is safe (pass io.Discard).
func CheckConventions(commandNames, flagNames []string, w io.Writer) int {
	count := 0
	for _, name := range commandNames {
		if preferred, bad := nonConventionalVerbs[name]; bad {
			fmt.Fprintf(w, "[murli advisory] command %q: prefer %q (conventional vocabulary)\n", name, preferred)
			count++
		}
	}
	for _, name := range flagNames {
		if preferred, bad := nonConventionalFlags[name]; bad {
			fmt.Fprintf(w, "[murli advisory] flag --%s: prefer --%s (conventional vocabulary)\n", name, preferred)
			count++
		}
	}
	return count
}

// ConventionalVocabulary returns the recommended verb and flag vocabulary as a
// Conventions value suitable for inclusion in describe output.
func ConventionalVocabulary() *Conventions {
	return &Conventions{
		Vocabulary: map[string]string{
			"get":       "preferred verb for read operations (over fetch, info, retrieve)",
			"list":      "preferred verb for enumeration (over show-all, ls, enumerate)",
			"delete":    "preferred verb for removal (over remove, rm)",
			"create":    "preferred verb for creation (over add, new, make)",
			"update":    "preferred verb for modification (over edit, modify, set)",
			"--force":   "preferred flag for bypassing confirmations (over --skip-confirmations, --no-confirm)",
			"--quiet":   "preferred flag for suppressing output (over --silent, --no-output)",
			"--dry-run": "preferred flag for preview mode (over --preview, --what-if)",
			"--output":  "preferred flag for format selection (over --format, --output-format)",
		},
	}
}
