// Package export defines the interface every exporter in this module
// implements; each format is a subpackage. Hold an Exporter to pick a format at
// run time, or to write your own.
package export

import (
	"errors"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh/cycle"
)

// ErrNilMesh is returned by Export and ExportCycle when the mesh is nil.
var ErrNilMesh = errors.New("export: mesh is nil")

// ErrNilCycle is returned by ExportCycle when the cycle is nil.
var ErrNilCycle = errors.New("export: cycle is nil")

// Exporter renders a mesh in one format. An exporter only reads the mesh, and
// its output is deterministic: equal meshes export byte-identical documents.
type Exporter interface {
	// Export renders the mesh structure: components, ports and pipes. A nil
	// mesh is ErrNilMesh.
	Export(fm *fmesh.FMesh) ([]byte, error)

	// ExportCycle renders the structure together with the results of one
	// cycle of a run, e.g. one from RuntimeInfo.Cycles or from an AfterCycle
	// hook. A component with no result in the cycle had no input. A nil mesh
	// is ErrNilMesh, and a nil cycle is ErrNilCycle.
	ExportCycle(fm *fmesh.FMesh, c *cycle.Cycle) ([]byte, error)
}
