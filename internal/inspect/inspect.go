package inspect

import (
	"fmt"
	"sort"

	gitadapter "github.com/Sorune/wsp/internal/adapter/git"
	"github.com/Sorune/wsp/internal/model"
	"github.com/Sorune/wsp/internal/normalize"
	"github.com/Sorune/wsp/internal/relation"
)

func Repository(path string) (model.Document, error) {
	facts, err := gitadapter.Inspect(path)
	if err != nil {
		return model.Document{}, err
	}
	doc := normalize.Repository(facts)
	if err := relation.Validate(doc.Relations, doc.Entities); err != nil {
		return model.Document{}, err
	}
	sort.Slice(doc.Entities, func(i, j int) bool { return doc.Entities[i].ID < doc.Entities[j].ID })
	doc.Relations = relation.Ordered(doc.Relations)
	return doc, nil
}

func WithError(command, category, message string) model.Document {
	return model.Document{SchemaVersion: 1, Command: command, Status: "ERROR", Errors: []model.Error{{Category: category, Message: message}}}
}

func ValidateDocument(doc model.Document) error {
	if doc.SchemaVersion != 1 {
		return fmt.Errorf("INVALID_CONFIGURATION: unsupported schema_version")
	}
	return relation.Validate(doc.Relations, doc.Entities)
}

func FactsFor(doc model.Document) map[string]model.Entity {
	out := map[string]model.Entity{}
	for _, e := range doc.Entities {
		out[e.ID] = e
	}
	return out
}
