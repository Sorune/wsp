package present

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/Sorune/wsp/internal/lens"
	"github.com/Sorune/wsp/internal/model"
)

func JSON(doc model.Document) ([]byte, error) {
	// Keep the public envelope shape stable: an empty collection is represented
	// as [] rather than disappearing or becoming an implementation-dependent
	// nil value.
	if doc.Entities == nil {
		doc.Entities = []model.Entity{}
	}
	if doc.Relations == nil {
		doc.Relations = []model.Relation{}
	}
	if doc.Findings == nil {
		doc.Findings = []model.Finding{}
	}
	if doc.Unknowns == nil {
		doc.Unknowns = []model.Unknown{}
	}
	if doc.Errors == nil {
		doc.Errors = []model.Error{}
	}
	return json.MarshalIndent(doc, "", "  ")
}

func Document(doc model.Document) string {
	var b strings.Builder
	fmt.Fprintf(&b, "STATUS: %s\n", doc.Status)
	if doc.Target != "" {
		fmt.Fprintf(&b, "TARGET: %s\n", doc.Target)
	}
	for _, e := range doc.Entities {
		fmt.Fprintf(&b, "%s %s\n", e.Kind, e.Name)
		keys := make([]string, 0, len(e.Attributes))
		for k := range e.Attributes {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			label := map[string]string{"root": "REPOSITORY_ROOT", "remote": "REMOTE_ORIGIN", "working_tree": "WORKING_TREE", "revision": "REVISION", "branch": "BRANCH", "upstream": "UPSTREAM", "ahead": "AHEAD", "behind": "BEHIND"}[k]
			if label == "" {
				label = strings.ToUpper(k)
			}
			fmt.Fprintf(&b, "  %s: %s\n", label, e.Attributes[k])
		}
		for _, u := range e.Unknown {
			fmt.Fprintf(&b, "  UNKNOWN %s: %s\n", u.Field, u.Reason)
		}
	}
	for _, f := range doc.Findings {
		fmt.Fprintf(&b, "FINDING %s: %s\n", f.Code, f.Message)
	}
	for _, u := range doc.Unknowns {
		fmt.Fprintf(&b, "UNKNOWN %s: %s\n", u.Field, u.Reason)
	}
	for _, e := range doc.Errors {
		fmt.Fprintf(&b, "ERROR %s: %s\n", e.Category, e.Message)
	}
	fmt.Fprintln(&b, "NETWORK_FETCH: NOT_PERFORMED")
	fmt.Fprintln(&b, "AUTHORITY: OBSERVED_ONLY")
	return strings.TrimRight(b.String(), "\n") + "\n"
}

func Tree(t lens.Tree) string {
	var b strings.Builder
	fmt.Fprintf(&b, "AXIS: %s\n", t.Axis)
	for i, root := range t.Roots {
		renderNode(&b, root, "", i == len(t.Roots)-1)
	}
	return b.String()
}

func renderNode(b *strings.Builder, n lens.Node, prefix string, last bool) {
	branch := "├─ "
	next := prefix + "│  "
	if last {
		branch = "└─ "
		next = prefix + "   "
	}
	if prefix == "" {
		fmt.Fprintf(b, "%s\n", n.Label)
	} else {
		fmt.Fprintf(b, "%s%s%s\n", prefix, branch, n.Label)
	}
	for i, c := range n.Children {
		renderNode(b, c, next, i == len(n.Children)-1)
	}
}
