package inspect

import (
	"fmt"
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
