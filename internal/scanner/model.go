// Package scanner defines WSP's optional, read-only mechanical observation model.
// Scanner evidence supports Lens calibration and structural-drift review; it does not define semantic authority.
package scanner

const (
	ScannerVersion          = "0.2.0"
	ObservationModelVersion = "1"
	DerivationModelVersion  = "1"
)

// Request describes one bounded repository observation.
type Request struct {
	Path, SubjectID string
	Config          Config
}

// Config contains deterministic source selection rules.
type Config struct {
	SourceRoots       []string `json:"source_roots"`
	NamespacePrefixes []string `json:"namespace_prefixes"`
}

// Subject identifies the repository revision being observed. Root is runtime
// location context and is intentionally excluded from serialized artifacts.
type Subject struct {
	ID          string `json:"id"`
	Revision    string `json:"revision"`
	Branch      string `json:"branch"`
	WorkingTree string `json:"working_tree"`
	Root        string `json:"-"`
}

type Producer struct {
	ID                     string `json:"id"`
	Version                string `json:"version"`
	ModelVersion           string `json:"model_version"`
	DerivationModelVersion string `json:"derivation_model_version"`
}

type ObservationBatch struct {
	Producer    Producer     `json:"producer"`
	Subject     Subject      `json:"subject"`
	Nodes       []Node       `json:"nodes"`
	Edges       []Edge       `json:"edges"`
	Evidence    []Evidence   `json:"evidence"`
	Coverage    []Coverage   `json:"coverage"`
	Unknowns    []Unknown    `json:"unknowns"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}
type Node struct {
	ID           string            `json:"id"`
	Kind         string            `json:"kind"`
	Coordinate   string            `json:"coordinate"`
	Name         string            `json:"name"`
	Attributes   map[string]string `json:"attributes"`
	EvidenceRefs []string          `json:"evidence_refs"`
}
type Edge struct {
	ID           string   `json:"id"`
	Kind         string   `json:"kind"`
	From         string   `json:"from"`
	To           string   `json:"to"`
	Resolution   string   `json:"resolution"`
	EvidenceRefs []string `json:"evidence_refs"`
}
type Evidence struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Path   string `json:"path"`
	Line   int    `json:"line,omitempty"`
	Column int    `json:"column,omitempty"`
	Detail string `json:"detail,omitempty"`
}
type Coverage struct {
	Capability string `json:"capability"`
	State      string `json:"state"`
}
type Unknown struct {
	ID           string   `json:"id"`
	Kind         string   `json:"kind"`
	Coordinate   string   `json:"coordinate"`
	Reason       string   `json:"reason"`
	EvidenceRefs []string `json:"evidence_refs"`
}
type Diagnostic struct {
	Code       string `json:"code"`
	Severity   string `json:"severity"`
	Coordinate string `json:"coordinate,omitempty"`
	Message    string `json:"message"`
}
type Digests struct {
	Structure string `json:"structure"`
	Evidence  string `json:"evidence"`
	Coverage  string `json:"coverage"`
	Producer  string `json:"producer"`
	Config    string `json:"config"`
	Artifact  string `json:"artifact"`
}

// PhysicalSnapshot is a normalized observation with separately auditable
// digest domains. Treat values as immutable after construction.
type PhysicalSnapshot struct {
	SchemaVersion int          `json:"schema_version"`
	Producer      Producer     `json:"producer"`
	Subject       Subject      `json:"subject"`
	Config        Config       `json:"config"`
	Nodes         []Node       `json:"nodes"`
	Edges         []Edge       `json:"edges"`
	Evidence      []Evidence   `json:"evidence"`
	Coverage      []Coverage   `json:"coverage"`
	Unknowns      []Unknown    `json:"unknowns"`
	Diagnostics   []Diagnostic `json:"diagnostics"`
	Digests       Digests      `json:"digests"`
}

type Confidence string

const (
	ConfidenceStrong     Confidence = "STRONG"
	ConfidenceSupported  Confidence = "SUPPORTED"
	ConfidenceWeak       Confidence = "WEAK"
	ConfidenceConflicted Confidence = "CONFLICTED"
	ConfidenceUnresolved Confidence = "UNRESOLVED"
)

type Signal struct {
	Kind         string   `json:"kind"`
	Value        string   `json:"value"`
	NodeIDs      []string `json:"node_ids"`
	EvidenceRefs []string `json:"evidence_refs"`
}
type Candidate struct {
	ID              string     `json:"id"`
	Coordinate      string     `json:"coordinate"`
	Label           string     `json:"label"`
	ParentID        string     `json:"parent_id,omitempty"`
	PhysicalNodeIDs []string   `json:"physical_node_ids"`
	Signals         []Signal   `json:"signals"`
	Confidence      Confidence `json:"confidence"`
	ReviewRequired  bool       `json:"review_required"`
	ReviewReasons   []string   `json:"review_reasons"`
}
type DerivedCandidateSet struct {
	Subject              Subject      `json:"subject"`
	Producer             Producer     `json:"producer"`
	SourceSnapshotDigest string       `json:"source_snapshot_digest"`
	Config               Config       `json:"config"`
	Candidates           []Candidate  `json:"candidates"`
	UnmappedNodeIDs      []string     `json:"unmapped_node_ids"`
	Coverage             []Coverage   `json:"coverage"`
	Unknowns             []Unknown    `json:"unknowns"`
	Diagnostics          []Diagnostic `json:"diagnostics"`
	Digest               string       `json:"digest"`
}
