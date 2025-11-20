package dot

import (
	"bytes"
	"fmt"
	"html/template"
	"sort"

	"github.com/emicklei/dot"
	"github.com/hovsep/fmesh"
	fmeshcomponent "github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/cycle"
	"github.com/hovsep/fmesh/port"
)

type statEntry struct {
	Name  string
	Value int
}

// Exporter implements the graphviz.Exporter interface.
type Exporter struct {
	config *Config
}

// NewDotExporter returns exporter with default configuration.
func NewDotExporter() *Exporter {
	return NewDotExporterWithConfig(defaultConfig)
}

// NewDotExporterWithConfig returns exporter with custom configuration.
func NewDotExporterWithConfig(config *Config) *Exporter {
	return &Exporter{
		config: config,
	}
}

// Export returns the f-mesh as DOT-graph.
func (d *Exporter) Export(fm *fmesh.FMesh) ([]byte, error) {
	if fm.Components().Len() == 0 {
		return nil, nil
	}

	graph, err := d.buildGraph(fm, nil)

	if err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)
	graph.Write(buf)

	return buf.Bytes(), nil
}

// ExportWithCycles returns multiple graphs showing the state of the given f-mesh in each activation cycle.
func (d *Exporter) ExportWithCycles(fm *fmesh.FMesh, activationCycles *cycle.Group) ([][]byte, error) {
	if fm.Components().Len() == 0 {
		return nil, nil
	}

	if activationCycles.IsEmpty() {
		return nil, nil
	}

	results := make([][]byte, activationCycles.Len())

	activationCycles.ForEach(func(cycle *cycle.Cycle) error {
		graphForCycle, err := d.buildGraph(fm, cycle)
		if err != nil {
			return err
		}

		buf := new(bytes.Buffer)
		graphForCycle.Write(buf)

		results[cycle.Number()-1] = buf.Bytes()
		return nil
	})

	if activationCycles.HasChainableErr() {
		return nil, activationCycles.ChainableErr()
	}

	return results, nil
}

// buildGraph returns f-mesh as a graph
// activationCycle may be passed optionally to get a representation of f-mesh in a given activation cycle.
func (d *Exporter) buildGraph(fm *fmesh.FMesh, activationCycle *cycle.Cycle) (*dot.Graph, error) {
	mainGraph, err := d.getMainGraph(fm, activationCycle)
	if err != nil {
		return nil, fmt.Errorf("failed to get main graph: %w", err)
	}

	err = d.addComponents(mainGraph, fm.Components(), activationCycle)
	if err != nil {
		return nil, fmt.Errorf("failed to add components: %w", err)
	}

	err = d.addPipes(mainGraph, fm.Components())
	if err != nil {
		return nil, fmt.Errorf("failed to add pipes: %w", err)
	}
	return mainGraph, nil
}

// getMainGraph creates and returns the main (root) graph.
func (d *Exporter) getMainGraph(fm *fmesh.FMesh, activationCycle *cycle.Cycle) (*dot.Graph, error) {
	graph := dot.NewGraph(dot.Directed)

	setAttrMap(&graph.AttributesMap, d.config.MainGraph)

	err := d.addLegend(graph, fm, activationCycle)
	if err != nil {
		return nil, fmt.Errorf("failed to build main graph: %w", err)
	}

	return graph, nil
}

// addPipes adds pipes representation to the graph.
func (d *Exporter) addPipes(graph *dot.Graph, components *fmeshcomponent.Collection) error {
	components.ForEach(func(c *fmeshcomponent.Component) error {

		c.Outputs().ForEach(func(srcPort *port.Port) error {

			srcPort.Pipes().ForEach(func(destPort *port.Port) error {
				// Any source port in any pipe is always an output port, so we can build its node ID
				srcPortNode, ok := graph.FindNodeById(getPortID(c.Name(), port.DirectionOut, srcPort.Name()))
				if !ok {
					return fmt.Errorf("source port %s node not found in graph", srcPort.Name())
				}

				// Any destination port in any pipe is always an input port, so we can build its node ID
				destPortNode, ok := graph.FindNodeById(getPortID(destPort.ParentComponent().Name(), port.DirectionIn, destPort.Name()))
				if !ok {
					return fmt.Errorf("destination port node %s not found in graph", destPort.Name())
				}

				e := graph.Edge(srcPortNode, destPortNode)
				setAttrMap(&e.AttributesMap, d.config.Pipe.Edge)
				return nil
			})

			return nil
		})
		return nil
	})
	return nil
}

// addComponents adds components representation to the graph.
func (d *Exporter) addComponents(graph *dot.Graph, components *fmeshcomponent.Collection, activationCycle *cycle.Cycle) error {
	components.ForEach(func(c *fmeshcomponent.Component) error {
		// Component
		var activationResult *fmeshcomponent.ActivationResult
		if activationCycle != nil {
			activationResult = activationCycle.ActivationResults().ByName(c.Name())
		}
		componentSubgraph := d.getComponentSubgraph(graph, c, activationResult)
		componentNode := d.getComponentNode(componentSubgraph, c, activationResult)

		// Input ports
		c.Inputs().ForEach(func(p *port.Port) error {
			portNode := d.getPortNode(c, p, componentSubgraph)
			componentSubgraph.Edge(*portNode, *componentNode)
			return nil
		})

		// Output ports
		c.Outputs().ForEach(func(p *port.Port) error {
			portNode := d.getPortNode(c, p, componentSubgraph)
			componentSubgraph.Edge(*componentNode, *portNode)
			return nil
		})

		return nil
	})

	return components.ChainableErr()
}

// getPortNode creates and returns a node representing one port.
func (d *Exporter) getPortNode(c *fmeshcomponent.Component, p *port.Port, componentSubgraph *dot.Graph) *dot.Node {
	portID := getPortID(c.Name(), p.Direction(), p.Name())

	portNode := componentSubgraph.Node(portID).Label(p.Name()).Attr("group", c.Name())
	setAttrMap(&portNode.AttributesMap, d.config.Port.Node)

	return &portNode
}

// getComponentSubgraph creates a component subgraph and returns it.
func (d *Exporter) getComponentSubgraph(graph *dot.Graph, component *fmeshcomponent.Component, activationResult *fmeshcomponent.ActivationResult) *dot.Graph {
	componentSubgraph := graph.Subgraph("id-subgraph-"+component.Name(), dot.ClusterOption{})
	componentSubgraph.NodeInitializer(func(n dot.Node) {
		setAttrMap(&n.AttributesMap, d.config.Component.SubgraphNodeBaseAttrs)
	})

	setAttrMap(&componentSubgraph.AttributesMap, d.config.Component.Subgraph)

	// Set cycle-specific attributes
	if activationResult != nil {
		if attributesByCode, ok := d.config.Component.SubgraphAttributesByActivationResultCode[activationResult.Code()]; ok {
			setAttrMap(&componentSubgraph.AttributesMap, attributesByCode)
		}
	}

	componentSubgraph.Label(component.Name())

	return componentSubgraph
}

// getComponentNode creates component node and returns it.
func (d *Exporter) getComponentNode(componentSubgraph *dot.Graph, component *fmeshcomponent.Component, activationResult *fmeshcomponent.ActivationResult) *dot.Node {
	componentNode := componentSubgraph.Node("id-" + component.Name())
	setAttrMap(&componentNode.AttributesMap, d.config.Component.Node)

	label := d.config.Component.NodeDefaultLabel

	if component.Description() != "" {
		label = component.Description()
	}

	if activationResult != nil {
		if activationResult.ActivationError() != nil {
			errorNode := componentSubgraph.Node("id-error-" + activationResult.ComponentName())
			setAttrMap(&errorNode.AttributesMap, d.config.Component.ErrorNode)
			errorNode.Label(activationResult.ActivationError().Error())
			componentSubgraph.Edge(componentNode, errorNode)
		}
	}

	componentNode.
		Label(label).
		Attr("group", component.Name())
	return &componentNode
}

// addLegend adds useful information about f-mesh and (optionally) current activation cycle.
func (d *Exporter) addLegend(graph *dot.Graph, fm *fmesh.FMesh, activationCycle *cycle.Cycle) error {
	subgraph := graph.Subgraph("id-legend", dot.ClusterOption{})

	setAttrMap(&subgraph.AttributesMap, d.config.Legend.Subgraph)
	subgraph.Delete("label")

	legendData := make(map[string]any)
	legendData["meshDescription"] = fmt.Sprintf("A mesh with %d components", fm.Components().Len())
	if fm.Description() != "" {
		legendData["meshDescription"] = fm.Description()
	}

	if activationCycle != nil {
		legendData["cycleNumber"] = activationCycle.Number()
		legendData["stats"] = getCycleStats(activationCycle)
	}

	legendHTML := new(bytes.Buffer)
	err := template.Must(
		template.New("legend").
			Parse(legendTemplate)).
		Execute(legendHTML, legendData)

	if err != nil {
		return fmt.Errorf("failed to render legend: %w", err)
	}

	legendNode := subgraph.Node("legend-subgraph")
	setAttrMap(&legendNode.AttributesMap, d.config.Legend.Node)
	legendNode.Attr("label", dot.HTML(legendHTML.String()))

	return nil
}

// getCycleStats returns basic cycle stats.
func getCycleStats(activationCycle *cycle.Cycle) []*statEntry {
	// Initialize all possible activation states with zero values
	// This ensures all counters are always shown, even when zero
	statsMap := map[string]*statEntry{
		"activated": {
			Name:  "Activated",
			Value: 0,
		},
		// All possible activation result codes
		fmeshcomponent.ActivationCodeOK.String(): {
			Name:  fmeshcomponent.ActivationCodeOK.String(),
			Value: 0,
		},
		fmeshcomponent.ActivationCodeNoInput.String(): {
			Name:  fmeshcomponent.ActivationCodeNoInput.String(),
			Value: 0,
		},
		fmeshcomponent.ActivationCodeNoFunction.String(): {
			Name:  fmeshcomponent.ActivationCodeNoFunction.String(),
			Value: 0,
		},
		fmeshcomponent.ActivationCodeReturnedError.String(): {
			Name:  fmeshcomponent.ActivationCodeReturnedError.String(),
			Value: 0,
		},
		fmeshcomponent.ActivationCodePanicked.String(): {
			Name:  fmeshcomponent.ActivationCodePanicked.String(),
			Value: 0,
		},
		fmeshcomponent.ActivationCodeWaitingForInputsClear.String(): {
			Name:  fmeshcomponent.ActivationCodeWaitingForInputsClear.String(),
			Value: 0,
		},
		fmeshcomponent.ActivationCodeWaitingForInputsKeep.String(): {
			Name:  fmeshcomponent.ActivationCodeWaitingForInputsKeep.String(),
			Value: 0,
		},
	}

	activationCycle.ActivationResults().ForEach(func(ar *fmeshcomponent.ActivationResult) error {
		if ar.Activated() {
			statsMap["activated"].Value++
		}

		// Increment the counter for this activation result code
		// All possible codes are pre-initialized above
		if entryByCode, ok := statsMap[ar.Code().String()]; ok {
			entryByCode.Value++
		}

		return ar.ChainableErr()
	})

	// Convert to slice to preserve keys order
	statsList := make([]*statEntry, 0)
	for _, entry := range statsMap {
		statsList = append(statsList, entry)
	}

	sort.Slice(statsList, func(i, j int) bool {
		return statsList[i].Name < statsList[j].Name
	})
	return statsList
}

// getPortID returns unique ID used to locate ports while building pipe edges.
func getPortID(componentName string, direction port.Direction, name string) string {
	return fmt.Sprintf("component/%s/%s/%s", componentName, portDirectionToString(direction), name)
}

// setAttrMap sets all attributes to target.
func setAttrMap(target *dot.AttributesMap, attributes attributesMap) {
	for attrName, attrValue := range attributes {
		target.Attr(attrName, attrValue)
	}
}

func portDirectionToString(direction port.Direction) string {
	switch direction {
	case port.DirectionIn:
		return "in"
	case port.DirectionOut:
		return "out"
	default:
		return ""
	}
}
