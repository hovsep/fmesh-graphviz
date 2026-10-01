// Package dot provides [Exporter], which exports an fmesh mesh as a Graphviz
// DOT graph: the structure, or the structure in the state of one cycle.
package dot

import (
	"bytes"
	"fmt"
	"html/template"

	"github.com/emicklei/dot"
	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-export"
	fmeshcomponent "github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/cycle"
	"github.com/hovsep/fmesh/port"
)

var _ export.Exporter = (*Exporter)(nil)

// Exporter exports a mesh as a Graphviz DOT graph. It holds only its options,
// so one value can export many meshes, also from an AfterCycle hook.
type Exporter struct {
	style *style
}

// Option configures an Exporter.
type Option func(*Exporter)

// WithAttrs sets Graphviz attributes on one element of the drawing, over the
// defaults: attributes it does not name keep their default values.
func WithAttrs(element Element, attrs map[string]string) Option {
	return func(e *Exporter) { merge(e.style.attrs, element, attrs) }
}

// WithResultAttrs sets attributes on a component's cluster in a cycle graph when
// its activation ended with code, over the defaults (the colors).
func WithResultAttrs(code fmeshcomponent.ActivationResultCode, attrs map[string]string) Option {
	return func(e *Exporter) { merge(e.style.resultAttrs, code, attrs) }
}

// WithComponentLabel sets the label of a component node whose component has no
// description (default "𝑓").
func WithComponentLabel(label string) Option {
	return func(e *Exporter) { e.style.componentLabel = label }
}

// New returns an exporter with the given options.
func New(opts ...Option) *Exporter {
	e := &Exporter{style: defaultStyle()}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Export returns the mesh structure as a DOT graph. An empty mesh exports nothing.
func (e *Exporter) Export(fm *fmesh.FMesh) ([]byte, error) {
	return e.render(fm, nil)
}

// ExportCycle returns the mesh as a DOT graph in the state of cycle c:
// components are colored by their activation result and the legend shows the
// cycle stats. A component with no result in c had no input. An empty mesh
// exports nothing.
func (e *Exporter) ExportCycle(fm *fmesh.FMesh, c *cycle.Cycle) ([]byte, error) {
	if c == nil {
		return nil, export.ErrNilCycle
	}
	return e.render(fm, c)
}

// render draws the mesh, optionally in the state of one cycle.
func (e *Exporter) render(fm *fmesh.FMesh, activationCycle *cycle.Cycle) ([]byte, error) {
	if fm.Components().IsEmpty() {
		return nil, nil
	}
	b := &graphBuilder{style: e.style, cycle: activationCycle, ports: make(map[*port.Port]dot.Node)}
	if err := fm.Walk(b); err != nil {
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
		data["hasCycle"] = true // the number alone hides cycle 0
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
