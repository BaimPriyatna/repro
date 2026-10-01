// Package entity defines the core Entity domain type for Repro.
//
// An Entity represents any named, trackable component in the development
// environment — packages, files, services, runtimes, configurations, etc.
// Entities are the nodes in the RepairMap dependency graph.
package entity

// Kind classifies what category of resource an Entity represents.
type Kind string

const (
	// KindPackage is a software package (library, module, binary).
	KindPackage Kind = "package"

	// KindFile is a file tracked by the system.
	KindFile Kind = "file"

	// KindService is a running or configured service.
	KindService Kind = "service"

	// KindRuntime is a language runtime or interpreter.
	KindRuntime Kind = "runtime"

	// KindConfig is a configuration file or key-value store.
	KindConfig Kind = "config"

	// KindTool is an external CLI tool or build utility.
	KindTool Kind = "tool"

	// KindModule is a code module (directory, Go package, etc.).
	KindModule Kind = "module"

	// KindUnknown is used when the kind cannot be determined.
	KindUnknown Kind = "unknown"
)

// ID is a stable, unique entity identifier within the Repro system.
// The format is implementation-defined but must be globally unique within
// a single local store.
type ID string

// Entity is a named, trackable component in the development environment.
//
// Entities are the nodes in the RepairMap dependency graph.
// The same physical resource should map to the same Entity ID across snapshots
// to allow history tracking and impact analysis.
type Entity struct {
	// ID is the stable, unique identifier for this entity.
	ID ID `json:"id"`

	// Kind classifies the type of resource this entity represents.
	Kind Kind `json:"kind"`

	// Name is the human-readable name of the entity (e.g. "react", "go", "/etc/hosts").
	Name string `json:"name"`

	// Version is the optional version string of the entity at the time of capture.
	// Empty string means version is unknown or not applicable.
	Version string `json:"version,omitempty"`

	// Namespace provides optional scope context (e.g. npm scope, Go module path,
	// OS distribution). Empty string means global / unscoped.
	Namespace string `json:"namespace,omitempty"`

	// Attributes holds additional, kind-specific structured metadata.
	// The storage layer must preserve unknown keys without modification.
	Attributes map[string]any `json:"attributes,omitempty"`
}

// IsVersioned reports whether the entity has a known version.
func (e *Entity) IsVersioned() bool {
	return e.Version != ""
}
