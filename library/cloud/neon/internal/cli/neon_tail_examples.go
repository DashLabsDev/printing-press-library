// Hand-authored: Neon-specific examples for the generated tail command.

package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		tail, _, err := root.Find([]string{"tail"})
		if err != nil || tail == nil || tail.Name() != "tail" {
			return
		}
		tail.Example = strings.Trim(`
  # Poll projects once
  neon-pp-cli tail projects --follow=false

  # Watch projects every 30 seconds
  neon-pp-cli tail projects --interval 30s
`, "\n")
		if tail.Annotations == nil {
			tail.Annotations = map[string]string{}
		}
		tail.Annotations["pp:happy-args"] = "resource=projects;--follow=false"
	})
}
