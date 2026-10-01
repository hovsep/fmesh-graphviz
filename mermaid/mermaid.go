// Package mermaid provides [Exporter], which exports an fmesh mesh as a
// Mermaid flowchart: the structure, or the structure in the state of one cycle
// with components colored by their activation result.
package mermaid

import (
	"fmt"
	"strings"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-export"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/cycle"
	"github.com/hovsep/fmesh/port"
)

var _ export.Exporter = (*Exporter)(nil)

// defaultColors returns a fresh copy of the default stroke colors of a
// component in a cycle chart, by activation result.
func defaultColors() map[component.ActivationResultCode]string {
	return map[component.ActivationResultCode]string{
		component.ActivationCodeOK:                    "green",
		component.ActivationCodeNoInput:               "gold",
		component.ActivationCodeReturnedError:         "red",
		component.ActivationCodePanicked:              "hotpink",
		component.ActivationCodeHookFailed:            "orange",
		component.ActivationCodeWaitingForInputsClear: "blue",
		component.ActivationCodeWaitingForInputsKeep:  "purple",
	}
}

// Exporter exports a mesh as a Mermaid flowchart. It holds only its options,
// so one value can export many meshes, also from an AfterCycle hook.
type Exporter struct {
	direction string
	colors    map[component.ActivationResultCode]string
}

// Option configures an Exporter.
type Option func(*Exporter)

// WithDirection sets the flowchart direction: "LR" (the default), "RL", "TB" or "BT".
func WithDirection(direction string) Option {
	return func(e *Exporter) { e.direction = direction }
}

// WithResultColor sets the stroke color of a component in a cycle chart when its
// activation ended with code. Codes it does not name keep their default colors.
func WithResultColor(code component.ActivationResultCode, color string) Option {
	return func(e *Exporter) { e.colors[code] = color }
}

// New returns an exporter with the given options. Export and ExportCycle
// report an invalid option.
func New(opts ...Option) *Exporter {
	e := &Exporter{direction: "LR", colors: defaultColors()}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Export returns the mesh structure as Mermaid flowchart source.
func (e *Exporter) Export(fm *fmesh.FMesh) ([]byte, error) {
	return e.render(fm, nil)
}

// ExportCycle returns the mesh as Mermaid flowchart source in the state of
// cycle c, with components colored by their activation result. A component
// with no result in c had no input.
func (e *Exporter) ExportCycle(fm *fmesh.FMesh, c *cycle.Cycle) ([]byte, error) {
	if c == nil {
		return nil, export.ErrNilCycle
	}
	return e.render(fm, c)
}

// validate rejects options that would produce an invalid chart.
func (e *Exporter) validate() error {
	switch e.direction {
	case "LR", "RL", "TB", "BT":
		return nil
	default:
		return fmt.Errorf("mermaid: unknown direction %q", e.direction)
	}
}

func (e *Exporter) render(fm *fmesh.FMesh, activationCycle *cycle.Cycle) ([]byte, error) {
	if err := e.validate(); err != nil {
		return nil, err
	}
	b := &chartBuilder{exporter: e, cycle: activationCycle, ports: make(map[*port.Port]string)}
	if err := fm.Walk(b); err != nil {
		return nil, err
	}
	b.closeSubgraph()
	return []byte(b.out.String()), nil
}

// chartBuilder writes a flowchart from a walk. Node IDs are generated in walk
// order, so they are stable and never depend on user-chosen names.
type chartBuilder struct {
	exporter *Exporter
	cycle    *cycle.Cycle

	out          strings.Builder
	nodes        int
	component    string // node ID of the component visited last
	subgraphOpen bool
	ports        map[*port.Port]string
}

func (b *chartBuilder) nextID(prefix string) string {
	b.nodes++
	return fmt.Sprintf("%s%d", prefix, b.nodes)
}

func (b *chartBuilder) VisitMesh(fm *fmesh.FMesh) error {
	title := fm.Name()
	if b.cycle != nil {
		title = fmt.Sprintf("%s — cycle %d", title, b.cycle.Number())
	}
	fmt.Fprintf(&b.out, "---\ntitle: %s\n---\nflowchart %s\n", quote(title), b.exporter.direction)
	if fm.Description() != "" {
		fmt.Fprintf(&b.out, "  %%%% %s\n", oneLine(fm.Description()))
	}
	return nil
}

func (b *chartBuilder) VisitComponent(c *component.Component) error {
	b.closeSubgraph()
	subgraph := b.nextID("s")
	b.component = b.nextID("c")

	label := c.Name()
	if c.Description() != "" {
		label = c.Description()
	}
	fmt.Fprintf(&b.out, "  subgraph %s[%s]\n", subgraph, quote(c.Name()))
	fmt.Fprintf(&b.out, "    %s[%s]\n", b.component, quote(label))
	b.subgraphOpen = true

	if b.cycle != nil {
		result := b.cycle.ActivationResults().ByName(c.Name())
		code := component.ActivationCodeNoInput // no result means the component had no input
		if result != nil {
			code = result.Code()
		}
		if color, ok := b.exporter.colors[code]; ok {
			fmt.Fprintf(&b.out, "    style %s stroke:%s,stroke-width:3px\n", subgraph, color)
		}
		if result != nil && result.ActivationError() != nil {
			errorNode := b.nextID("e")
			fmt.Fprintf(&b.out, "    %s -.- %s>%s]\n", b.component, errorNode, quote(errorText(result)))
		}
	}
	return nil
}

func (b *chartBuilder) VisitPort(_ *component.Component, p *port.Port) error {
	id := b.nextID("p")
	b.ports[p] = id
	fmt.Fprintf(&b.out, "    %s((%s))\n", id, quote(p.Name()))
	if p.IsInput() {
		fmt.Fprintf(&b.out, "    %s --> %s\n", id, b.component)
	} else {
		fmt.Fprintf(&b.out, "    %s --> %s\n", b.component, id)
	}
	return nil
}

func (b *chartBuilder) VisitPipe(from, to *port.Port) error {
	b.closeSubgraph()
	fromID, ok := b.ports[from]
	if !ok {
		return fmt.Errorf("pipe source port %q is not in the mesh", from.Name())
	}
	toID, ok := b.ports[to]
	if !ok {
		return fmt.Errorf("pipe destination port %q is not in the mesh", to.Name())
	}
	fmt.Fprintf(&b.out, "  %s ==> %s\n", fromID, toID)
	return nil
}

func (b *chartBuilder) closeSubgraph() {
	if b.subgraphOpen {
		b.out.WriteString("  end\n")
		b.subgraphOpen = false
	}
}

// quote makes a Mermaid string label; a raw double quote would end it early.
func quote(s string) string {
	return `"` + strings.ReplaceAll(oneLine(s), `"`, "#quot;") + `"`
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// errorText joins a result's activation errors with "; ". The label is one
// line, so the newlines of ActivationError would leave "e1 e2" ambiguous.
func errorText(result *component.ActivationResult) string {
	msgs := make([]string, 0, len(result.ActivationErrors()))
	for _, err := range result.ActivationErrors() {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}
