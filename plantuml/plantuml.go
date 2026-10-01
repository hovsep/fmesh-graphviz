// Package plantuml provides [Plugin], a mesh plugin that exports an fmesh mesh
// as a PlantUML component diagram: the static structure, and optionally one
// diagram per cycle with components colored by their activation result.
package plantuml

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/cycle"
	"github.com/hovsep/fmesh/port"
)

var (
	// ErrNotAttached is returned by an export before the plugin is attached to a mesh.
	ErrNotAttached = errors.New("plantuml: plugin is not attached to a mesh")

	// ErrCyclesNotRecorded is returned by ExportCycles when the plugin was created without WithCycles.
	ErrCyclesNotRecorded = errors.New("plantuml: cycles are not recorded, create the plugin with WithCycles")
)

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

// Plugin exports the mesh it is attached to as a PlantUML component diagram.
// One instance serves one mesh.
type Plugin struct {
	direction    string
	colors       map[component.ActivationResultCode]string
	recordCycles bool
	fm           *fmesh.FMesh
	cycles       []*cycle.Cycle
}

// Option configures a Plugin.
type Option func(*Plugin)

// WithDirection sets the layout direction: "left to right" (the default) or "top to bottom".
func WithDirection(direction string) Option {
	return func(p *Plugin) { p.direction = direction }
}

// WithResultColor sets the border color of a component in a cycle diagram when
// its activation ended with code. color is a PlantUML color name ("teal") or a
// hex code ("#00AA00"). Codes it does not name keep their default colors.
func WithResultColor(code component.ActivationResultCode, color string) Option {
	return func(p *Plugin) { p.colors[code] = color }
}

// WithCycles records every cycle of the latest run, for ExportCycles.
func WithCycles() Option {
	return func(p *Plugin) { p.recordCycles = true }
}

// New returns a plugin to attach with fmesh.WithPlugins.
func New(opts ...Option) *Plugin {
	p := &Plugin{direction: "left to right", colors: defaultColors()}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Name returns the plugin name.
func (p *Plugin) Name() string { return "plantuml" }

// Init attaches the plugin to the mesh and, with WithCycles, starts recording.
func (p *Plugin) Init(fm *fmesh.FMesh) error {
	if err := p.validate(); err != nil {
		return err
	}
	if p.fm == fm {
		return nil // already attached: the hooks are registered once
	}
	if p.fm != nil {
		return errors.New("plantuml: plugin is already attached to another mesh")
	}
	p.fm = fm

	if p.recordCycles {
		fm.SetupHooks(func(h *fmesh.Hooks) {
			h.BeforeRun(func(context.Context, *fmesh.FMesh) error {
				p.cycles = nil
				return nil
			})
			// Recorded here rather than read from RuntimeInfo, so a cycles history
			// limit does not cut the replay short. BeforeCycle gets the same cycle
			// the run then fills in, and unlike AfterCycle it cannot be skipped by
			// another plugin's failing AfterCycle hook.
			h.BeforeCycle(func(_ context.Context, cc *fmesh.CycleContext) error {
				p.cycles = append(p.cycles, cc.Cycle)
				return nil
			})
		})
	}
	return nil
}

func (p *Plugin) validate() error {
	switch p.direction {
	case "left to right", "top to bottom":
		return nil
	default:
		return fmt.Errorf("plantuml: unknown direction %q", p.direction)
	}
}

// Export returns fm's structure as PlantUML source, for a mesh built without the
// plugin. opts style it as they would the plugin; WithCycles has no effect.
func Export(fm *fmesh.FMesh, opts ...Option) ([]byte, error) {
	p := New(opts...)
	if err := p.validate(); err != nil {
		return nil, err
	}
	p.fm = fm
	return p.Export()
}

// Export returns the mesh structure as PlantUML source.
func (p *Plugin) Export() ([]byte, error) {
	if p.fm == nil {
		return nil, ErrNotAttached
	}
	return p.render(nil)
}

// ExportCycles returns one diagram per cycle of the latest run, in order, with
// components colored by their activation result.
func (p *Plugin) ExportCycles() ([][]byte, error) {
	if p.fm == nil {
		return nil, ErrNotAttached
	}
	if !p.recordCycles {
		return nil, ErrCyclesNotRecorded
	}
	diagrams := make([][]byte, 0, len(p.cycles))
	for _, c := range p.cycles {
		diagram, err := p.render(c)
		if err != nil {
			return nil, fmt.Errorf("cycle %d: %w", c.Number(), err)
		}
		diagrams = append(diagrams, diagram)
	}
	return diagrams, nil
}

func (p *Plugin) render(activationCycle *cycle.Cycle) ([]byte, error) {
	b := &diagramBuilder{plugin: p, cycle: activationCycle, ports: make(map[*port.Port]string)}
	if err := p.fm.Walk(b); err != nil {
		return nil, err
	}
	b.closeComponent()
	b.out.WriteString("@enduml\n")
	return []byte(b.out.String()), nil
}

// diagramBuilder writes a diagram from a walk. Aliases are generated in walk
// order, so they are stable and never depend on user-chosen names.
type diagramBuilder struct {
	plugin *Plugin
	cycle  *cycle.Cycle

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
	fmt.Fprintf(&b.out, "@startuml\ntitle %s\n%s direction\n", escape(title), b.plugin.direction)
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
		if color, ok := b.plugin.colors[code]; ok {
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
