package cobra

import (
	"encoding/json"

	"github.com/allank/murli"
	"github.com/spf13/cobra"
)

// Annotate serializes murli Metadata and binds it to a Cobra command's Annotations map.
func Annotate(cmd *cobra.Command, meta murli.Metadata) {
	if cmd.Annotations == nil {
		cmd.Annotations = make(map[string]string)
	}
	data, _ := json.Marshal(meta)
	cmd.Annotations["agentcobra"] = string(data)
}
