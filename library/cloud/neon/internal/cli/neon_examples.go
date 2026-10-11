// Hand-authored: fill in help examples for generated commands whose request
// body is nested JSON (the generator omits an Example for those).

package cli

import (
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

var neonExampleIDs = map[string]string{
	"project_id": "lucky-sun-123456", "branch_id": "br-wispy-meadow-a1b2c3d4",
	"endpoint_id": "ep-cool-darkness-a1b2c3d4", "org_id": "org-young-tree-12345678",
	"source_org_id": "org-young-tree-12345678", "database_name": "neondb", "db_name": "neondb",
	"role_name": "neondb_owner", "snapshot_id": "snap-silent-sea-a1b2c3d4",
	"vpc_endpoint_id": "vpce-0123456789abcdef0", "region_id": "aws-us-east-2",
	"domain": "app.example.com", "slug": "my-function", "bucket_name": "uploads",
}

var neonUsagePositional = regexp.MustCompile(`<([a-z_]+)>`)

func neonSynthesizeExample(cmd *cobra.Command) string {
	args := make([]string, 0)
	for _, m := range neonUsagePositional.FindAllStringSubmatch(cmd.Use, -1) {
		v, ok := neonExampleIDs[m[1]]
		if !ok {
			v = "a07f8772-1877-4da9-a939-3a3ae62d1d8d"
		}
		args = append(args, v)
	}
	base := strings.TrimSpace(cmd.CommandPath() + " " + strings.Join(args, " "))
	lines := []string{"  " + base + " --dry-run"}
	if cmd.Flags().Lookup("stdin") != nil {
		lines = append(lines, "  "+base+" --stdin < request.json")
	}
	return strings.Join(lines, "\n")
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		var walk func(*cobra.Command)
		walk = func(c *cobra.Command) {
			for _, child := range c.Commands() {
				if child.Hidden {
					continue
				}
				if !child.HasSubCommands() && strings.TrimSpace(child.Example) == "" && child.Annotations["pp:endpoint"] != "" {
					child.Example = neonSynthesizeExample(child)
				}
				walk(child)
			}
		}
		walk(root)
	})
}
