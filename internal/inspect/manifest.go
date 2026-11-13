package inspect

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/Sorune/wsp/internal/config"
	"github.com/Sorune/wsp/internal/lens"
	"github.com/Sorune/wsp/internal/model"
)

func Manifest(root string) (model.Document, error) {
	m, err := config.Load(root)
	if err != nil {
		return model.Document{}, err
	}
	d := config.ToDocument(m)
	// Repository paths are authored relative to the manifest-owning workspace,
	// never to the process caller's current directory. Keep the source value for
	// provenance while exposing the resolved path to Lens consumers.
	for i := range d.Entities {
		if d.Entities[i].Kind != "REPOSITORY" {
			continue
		}
		configured := d.Entities[i].Attributes["path"]
		resolved := configured
		if !filepath.IsAbs(configured) {
			resolved = filepath.Join(root, configured)
		}
		resolved, err = filepath.Abs(resolved)
		if err != nil {
			return model.Document{}, fmt.Errorf("TARGET_UNAVAILABLE: resolve repository path: %w", err)
		}
		d.Entities[i].Attributes["path_configured"] = configured
		d.Entities[i].Attributes["path"] = filepath.Clean(resolved)
	}
	if err := ValidateDocument(d); err != nil {
		return model.Document{}, err
	}
	sort.Slice(d.Entities, func(i, j int) bool { return d.Entities[i].ID < d.Entities[j].ID })
	return d, nil
}

func Tree(root, axis string) (model.Document, lens.Tree, error) {
	d, err := Manifest(root)
	if err != nil {
		return model.Document{}, lens.Tree{}, err
	}
	if axis != "logical" && axis != "session" {
		return model.Document{}, lens.Tree{}, fmt.Errorf("INVALID_REQUEST: unsupported axis %q", axis)
	}
	return d, lens.TreeProjection(d, axis), nil
}
