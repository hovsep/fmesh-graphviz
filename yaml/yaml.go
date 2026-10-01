// Package yaml exports a mesh as YAML: its structure, or its state in one
// cycle. It encodes the module's shared data documents; unmarshal the output
// into Mesh or Cycle to read it back.
package yaml

import (
	"bytes"

	goyaml "go.yaml.in/yaml/v3"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-export"
	"github.com/hovsep/fmesh-export/internal/doc"
	"github.com/hovsep/fmesh/cycle"
)

var _ export.Exporter = (*Exporter)(nil)

// Exporter exports a mesh as YAML with a two-space indent. It has no options
// and no state, so one value can be reused.
type Exporter struct{}

// New returns the YAML exporter.
func New() *Exporter {
	return &Exporter{}
}

// The document types.
type (
	// Mesh is the document Export produces.
	Mesh = doc.Mesh
	// Component is one component with its ports.
	Component = doc.Component
	// Port is one input or output port.
	Port = doc.Port
	// Pipe connects an output port to an input port.
	Pipe = doc.Pipe
	// Endpoint names a port and the component it belongs to.
	Endpoint = doc.Endpoint
	// Cycle is the document ExportCycle produces: the structure plus one
	// result per component, in component name order.
	Cycle = doc.Cycle
	// Result is how one component's activation ended in a cycle.
	Result = doc.Result
)

// Export returns the mesh structure as YAML, shaped as Mesh. A nil mesh is
// export.ErrNilMesh.
func (e *Exporter) Export(fm *fmesh.FMesh) ([]byte, error) {
	mesh, err := doc.Structure(fm)
	if err != nil {
		return nil, err
	}
	return encode(mesh)
}

// ExportCycle returns the structure and the results of c as YAML, shaped as
// Cycle. A nil mesh is export.ErrNilMesh, and a nil cycle export.ErrNilCycle.
func (e *Exporter) ExportCycle(fm *fmesh.FMesh, c *cycle.Cycle) ([]byte, error) {
	cyc, err := doc.CycleOf(fm, c)
	if err != nil {
		return nil, err
	}
	return encode(cyc)
}

// encode writes v with a two-space indent. Map keys (metadata) come out
// sorted, so the output is deterministic.
func encode(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := goyaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
