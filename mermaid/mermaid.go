// Package mermaid provides [Plugin], a mesh plugin that exports an fmesh mesh
// as a Mermaid flowchart: the static structure, and optionally one chart per
// cycle with components colored by their activation result.
package mermaid

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
	ErrNotAttached = errors.New("mermaid: plugin is not attached to a mesh")

	// ErrCyclesNotRecorded is returned by ExportCycles when the plugin was created without WithCycles.
	ErrCyclesNotRecorded = errors.New("mermaid: cycles are not recorded, create the plugin with WithCycles")
)

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

// Plugin exports the mesh it is attached to as a Mermaid flowchart. One
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

// WithDirection sets the flowchart direction: "LR" (the default), "RL", "TB" or "BT".
func WithDirection(direction string) Option {
	return func(p *Plugin) { p.direction = direction }
}

// WithResultColor sets the stroke color of a component in a cycle chart when its
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
	p := &Plugin{direction: "LR", colors: defaultColors()}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Name returns the plugin name.
func (p *Plugin) Name() string { return "mermaid" }

// Init attaches the plugin to the mesh and, with WithCycles, starts recording.
func (p *Plugin) Init(fm *fmesh.FMesh) error {
	if err := p.validate(); err != nil {
		return err
	}
	if p.fm == fm {
		return nil // already attached: the hooks are registered once
	}
	if p.fm != nil {
		return errors.New("mermaid: plugin is already attached to another mesh")
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

// Export returns fm's structure as Mermaid flowchart source, for a mesh built without the
// plugin. opts style it as they would the plugin; WithCycles has no effect.
func Export(fm *fmesh.FMesh, opts ...Option) ([]byte, error) {
	p := New(opts...)
	if err := p.validate(); err != nil {
		return nil, err
	}
	p.fm = fm
	return p.Export()
}

// validate rejects options that would produce an invalid chart.
func (p *Plugin) validate() error {
	switch p.direction {
	case "LR", "RL", "TB", "BT":
		return nil
	default:
		return fmt.Errorf("mermaid: unknown direction %q", p.direction)
	}
}

// Export returns the mesh structure as Mermaid flowchart source.
func (p *Plugin) Export() ([]byte, error) {
	if p.fm == nil {
		return nil, ErrNotAttached
	}
	return p.render(nil)
}

// ExportCycles returns one chart per cycle of the latest run, in order, with
// components colored by their activation result.
func (p *Plugin) ExportCycles() ([][]byte, error) {
	if p.fm == nil {
		return nil, ErrNotAttached
	}
	if !p.recordCycles {
		return nil, ErrCyclesNotRecorded
	}
	charts := make([][]byte, 0, len(p.cycles))
	for _, c := range p.cycles {
		chart, err := p.render(c)
		if err != nil {
			return nil, fmt.Errorf("cycle %d: %w", c.Number(), err)
		}
		charts = append(charts, chart)
	}
	return charts, nil
}

func (p *Plugin) render(activationCycle *cycle.Cycle) ([]byte, error) {
	b := &chartBuilder{plugin: p, cycle: activationCycle, ports: make(map[*port.Port]string)}
	if err := p.fm.Walk(b); err != nil {
		return nil, err
	}
	b.closeSubgraph()
	return []byte(b.out.String()), nil
}

// chartBuilder writes a flowchart from a walk. Node IDs are generated in walk
// order, so they are stable and never depend on user-chosen names.
type chartBuilder struct {
	plugin *Plugin
	cycle  *cycle.Cycle

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
	fmt.Fprintf(&b.out, "---\ntitle: %s\n---\nflowchart %s\n", quote(title), b.plugin.direction)
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
		if color, ok := b.plugin.colors[code]; ok {
			fmt.Fprintf(&b.out, "    style %s stroke:%s,stroke-width:3px\n", subgraph, color)
		}
		if result != nil && result.ActivationError() != nil {
			errorNode := b.nextID("e")
			fmt.Fprintf(&b.out, "    %s -.- %s>%s]\n", b.component, errorNode, quote(result.ActivationError().Error()))
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
