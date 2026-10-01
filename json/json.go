// Package json exports a mesh as indented JSON: its structure, or its state
// in one cycle. Unmarshal the output into Mesh or Cycle to read it back.
package json

import (
	stdjson "encoding/json"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/cycle"
	"github.com/hovsep/fmesh/export"
	"github.com/hovsep/fmesh/meta"
	"github.com/hovsep/fmesh/port"
)

var _ export.Exporter = (*Exporter)(nil)

// Exporter exports a mesh as indented JSON. It has no options and no state, so
// one value can be reused.
type Exporter struct{}

// New returns the JSON exporter.
func New() *Exporter {
	return &Exporter{}
}

// Mesh is the document Export produces.
type Mesh struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Meta        map[string]any `json:"meta,omitempty"`
	Components  []Component    `json:"components"`
	Pipes       []Pipe         `json:"pipes"`
}

// Component is one component with its ports.
type Component struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Meta        map[string]any `json:"meta,omitempty"`
	Inputs      []Port         `json:"inputs"`
	Outputs     []Port         `json:"outputs"`
}

// Port is one input or output port.
type Port struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Meta        map[string]any `json:"meta,omitempty"`
}

// Pipe connects an output port to an input port.
type Pipe struct {
	From Endpoint `json:"from"`
	To   Endpoint `json:"to"`
}

// Endpoint names a port and the component it belongs to.
type Endpoint struct {
	Component string `json:"component"`
	Port      string `json:"port"`
}

// Cycle is the document ExportCycle produces: the structure plus one
// result per component, in component name order.
type Cycle struct {
	Number  int      `json:"number"`
	Mesh    Mesh     `json:"mesh"`
	Results []Result `json:"results"`
}

// Result is how one component's activation ended in a cycle.
type Result struct {
	Component string   `json:"component"`
	Code      string   `json:"code"`
	Activated bool     `json:"activated"`
	Errors    []string `json:"errors,omitempty"`
}

// Export returns the mesh structure as indented JSON, shaped as Mesh.
func (e *Exporter) Export(fm *fmesh.FMesh) ([]byte, error) {
	mesh, err := structureOf(fm)
	if err != nil {
		return nil, err
	}
	return stdjson.MarshalIndent(mesh, "", "  ")
}

// ExportCycle returns the structure and the results of c as indented JSON,
// shaped as Cycle.
func (e *Exporter) ExportCycle(fm *fmesh.FMesh, c *cycle.Cycle) ([]byte, error) {
	if c == nil {
		return nil, export.ErrNilCycle
	}
	mesh, err := structureOf(fm)
	if err != nil {
		return nil, err
	}
	doc := Cycle{Number: c.Number(), Mesh: mesh, Results: make([]Result, 0, len(mesh.Components))}
	for _, comp := range mesh.Components {
		doc.Results = append(doc.Results, resultOf(comp.Name, c.ActivationResults().ByName(comp.Name)))
	}
	return stdjson.MarshalIndent(doc, "", "  ")
}

// resultOf treats a missing result as NoInput, which is what it means.
func resultOf(name string, ar *component.ActivationResult) Result {
	if ar == nil {
		return Result{Component: name, Code: component.ActivationCodeNoInput.String()}
	}
	r := Result{Component: name, Code: ar.Code().String(), Activated: ar.Activated()}
	for _, err := range ar.ActivationErrors() {
		r.Errors = append(r.Errors, err.Error())
	}
	return r
}

func structureOf(fm *fmesh.FMesh) (Mesh, error) {
	b := &builder{}
	if err := fm.Walk(b); err != nil {
		return Mesh{}, err
	}
	return b.mesh, nil
}

// builder assembles a Mesh from a walk.
type builder struct {
	mesh Mesh
}

func (b *builder) VisitMesh(fm *fmesh.FMesh) error {
	b.mesh = Mesh{
		Name:        fm.Name(),
		Description: fm.Description(),
		Meta:        metaOf(fm.Meta()),
		Components:  []Component{},
		Pipes:       []Pipe{},
	}
	return nil
}

func (b *builder) VisitComponent(c *component.Component) error {
	b.mesh.Components = append(b.mesh.Components, Component{
		Name:        c.Name(),
		Description: c.Description(),
		Meta:        metaOf(c.Meta()),
		Inputs:      []Port{},
		Outputs:     []Port{},
	})
	return nil
}

// VisitPort adds the port to the component visited last: Walk visits a
// component's ports right after the component.
func (b *builder) VisitPort(_ *component.Component, p *port.Port) error {
	c := &b.mesh.Components[len(b.mesh.Components)-1]
	exported := Port{Name: p.Name(), Description: p.Description(), Meta: metaOf(p.Meta())}
	if p.IsInput() {
		c.Inputs = append(c.Inputs, exported)
	} else {
		c.Outputs = append(c.Outputs, exported)
	}
	return nil
}

func (b *builder) VisitPipe(from, to *port.Port) error {
	b.mesh.Pipes = append(b.mesh.Pipes, Pipe{From: endpointOf(from), To: endpointOf(to)})
	return nil
}

func endpointOf(p *port.Port) Endpoint {
	e := Endpoint{Port: p.Name()}
	if parent := p.ParentComponent(); parent != nil {
		e.Component = parent.Name()
	}
	return e
}

// metaOf returns nil for an empty store, so omitempty drops it.
func metaOf(m *meta.Meta) map[string]any {
	if m.IsEmpty() {
		return nil
	}
	return m.All()
}
