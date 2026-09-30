// Package dot provides [Plugin], a mesh plugin that exports an fmesh mesh as a
// Graphviz DOT graph: the static structure, and optionally one graph per cycle.
package dot

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"

	"github.com/emicklei/dot"
	"github.com/hovsep/fmesh"
	fmeshcomponent "github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/cycle"
	"github.com/hovsep/fmesh/port"
)

var (
	// ErrNotAttached is returned by an export before the plugin is attached to a mesh.
	ErrNotAttached = errors.New("dot: plugin is not attached to a mesh")

	// ErrCyclesNotRecorded is returned by ExportCycles when the plugin was created without WithCycles.
	ErrCyclesNotRecorded = errors.New("dot: cycles are not recorded, create the plugin with WithCycles")
)

// Plugin exports the mesh it is attached to as a Graphviz DOT graph. One
// instance serves one mesh.
type Plugin struct {
	style        *style
	recordCycles bool
	fm           *fmesh.FMesh
	cycles       []*cycle.Cycle
}

// Option configures a Plugin.
type Option func(*Plugin)

// WithAttrs sets Graphviz attributes on one element of the drawing, over the
// defaults: attributes it does not name keep their default values.
func WithAttrs(element Element, attrs map[string]string) Option {
	return func(p *Plugin) { merge(p.style.attrs, element, attrs) }
}

// WithResultAttrs sets attributes on a component's cluster in a cycle graph when
// its activation ended with code, over the defaults (the colors).
func WithResultAttrs(code fmeshcomponent.ActivationResultCode, attrs map[string]string) Option {
	return func(p *Plugin) { merge(p.style.resultAttrs, code, attrs) }
}

// WithComponentLabel sets the label of a component node whose component has no
// description (default "𝑓").
func WithComponentLabel(label string) Option {
	return func(p *Plugin) { p.style.componentLabel = label }
}

// WithCycles records every cycle of the latest run, for ExportCycles.
func WithCycles() Option {
	return func(p *Plugin) { p.recordCycles = true }
}

// New returns a plugin to attach with fmesh.WithPlugins.
func New(opts ...Option) *Plugin {
	p := &Plugin{style: defaultStyle()}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Name returns the plugin name.
func (p *Plugin) Name() string { return "dot" }

// Init attaches the plugin to the mesh and, with WithCycles, starts recording.
func (p *Plugin) Init(fm *fmesh.FMesh) error {
	if p.fm != nil && p.fm != fm {
		return errors.New("dot: plugin is already attached to another mesh")
	}
	p.fm = fm

	if p.recordCycles {
		fm.SetupHooks(func(h *fmesh.Hooks) {
			h.BeforeRun(func(context.Context, *fmesh.FMesh) error {
				p.cycles = nil
				return nil
			})
			// Recorded here rather than read from RuntimeInfo, so a cycles history
			// limit does not cut the replay short.
			h.AfterCycle(func(_ context.Context, cc *fmesh.CycleContext) error {
				p.cycles = append(p.cycles, cc.Cycle)
				return nil
			})
		})
	}
	return nil
}

// Export returns fm's structure as a DOT graph, for a mesh built without the
// plugin. opts style it as they would the plugin; WithCycles has no effect.
func Export(fm *fmesh.FMesh, opts ...Option) ([]byte, error) {
	p := New(opts...)
	p.fm = fm
	return p.Export()
}

// Export returns the mesh structure as a DOT graph. An empty mesh exports nothing.
func (p *Plugin) Export() ([]byte, error) {
	if p.fm == nil {
		return nil, ErrNotAttached
	}
	if p.fm.Components().IsEmpty() {
		return nil, nil
	}
	return p.render(nil)
}

// ExportCycles returns one DOT graph per cycle of the latest run, in order.
// Each graph colors components by their activation result and shows the cycle
// stats in the legend.
func (p *Plugin) ExportCycles() ([][]byte, error) {
	if p.fm == nil {
		return nil, ErrNotAttached
	}
	if !p.recordCycles {
		return nil, ErrCyclesNotRecorded
	}
	if p.fm.Components().IsEmpty() {
		return nil, nil
	}

	graphs := make([][]byte, 0, len(p.cycles))
	for _, c := range p.cycles {
		graph, err := p.render(c)
		if err != nil {
			return nil, fmt.Errorf("cycle %d: %w", c.Number(), err)
		}
		graphs = append(graphs, graph)
	}
	return graphs, nil
}

// render draws the mesh, optionally in the state of one cycle.
func (p *Plugin) render(activationCycle *cycle.Cycle) ([]byte, error) {
	b := &graphBuilder{style: p.style, cycle: activationCycle, ports: make(map[*port.Port]dot.Node)}
	if err := p.fm.Walk(b); err != nil {
		return nil, err
	}
	buf := new(bytes.Buffer)
	b.graph.Write(buf)
	return buf.Bytes(), nil
}

// graphBuilder draws a mesh from a walk. It keeps its own port-to-node map, so
// the mesh itself is never touched.
type graphBuilder struct {
	style *style
	cycle *cycle.Cycle

	graph         *dot.Graph
	subgraph      *dot.Graph // of the component visited last
	componentNode dot.Node   // of the component visited last
	ports         map[*port.Port]dot.Node
}

func (b *graphBuilder) VisitMesh(fm *fmesh.FMesh) error {
	b.graph = dot.NewGraph(dot.Directed)
	setAttrMap(&b.graph.AttributesMap, b.style.attrs[Graph])
	return b.addLegend(fm)
}

func (b *graphBuilder) VisitComponent(c *fmeshcomponent.Component) error {
	var result *fmeshcomponent.ActivationResult
	if b.cycle != nil {
		result = b.cycle.ActivationResults().ByName(c.Name())
	}

	b.subgraph = b.graph.Subgraph("id-subgraph-"+c.Name(), dot.ClusterOption{})
	b.subgraph.NodeInitializer(func(n dot.Node) {
		setAttrMap(&n.AttributesMap, b.style.attrs[ComponentNodes])
	})
	setAttrMap(&b.subgraph.AttributesMap, b.style.attrs[Component])
	if b.cycle != nil {
		setAttrMap(&b.subgraph.AttributesMap, b.style.resultAttrs[codeOf(result)])
	}
	b.subgraph.Label(c.Name())

	label := b.style.componentLabel
	if c.Description() != "" {
		label = c.Description()
	}
	b.componentNode = b.subgraph.Node("id-" + c.Name())
	setAttrMap(&b.componentNode.AttributesMap, b.style.attrs[ComponentNode])
	b.componentNode.Label(label).Attr("group", c.Name())

	if result != nil && result.ActivationError() != nil {
		errorNode := b.subgraph.Node("id-error-" + c.Name())
		setAttrMap(&errorNode.AttributesMap, b.style.attrs[ErrorNode])
		errorNode.Label(result.ActivationError().Error())
		b.subgraph.Edge(b.componentNode, errorNode)
	}
	return nil
}

func (b *graphBuilder) VisitPort(c *fmeshcomponent.Component, p *port.Port) error {
	node := b.subgraph.Node(portID(c.Name(), p)).Label(p.Name()).Attr("group", c.Name())
	setAttrMap(&node.AttributesMap, b.style.attrs[Port])
	b.ports[p] = node

	if p.IsInput() {
		b.subgraph.Edge(node, b.componentNode)
	} else {
		b.subgraph.Edge(b.componentNode, node)
	}
	return nil
}

func (b *graphBuilder) VisitPipe(from, to *port.Port) error {
	fromNode, ok := b.ports[from]
	if !ok {
		return fmt.Errorf("pipe source port %q is not in the mesh", from.Name())
	}
	toNode, ok := b.ports[to]
	if !ok {
		return fmt.Errorf("pipe destination port %q is not in the mesh", to.Name())
	}
	edge := b.graph.Edge(fromNode, toNode)
	setAttrMap(&edge.AttributesMap, b.style.attrs[Pipe])
	return nil
}

// addLegend adds the mesh description and, for a cycle, its number and stats.
func (b *graphBuilder) addLegend(fm *fmesh.FMesh) error {
	subgraph := b.graph.Subgraph("id-legend", dot.ClusterOption{})
	setAttrMap(&subgraph.AttributesMap, b.style.attrs[Legend])
	subgraph.Delete("label")

	data := map[string]any{
		"meshDescription": fmt.Sprintf("A mesh with %d components", fm.Components().Len()),
	}
	if fm.Description() != "" {
		data["meshDescription"] = fm.Description()
	}
	if b.cycle != nil {
		data["cycleNumber"] = b.cycle.Number()
		data["stats"] = cycleStats(fm, b.cycle)
	}

	legend := new(bytes.Buffer)
	if err := legendTemplate.Execute(legend, data); err != nil {
		return fmt.Errorf("failed to render legend: %w", err)
	}

	node := subgraph.Node("legend-subgraph")
	setAttrMap(&node.AttributesMap, b.style.attrs[LegendNode])
	node.Attr("label", dot.HTML(legend.String()))
	return nil
}

type statEntry struct {
	Name  string
	Value int
}

// statCodes lists every activation result code, in the order the legend shows them.
var statCodes = []fmeshcomponent.ActivationResultCode{
	fmeshcomponent.ActivationCodeOK,
	fmeshcomponent.ActivationCodeNoInput,
	fmeshcomponent.ActivationCodeReturnedError,
	fmeshcomponent.ActivationCodePanicked,
	fmeshcomponent.ActivationCodeHookFailed,
	fmeshcomponent.ActivationCodeWaitingForInputsClear,
	fmeshcomponent.ActivationCodeWaitingForInputsKeep,
}

// cycleStats counts results per code. A cycle records no result for a
// component without input, so those are counted as NoInput.
func cycleStats(fm *fmesh.FMesh, c *cycle.Cycle) []statEntry {
	counts := make(map[fmeshcomponent.ActivationResultCode]int, len(statCodes))
	activated := 0
	for _, comp := range fm.Components().AllOrdered() {
		result := c.ActivationResults().ByName(comp.Name())
		counts[codeOf(result)]++
		if result != nil && result.Activated() {
			activated++
		}
	}

	stats := make([]statEntry, 0, 1+len(statCodes))
	stats = append(stats, statEntry{Name: "Activated", Value: activated})
	for _, code := range statCodes {
		stats = append(stats, statEntry{Name: code.String(), Value: counts[code]})
	}
	return stats
}

// codeOf treats a missing result as NoInput, which is what it means.
func codeOf(result *fmeshcomponent.ActivationResult) fmeshcomponent.ActivationResultCode {
	if result == nil {
		return fmeshcomponent.ActivationCodeNoInput
	}
	return result.Code()
}

// portID is a node ID unique across the graph.
func portID(componentName string, p *port.Port) string {
	direction := "out"
	if p.IsInput() {
		direction = "in"
	}
	return fmt.Sprintf("component/%s/%s/%s", componentName, direction, p.Name())
}

// setAttrMap sets all attributes on target.
func setAttrMap(target *dot.AttributesMap, attributes attributesMap) {
	for name, value := range attributes {
		target.Attr(name, value)
	}
}

var legendTemplate = template.Must(template.New("legend").Parse(legendHTML))
