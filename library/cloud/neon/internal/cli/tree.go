// pp:data-source local
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/cloud/neon/internal/store"
)

type neonTreeNode struct {
	Kind     string          `json:"kind"`
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Children []*neonTreeNode `json:"children"`
}

func treeNode(kind, id, name string) *neonTreeNode {
	return &neonTreeNode{Kind: kind, ID: id, Name: name, Children: make([]*neonTreeNode, 0)}
}
func buildNeonTree(db *store.Store) ([]*neonTreeNode, error) {
	roots := make([]*neonTreeNode, 0)
	orgs, err := neonRows(db, "users-me-organizations", "organizations")
	if err != nil {
		return nil, err
	}
	byOrg := map[string]*neonTreeNode{}
	for _, o := range orgs {
		id := ns(o, "id")
		if _, ok := byOrg[id]; ok {
			continue
		}
		n := treeNode("organization", id, ns(o, "name"))
		byOrg[id] = n
		roots = append(roots, n)
	}
	projects, err := neonRows(db, "projects")
	if err != nil {
		return nil, err
	}
	byProject := map[string]*neonTreeNode{}
	for _, p := range projects {
		org := ns(p, "org_id")
		if org == "" {
			org = ns(p, "owner_id")
		}
		n, ok := byOrg[org]
		if !ok {
			n = treeNode("organization", org, org)
			byOrg[org] = n
			roots = append(roots, n)
		}
		pn := treeNode("project", ns(p, "id"), ns(p, "name"))
		byProject[pn.ID] = pn
		n.Children = append(n.Children, pn)
	}
	branches, err := neonRows(db, "branches", "projects/branches", "projects_branches")
	if err != nil {
		return nil, err
	}
	byBranch := map[string]*neonTreeNode{}
	parents := map[string]string{}
	for _, b := range branches {
		p := projectOf(b)
		key := p + "/" + ns(b, "id")
		byBranch[key] = treeNode("branch", ns(b, "id"), ns(b, "name"))
		parent := ns(b, "branch_parent_id")
		if parent == "" {
			parent = ns(b, "parent_id")
		}
		if parent != p {
			parents[key] = p + "/" + parent
		}
	}
	// Detect cycles before attachment; malformed mirrors fail instead of losing rows.
	for key := range byBranch {
		seen := map[string]bool{}
		for k := key; byBranch[k] != nil; k = parents[k] {
			if seen[k] {
				return nil, fmt.Errorf("branch parent cycle at %s", k)
			}
			seen[k] = true
		}
	}
	for _, b := range branches {
		p := projectOf(b)
		key := p + "/" + ns(b, "id")
		parent := byBranch[parents[key]]
		if parent == nil {
			parent = byProject[p]
		}
		if parent != nil {
			parent.Children = append(parent.Children, byBranch[key])
		}
	}
	endpoints, err := neonRows(db, "endpoints", "projects/endpoints", "projects_endpoints")
	if err != nil {
		return nil, err
	}
	for _, e := range endpoints {
		parent := byBranch[projectOf(e)+"/"+ns(e, "branch_id")]
		if parent != nil {
			parent.Children = append(parent.Children, treeNode("endpoint", ns(e, "id"), ns(e, "type")))
		}
	}
	return roots, nil
}
func printNeonTree(cmd *cobra.Command, nodes []*neonTreeNode, indent string) error {
	for _, n := range nodes {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s%s %s (%s)\n", indent, n.Kind, n.Name, n.ID); err != nil {
			return err
		}
		if err := printNeonTree(cmd, n.Children, indent+"  "); err != nil {
			return err
		}
	}
	return nil
}
func newNovelTreeCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{Use: "tree", Short: "Show organizations, projects, branch ancestry and endpoints.", Example: strings.Trim(`
  neon-pp-cli tree --db ./dev.db --json
`, "\n"), Annotations: map[string]string{"pp:data-source": "local", "mcp:read-only": "true", "pp:happy-args": "--db=./dev.db"}, RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && cmd.Flags().NFlag() == 0 {
			return cmd.Help()
		}
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "tree")
		}
		if len(args) != 0 {
			return neonMissing(cmd, "unexpected positional arguments")
		}
		db, err := openNeonMirror(cmd, flags, dbPath, "branches")
		if err != nil {
			return err
		}
		if db != nil {
			defer db.Close()
		}
		nodes, err := buildNeonTree(db)
		if err != nil {
			return err
		}
		if wantsHumanTable(cmd.OutOrStdout(), flags) {
			return printNeonTree(cmd, nodes, "")
		}
		return printJSONFiltered(cmd.OutOrStdout(), nodes, flags)
	}}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite mirror path")
	return cmd
}
