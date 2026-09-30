package dot

import (
	"maps"

	fmeshcomponent "github.com/hovsep/fmesh/component"
)

type attributesMap map[string]string

const (
	attrColor    = "color"
	attrPenwidth = "penwidth"
	attrShape    = "shape"
	attrStyle    = "style"
)

// Element names a part of the drawing that WithAttrs styles.
type Element int

// Drawing elements.
const (
	Graph          Element = iota // the whole graph
	Component                     // a component's cluster
	ComponentNodes                // defaults for every node inside a component's cluster
	ComponentNode                 // the node standing for the component itself
	ErrorNode                     // the activation-error node of a cycle graph
	Port                          // a port node
	Pipe                          // a pipe edge
	Legend                        // the legend cluster
	LegendNode                    // the legend's text node
)

// style is the rendering configuration. Options change it one piece at a time
// on top of defaultStyle, so an option never resets what it does not mention.
type style struct {
	attrs          map[Element]attributesMap
	resultAttrs    map[fmeshcomponent.ActivationResultCode]attributesMap
	componentLabel string
}

func defaultStyle() *style {
	return &style{
		attrs: map[Element]attributesMap{
			Graph: {
				"layout":  "dot",
				"splines": "ortho",
			},
			Component: {
				attrStyle:    "rounded",
				attrColor:    "black",
				"margin":     "20",
				attrPenwidth: "5",
			},
			ComponentNodes: {
				"fontname":   "Courier New",
				"width":      "1.0",
				"height":     "1.0",
				attrPenwidth: "2.5",
				attrStyle:    "filled",
			},
			ComponentNode: {
				attrShape: "rect",
				attrColor: "#9dddea",
				attrStyle: "filled",
			},
			Port: {
				attrShape: "circle",
			},
			Pipe: {
				"minlen":     "3",
				attrPenwidth: "2",
				attrColor:    "#e437ea",
			},
			Legend: {
				attrStyle:   "dashed,filled",
				"fillcolor": "#e2c6fc",
			},
			LegendNode: {
				attrShape:  "plaintext",
				attrColor:  "green",
				"fontname": "Courier New",
			},
		},
		resultAttrs: map[fmeshcomponent.ActivationResultCode]attributesMap{
			fmeshcomponent.ActivationCodeOK:                    {attrColor: "green"},
			fmeshcomponent.ActivationCodeNoInput:               {attrColor: "yellow"},
			fmeshcomponent.ActivationCodeReturnedError:         {attrColor: "red"},
			fmeshcomponent.ActivationCodePanicked:              {attrColor: "pink"},
			fmeshcomponent.ActivationCodeWaitingForInputsClear: {attrColor: "blue"},
			fmeshcomponent.ActivationCodeWaitingForInputsKeep:  {attrColor: "purple"},
			fmeshcomponent.ActivationCodeHookFailed:            {attrColor: "orange"},
		},
		componentLabel: "𝑓",
	}
}

// merge sets attrs over the current ones for a key, creating the map if needed.
func merge[K comparable](m map[K]attributesMap, key K, attrs map[string]string) {
	if m[key] == nil {
		m[key] = attributesMap{}
	}
	maps.Copy(m[key], attrs)
}

const legendHTML = `
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
