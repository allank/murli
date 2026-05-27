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

