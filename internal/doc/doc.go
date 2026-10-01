// Package doc builds the documents that data-format exporters encode: the mesh
// structure, and the structure with one cycle's results. Sharing these types
// keeps every data format carrying the same fields.
package doc

import (
	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-export"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/cycle"
	"github.com/hovsep/fmesh/meta"
	"github.com/hovsep/fmesh/port"
)

// Mesh is the structure of a mesh.
type Mesh struct {
	Name        string         `json:"name" yaml:"name"`
	Description string         `json:"description,omitempty" yaml:"description,omitempty"`
	Meta        map[string]any `json:"meta,omitempty" yaml:"meta,omitempty"`
	Components  []Component    `json:"components" yaml:"components"`
	Pipes       []Pipe         `json:"pipes" yaml:"pipes"`
}

// Component is one component with its ports.
type Component struct {
	Name        string         `json:"name" yaml:"name"`
	Description string         `json:"description,omitempty" yaml:"description,omitempty"`
	Meta        map[string]any `json:"meta,omitempty" yaml:"meta,omitempty"`
	Inputs      []Port         `json:"inputs" yaml:"inputs"`
	Outputs     []Port         `json:"outputs" yaml:"outputs"`
}

// Port is one input or output port.
type Port struct {
	Name        string         `json:"name" yaml:"name"`
	Description string         `json:"description,omitempty" yaml:"description,omitempty"`
	Meta        map[string]any `json:"meta,omitempty" yaml:"meta,omitempty"`
}

// Pipe connects an output port to an input port.
type Pipe struct {
	From Endpoint `json:"from" yaml:"from"`
	To   Endpoint `json:"to" yaml:"to"`
}

// Endpoint names a port and the component it belongs to.
type Endpoint struct {
	Component string `json:"component" yaml:"component"`
	Port      string `json:"port" yaml:"port"`
}

// Cycle is the structure plus one result per component, in component name
// order.
type Cycle struct {
	Number  int      `json:"number" yaml:"number"`
	Mesh    Mesh     `json:"mesh" yaml:"mesh"`
	Results []Result `json:"results" yaml:"results"`
}

// Result is how one component's activation ended in a cycle.
type Result struct {
	Component string   `json:"component" yaml:"component"`
	Code      string   `json:"code" yaml:"code"`
	Activated bool     `json:"activated" yaml:"activated"`
	Errors    []string `json:"errors,omitempty" yaml:"errors,omitempty"`
}

// Structure walks fm into a Mesh. A nil mesh is export.ErrNilMesh.
func Structure(fm *fmesh.FMesh) (Mesh, error) {
	if fm == nil {
		return Mesh{}, export.ErrNilMesh
	}
	b := &builder{}
	if err := fm.Walk(b); err != nil {
		return Mesh{}, err
	}
	return b.mesh, nil
}

// CycleOf returns the structure of fm with the results of c. A nil mesh is
// export.ErrNilMesh, and a nil cycle export.ErrNilCycle.
func CycleOf(fm *fmesh.FMesh, c *cycle.Cycle) (Cycle, error) {
	if c == nil {
		return Cycle{}, export.ErrNilCycle
	}
	mesh, err := Structure(fm)
	if err != nil {
		return Cycle{}, err
	}
	doc := Cycle{Number: c.Number(), Mesh: mesh, Results: make([]Result, 0, len(mesh.Components))}
	for _, comp := range mesh.Components {
		doc.Results = append(doc.Results, resultOf(comp.Name, c.ActivationResults().ByName(comp.Name)))
	}
	return doc, nil
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
