package dot

import fmeshcomponent "github.com/hovsep/fmesh/component"

type attributesMap map[string]string

const (
	attrColor    = "color"
	attrPenwidth = "penwidth"
	attrShape    = "shape"
	attrStyle    = "style"
)

// ComponentConfig defines the configuration for the component visualization.
type ComponentConfig struct {
	Subgraph                                 attributesMap
	SubgraphNodeBaseAttrs                    attributesMap
	Node                                     attributesMap
	NodeDefaultLabel                         string
	ErrorNode                                attributesMap
	SubgraphAttributesByActivationResultCode map[fmeshcomponent.ActivationResultCode]attributesMap
}

// PortConfig defines the configuration for the port visualization.
type PortConfig struct {
	Node attributesMap
}

// LegendConfig defines the configuration for the legend visualization.
type LegendConfig struct {
	Subgraph attributesMap
	Node     attributesMap
}

// PipeConfig defines the configuration for the pipe visualization.
type PipeConfig struct {
	Edge attributesMap
}

// Config defines the configuration for the dot exporter.
type Config struct {
	MainGraph attributesMap
	Component ComponentConfig
	Port      PortConfig
	Pipe      PipeConfig
	Legend    LegendConfig
}

var (
	defaultConfig = &Config{
		MainGraph: attributesMap{
			"layout":  "dot",
			"splines": "ortho",
		},
		Component: ComponentConfig{
			Subgraph: attributesMap{
				attrStyle:    "rounded",
				attrColor:    "black",
				"margin":     "20",
				attrPenwidth: "5",
			},
			SubgraphNodeBaseAttrs: attributesMap{
				"fontname":   "Courier New",
				"width":      "1.0",
				"height":     "1.0",
				attrPenwidth: "2.5",
				attrStyle:    "filled",
			},
			Node: attributesMap{
				attrShape: "rect",
				attrColor: "#9dddea",
				attrStyle: "filled",
			},
			NodeDefaultLabel: "𝑓",
			ErrorNode:        nil,
			SubgraphAttributesByActivationResultCode: map[fmeshcomponent.ActivationResultCode]attributesMap{
				fmeshcomponent.ActivationCodeOK: {
					attrColor: "green",
				},
				fmeshcomponent.ActivationCodeNoInput: {
					attrColor: "yellow",
				},
				fmeshcomponent.ActivationCodeReturnedError: {
					attrColor: "red",
				},
				fmeshcomponent.ActivationCodePanicked: {
					attrColor: "pink",
				},
				fmeshcomponent.ActivationCodeWaitingForInputsClear: {
					attrColor: "blue",
				},
				fmeshcomponent.ActivationCodeWaitingForInputsKeep: {
					attrColor: "purple",
				},
			},
		},
		Port: PortConfig{
			Node: attributesMap{
				attrShape: "circle",
			},
		},
		Pipe: PipeConfig{
			Edge: attributesMap{
				"minlen":     "3",
				attrPenwidth: "2",
				attrColor:    "#e437ea",
			},
		},
		Legend: LegendConfig{
			Subgraph: attributesMap{
				attrStyle:   "dashed,filled",
				"fillcolor": "#e2c6fc",
			},
			Node: attributesMap{
				attrShape:  "plaintext",
				attrColor:  "green",
				"fontname": "Courier New",
			},
		},
	}

	legendTemplate = `
	<table border="0" cellborder="0" cellspacing="10">
			{{ if .meshDescription }}
			<tr>
				<td>Description:</td><td>{{ .meshDescription }}</td>
			</tr>
			{{ end }}
		
			{{ if .cycleNumber }}
			<tr>
				<td>Cycle:</td><td>{{ .cycleNumber }}</td>
			</tr>
			{{ end }}

			{{ if .stats }}
				{{ range .stats }}
				<tr>
					<td>{{ .Name }}:</td><td>{{ .Value }}</td>
				</tr>
				{{ end }}
			{{ end }}
	</table>
	`
)
