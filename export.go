// Package export defines the interface every exporter in this module
// implements. Each format is a subpackage: json, dot, mermaid, d2, plantuml.
// Hold an Exporter to pick a format at run time, or to write your own.
package export

import (
	"errors"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh/cycle"
)

// ErrNilCycle is returned by ExportCycle when the cycle is nil.
var ErrNilCycle = errors.New("export: cycle is nil")

// Exporter renders a mesh in one format. An exporter only reads the mesh, and
// its output is deterministic: equal meshes export byte-identical documents.
type Exporter interface {
	// Export renders the mesh structure: components, ports and pipes.
	Export(fm *fmesh.FMesh) ([]byte, error)

	// ExportCycle renders the structure together with the results of one
	// cycle of a run, e.g. one from RuntimeInfo.Cycles or from an AfterCycle
	// hook. A component with no result in the cycle had no input. A nil cycle
	// is ErrNilCycle.
	ExportCycle(fm *fmesh.FMesh, c *cycle.Cycle) ([]byte, error)
}
