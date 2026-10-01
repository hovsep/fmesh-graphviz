// Package d2 provides [Plugin], a mesh plugin that exports an fmesh mesh as
// D2 diagram source: the static structure, and optionally one diagram per
// cycle with components colored by their activation result.
package d2

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
	ErrNotAttached = errors.New("d2: plugin is not attached to a mesh")

	// ErrCyclesNotRecorded is returned by ExportCycles when the plugin was created without WithCycles.
	ErrCyclesNotRecorded = errors.New("d2: cycles are not recorded, create the plugin with WithCycles")
)

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

// Plugin exports the mesh it is attached to as D2 diagram source. One
// instance serves one mesh.
type Plugin struct {
	direction    string
	colors       map[component.ActivationResultCode]string
	recordCycles bool
	fm           *fmesh.FMesh
	cycles       []*cycle.Cycle
}

// Option configures a Plugin.
type Option func(*Plugin)

// WithDirection sets the diagram direction: "right" (the default), "left", "down" or "up".
func WithDirection(direction string) Option {
	return func(p *Plugin) { p.direction = direction }
}

// WithResultColor sets the stroke color of a component in a cycle diagram when its
// activation ended with code. Codes it does not name keep their default colors.
func WithResultColor(code component.ActivationResultCode, color string) Option {
	return func(p *Plugin) { p.colors[code] = color }
}

// WithCycles records every cycle of the latest run, for ExportCycles.
func WithCycles() Option {
	return func(p *Plugin) { p.recordCycles = true }
}

// New returns a plugin to attach with fmesh.WithPlugins.
func New(opts ...Option) *Plugin {
	p := &Plugin{direction: "right", colors: defaultColors()}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Name returns the plugin name.
func (p *Plugin) Name() string { return "d2" }

// Init attaches the plugin to the mesh and, with WithCycles, starts recording.
func (p *Plugin) Init(fm *fmesh.FMesh) error {
	if err := p.validate(); err != nil {
		return err
	}
	if p.fm == fm {
		return nil // already attached: the hooks are registered once
	}
	if p.fm != nil {
		return errors.New("d2: plugin is already attached to another mesh")
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
	case "up", "down", "left", "right":
		return nil
	default:
		return fmt.Errorf("d2: unknown direction %q", p.direction)
	}
}

// Export returns fm's structure as D2 source, for a mesh built without the
// plugin. opts style it as they would the plugin; WithCycles has no effect.
func Export(fm *fmesh.FMesh, opts ...Option) ([]byte, error) {
	p := New(opts...)
	if err := p.validate(); err != nil {
		return nil, err
	}
	p.fm = fm
	return p.Export()
}

// Export returns the mesh structure as D2 source.
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
	b.closeContainer()
	return []byte(b.out.String()), nil
}

// diagramBuilder writes D2 source from a walk. Keys are generated in walk
// order, so they are stable and never depend on user-chosen names, which only
// appear as quoted labels.
type diagramBuilder struct {
	plugin *Plugin
	cycle  *cycle.Cycle

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
	fmt.Fprintf(&b.out, "direction: %s\n", b.plugin.direction)
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
		if color, ok := b.plugin.colors[code]; ok {
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
