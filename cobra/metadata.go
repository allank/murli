package cobra

import (
	"encoding/json"
	"fmt"

	"github.com/murli-cli/murli-go"
	"github.com/spf13/cobra"
)

// Annotate serializes murli Metadata and binds it to a Cobra command's Annotations map.
func Annotate(cmd *cobra.Command, meta murli.Metadata) {
	data, err := json.Marshal(meta)
	if err != nil {
		panic(fmt.Sprintf("murli: Annotate: failed to marshal metadata for command %q: %v\nHint: check that ReturnSchema.OutputSchema is valid JSON", cmd.Name(), err))
	}
	if cmd.Annotations == nil {
		cmd.Annotations = make(map[string]string)
	}
	cmd.Annotations["agentcobra"] = string(data)
}
