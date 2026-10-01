// Package d2 provides [Exporter], which exports an fmesh mesh as D2 diagram
// source: the structure, or the structure in the state of one cycle with
// components colored by their activation result.
package d2

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

// Exporter exports a mesh as D2 diagram source. It holds only its options,
// so one value can export many meshes, also from an AfterCycle hook.
type Exporter struct {
	direction string
	colors    map[component.ActivationResultCode]string
}

// Option configures an Exporter.
type Option func(*Exporter)

// WithDirection sets the diagram direction: "right" (the default), "left", "down" or "up".
func WithDirection(direction string) Option {
	return func(e *Exporter) { e.direction = direction }
}

// WithResultColor sets the stroke color of a component in a cycle diagram when its
// activation ended with code. Codes it does not name keep their default colors.
func WithResultColor(code component.ActivationResultCode, color string) Option {
	return func(e *Exporter) { e.colors[code] = color }
}

// New returns an exporter with the given options. Export and ExportCycle
// report an invalid option.
func New(opts ...Option) *Exporter {
	e := &Exporter{direction: "right", colors: defaultColors()}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Export returns the mesh structure as D2 source.
func (e *Exporter) Export(fm *fmesh.FMesh) ([]byte, error) {
	return e.render(fm, nil)
}

// ExportCycle returns the mesh as D2 source in the state of cycle c, with
// components colored by their activation result. A component with no result
// in c had no input.
func (e *Exporter) ExportCycle(fm *fmesh.FMesh, c *cycle.Cycle) ([]byte, error) {
	if c == nil {
		return nil, export.ErrNilCycle
	}
	return e.render(fm, c)
}

// validate rejects options that would produce an invalid diagram.
func (e *Exporter) validate() error {
	switch e.direction {
	case "up", "down", "left", "right":
		return nil
	default:
		return fmt.Errorf("d2: unknown direction %q", e.direction)
	}
}

func (e *Exporter) render(fm *fmesh.FMesh, activationCycle *cycle.Cycle) ([]byte, error) {
	if err := e.validate(); err != nil {
		return nil, err
	}
	b := &diagramBuilder{exporter: e, cycle: activationCycle, ports: make(map[*port.Port]string)}
	if err := fm.Walk(b); err != nil {
		return nil, err
	}
	b.closeContainer()
	return []byte(b.out.String()), nil
}

// diagramBuilder writes D2 source from a walk. Keys are generated in walk
// order, so they are stable and never depend on user-chosen names, which only
// appear as quoted labels.
type diagramBuilder struct {
	exporter *Exporter
	cycle    *cycle.Cycle

	out           strings.Builder
	nodes         int
	component     string // key of the component visited last
	containerOpen bool
	ports         map[*port.Port]string // port -> "component.port" key path
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
	if fm.Description() != "" {
		fmt.Fprintf(&b.out, "# %s\n", oneLine(fm.Description()))
	}
	fmt.Fprintf(&b.out, "direction: %s\n", b.exporter.direction)
	fmt.Fprintf(&b.out, "title: %s {\n  near: top-center\n  shape: text\n  style.font-size: 24\n}\n", quote(title))
	return nil
}

func (b *diagramBuilder) VisitComponent(c *component.Component) error {
	b.closeContainer()
	b.component = b.nextID("c")

	label := c.Name()
	if c.Description() != "" {
		label += "\n" + oneLine(c.Description())
	}
	fmt.Fprintf(&b.out, "%s: %s {\n", b.component, quote(label))
	b.containerOpen = true

	if b.cycle != nil {
		result := b.cycle.ActivationResults().ByName(c.Name())
		code := component.ActivationCodeNoInput // no result means the component had no input
		if result != nil {
			code = result.Code()
		}
		if color, ok := b.exporter.colors[code]; ok {
			fmt.Fprintf(&b.out, "  style.stroke: %s\n  style.stroke-width: 3\n", quote(color))
		}
		if result != nil && result.ActivationError() != nil {
			fmt.Fprintf(&b.out, "  %s: %s {shape: callout}\n", b.nextID("e"), quote(result.ActivationError().Error()))
		}
	}
	return nil
}

func (b *diagramBuilder) VisitPort(_ *component.Component, p *port.Port) error {
	id := b.nextID("p")
	b.ports[p] = b.component + "." + id
	fmt.Fprintf(&b.out, "  %s: %s {shape: oval}\n", id, quote(p.Name()))
	return nil
}

func (b *diagramBuilder) VisitPipe(from, to *port.Port) error {
	b.closeContainer()
	fromKey, ok := b.ports[from]
	if !ok {
		return fmt.Errorf("pipe source port %q is not in the mesh", from.Name())
	}
	toKey, ok := b.ports[to]
	if !ok {
		return fmt.Errorf("pipe destination port %q is not in the mesh", to.Name())
	}
	fmt.Fprintf(&b.out, "%s -> %s\n", fromKey, toKey)
	return nil
}

func (b *diagramBuilder) closeContainer() {
	if b.containerOpen {
		b.out.WriteString("}\n")
		b.containerOpen = false
	}
}

// quoter escapes the characters that end or alter a D2 double-quoted string:
// a backslash, a double quote, and "$", which starts a substitution.
var quoter = strings.NewReplacer(`\`, `\\`, `"`, `\"`, `$`, `\$`, "\n", `\n`)

// quote makes a D2 double-quoted string. Whitespace runs in each line
// collapse to one space; a newline in s stays a line break in the label.
func quote(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = oneLine(line)
	}
	return `"` + quoter.Replace(strings.Join(lines, "\n")) + `"`
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
