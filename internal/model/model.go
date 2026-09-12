package model

// Provenance identifies where an observed or declared value came from. It is
// deliberately separate from semantic ownership.
type Provenance string

const (
	ProvenanceGit        Provenance = "GIT"
	ProvenanceFilesystem Provenance = "FILESYSTEM"
	ProvenanceConfig     Provenance = "CONFIG"
	ProvenanceAdapter    Provenance = "ADAPTER"
	ProvenanceDerived    Provenance = "DERIVED"
	ProvenanceUnknown    Provenance = "UNKNOWN"
)

type Entity struct {
	ID         string            `json:"id"`
	Kind       string            `json:"kind"`
	Name       string            `json:"name,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
	Provenance Provenance        `json:"provenance"`
	Unknown    []Unknown         `json:"unknowns,omitempty"`
}

type Relation struct {
	ID         string     `json:"id"`
	Type       string     `json:"type"`
	From       string     `json:"from"`
	To         string     `json:"to"`
	Provenance Provenance `json:"provenance"`
	Reason     string     `json:"reason,omitempty"`
}

type Finding struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	EntityID string `json:"entity_id,omitempty"`
}

type Unknown struct {
	Field      string     `json:"field"`
	EntityID   string     `json:"entity_id,omitempty"`
	Reason     string     `json:"reason"`
	Provenance Provenance `json:"provenance"`
}

type Document struct {
	SchemaVersion int        `json:"schema_version"`
	Command       string     `json:"command"`
	Status        string     `json:"status"`
	Target        string     `json:"target,omitempty"`
	Entities      []Entity   `json:"entities,omitempty"`
	Relations     []Relation `json:"relations,omitempty"`
	Findings      []Finding  `json:"findings,omitempty"`
	Unknowns      []Unknown  `json:"unknowns,omitempty"`
	Projection    any        `json:"projection,omitempty"`
	Errors        []Error    `json:"errors,omitempty"`
}

type Error struct {
	Category string `json:"category"`
	Message  string `json:"message"`
}

func UnknownValue(field, entityID, reason string, source Provenance) Unknown {
	if reason == "" {
		reason = "required evidence unavailable"
	}
	return Unknown{Field: field, EntityID: entityID, Reason: reason, Provenance: source}
}
