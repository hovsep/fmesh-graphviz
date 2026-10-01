// Package plantuml provides [Exporter], which exports an fmesh mesh as a
// PlantUML component diagram: the structure, or the structure in the state of
// one cycle with components colored by their activation result.
package plantuml

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

// defaultColors returns a fresh copy of the default border colors of a
// component in a cycle diagram, by activation result.
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

// Exporter exports a mesh as a PlantUML component diagram. It holds only its
// options, so one value can export many meshes, also from an AfterCycle hook.
type Exporter struct {
	direction string
	colors    map[component.ActivationResultCode]string
}

// Option configures an Exporter.
type Option func(*Exporter)

// WithDirection sets the layout direction: "left to right" (the default) or "top to bottom".
func WithDirection(direction string) Option {
	return func(e *Exporter) { e.direction = direction }
}

// WithResultColor sets the border color of a component in a cycle diagram when
// its activation ended with code. color is a PlantUML color name ("teal") or a
// hex code ("#00AA00"). Codes it does not name keep their default colors.
func WithResultColor(code component.ActivationResultCode, color string) Option {
	return func(e *Exporter) { e.colors[code] = color }
}

// New returns an exporter with the given options. Export and ExportCycle
// report an invalid option.
func New(opts ...Option) *Exporter {
	e := &Exporter{direction: "left to right", colors: defaultColors()}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Export returns the mesh structure as PlantUML source.
func (e *Exporter) Export(fm *fmesh.FMesh) ([]byte, error) {
	return e.render(fm, nil)
}

// ExportCycle returns the mesh as PlantUML source in the state of cycle c,
// with components colored by their activation result. A component with no
// result in c had no input.
func (e *Exporter) ExportCycle(fm *fmesh.FMesh, c *cycle.Cycle) ([]byte, error) {
	if c == nil {
		return nil, export.ErrNilCycle
	}
	return e.render(fm, c)
}

// validate rejects options that would produce an invalid diagram.
func (e *Exporter) validate() error {
	switch e.direction {
	case "left to right", "top to bottom":
		return nil
	default:
		return fmt.Errorf("plantuml: unknown direction %q", e.direction)
	}
}

func (e *Exporter) render(fm *fmesh.FMesh, activationCycle *cycle.Cycle) ([]byte, error) {
	if fm == nil {
		return nil, export.ErrNilMesh
	}
	if err := e.validate(); err != nil {
		return nil, err
	}
	b := &diagramBuilder{exporter: e, cycle: activationCycle, ports: make(map[*port.Port]string)}
	if err := fm.Walk(b); err != nil {
		return nil, err
	}
	b.closeComponent()
	b.out.WriteString("@enduml\n")
	return []byte(b.out.String()), nil
}

// diagramBuilder writes a diagram from a walk. Aliases are generated in walk
// order, so they are stable and never depend on user-chosen names.
type diagramBuilder struct {
	exporter *Exporter
	cycle    *cycle.Cycle

	out       strings.Builder
	nodes     int
	component string // alias of the open component block, "" when none is open
	note      string // error note of the open component, written after its block
	ports     map[*port.Port]string
}

func (b *diagramBuilder) nextID(prefix string) string {
	b.nodes++
	return fmt.Sprintf("%s%d", prefix, b.nodes)
}

func (b *diagramBuilder) VisitMesh(fm *fmesh.FMesh) error {
	title := fm.Name()
	if b.cycle != nil {
		title = fmt.Sprintf("%s — cycle %d", title, b.cycle.Number())
	}
	fmt.Fprintf(&b.out, "@startuml\ntitle %s\n%s direction\n", escape(title), b.exporter.direction)
	if fm.Description() != "" {
		fmt.Fprintf(&b.out, "' %s\n", oneLine(fm.Description()))
	}
	return nil
}

func (b *diagramBuilder) VisitComponent(c *component.Component) error {
	b.closeComponent()
	b.component = b.nextID("c")

	label := escape(c.Name())
	if c.Description() != "" {
		label += `\n` + escape(c.Description())
	}
	style := ""
	if b.cycle != nil {
		result := b.cycle.ActivationResults().ByName(c.Name())
		code := component.ActivationCodeNoInput // no result means the component had no input
		if result != nil {
			code = result.Code()
		}
		if color, ok := b.exporter.colors[code]; ok {
			// PlantUML takes a hex line color without its leading "#".
			style = fmt.Sprintf(" #line:%s;line.bold", strings.TrimPrefix(color, "#"))
		}
		if result != nil && result.ActivationError() != nil {
			b.note = escape(result.ActivationError().Error())
		}
	}
	fmt.Fprintf(&b.out, "component \"%s\" as %s%s {\n", label, b.component, style)
	return nil
}

func (b *diagramBuilder) VisitPort(_ *component.Component, p *port.Port) error {
	id := b.nextID("p")
	b.ports[p] = id
	kind := "portout"
	if p.IsInput() {
		kind = "portin"
	}
	fmt.Fprintf(&b.out, "  %s \"%s\" as %s\n", kind, escape(p.Name()), id)
	return nil
}

func (b *diagramBuilder) VisitPipe(from, to *port.Port) error {
	b.closeComponent()
	fromID, ok := b.ports[from]
	if !ok {
		return fmt.Errorf("pipe source port %q is not in the mesh", from.Name())
	}
	toID, ok := b.ports[to]
	if !ok {
		return fmt.Errorf("pipe destination port %q is not in the mesh", to.Name())
	}
	fmt.Fprintf(&b.out, "%s --> %s\n", fromID, toID)
	return nil
}

// closeComponent ends the open component block and writes its error note.
func (b *diagramBuilder) closeComponent() {
	if b.component == "" {
		return
	}
	b.out.WriteString("}\n")
	if b.note != "" {
		fmt.Fprintf(&b.out, "note bottom of %s : %s\n", b.component, b.note)
	}
	b.component, b.note = "", ""
}

// escaper keeps text literal: a raw quote ends a name, a backslash starts an
// escape like \n, and "<" starts markup. Unicode escapes print the character.
var escaper = strings.NewReplacer(`\`, "<U+005C>", `"`, "<U+0022>", "<", "<U+003C>")

// escape makes text safe for a name, title or note; the result is one line.
func escape(s string) string {
	return escaper.Replace(oneLine(s))
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
