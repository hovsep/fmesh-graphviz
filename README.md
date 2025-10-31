# fmesh-graphviz

Export your [FMesh](https://pkg.go.dev/github.com/hovsep/fmesh) as DOT-graph for powerful visualization

This module provides high-quality DOT graph export for FMesh structures using the [DOT language](https://graphviz.org/doc/info/lang.html) - the graph description language used by Graphviz for visualizing structured data.

## ✨ Features

- **Static mesh visualization** - Export the structure of your FMesh
- **Cycle-by-cycle animation** - Export each activation cycle for dynamic visualization  
- **Complete statistics** - Always shows all activation states even with zero counts
- **Clear state labels** - Readable activation state names for easy understanding
- **Highly customizable** - Full control over colors, styles, and layout via configuration
- **Professional output** - Ready for documentation, presentations, and debugging

The exporter implements the [Exporter interface](https://pkg.go.dev/github.com/hovsep/fmesh/export) from the main FMesh library, making it easy to swap between different export formats.

Check out the [dot](https://pkg.go.dev/github.com/hovsep/fmesh-graphviz/dot) package documentation for detailed API reference.


## Using an Exporter

Let’s demonstrate how to export a mesh using the [DOT Exporter](https://pkg.go.dev/github.com/hovsep/fmesh/export/dot). We'll use [this example](https://github.com/hovsep/fmesh/blob/main/integration_tests/ports/waiting_for_inputs_test.go#L92) for demonstration:

```go
import "github.com/hovsep/fmesh-graphviz/dot"

// Create fm
// ...
exporter := dot.NewDotExporter()
data, err := exporter.Export(fm)
if err != nil {
	panic("failed to export mesh")
}
	
os.WriteFile("graph.dot",data, 0755)
```
If everything is successful, the `graph.dot` file will contain the DOT representation:

```dot
digraph  {
	layout="dot";splines="ortho";
	
	subgraph cluster_7 {
		cluster="true";color="black";label="d5";margin="20";penwidth="5";style="rounded";
...
```
You can now visualize the mesh using tools like [Edotor.net](https://edotor.net/) or render it with [Graphviz](https://graphviz.org/doc/info/command.html):

```bash
cat graph.dot  | dot -Tpng > graph.png
```

<img src="https://github.com/user-attachments/assets/b27bd458-c03d-4cc6-bea3-542f0e839697" width="500px">

>[!TIP]
You can customize every aspect of the graph's rendering by using **NewDotExporterWithConfig**

Graphviz supports various output formats such as PNG, SVG, and PDF. See the full list of supported formats [here](https://graphviz.org/docs/outputs/).

## Exporting Mesh with Cycles

To export a mesh along with its activation cycles, pass the cycle data to the exporter and save each cycle separately:
```go
runResult, err := fm.Run()
if err != nil {
    panic("failed to run mesh")
}

exporter := dot.NewDotExporter()
data, err := exporter.ExportWithCycles(fm, runResult.Cycles.CyclesOrNil())
if err != nil {
    panic("failed to export mesh")
}

for cycleNumber, cycleGraph := range data {
    filename := fmt.Sprintf("cycle-%d.dot", cycleNumber)
    os.WriteFile(filename, cycleGraph, 0644)
}
```

This creates separate `.dot` files for each cycle (e.g., `cycle-0.dot`, `cycle-1.dot`). Each cycle includes a **comprehensive statistics legend** showing:

- **Activated**: Total components that executed in this cycle
- **All activation states**: Counts for each component state (OK, NoInput, NoFunction, ReturnedError, Panicked, WaitingForInputsClear, WaitingForInputsKeep)

### Creating Animations

You can use these files to create animations of your program's execution:

![](https://github.com/user-attachments/assets/3ac501e7-b62f-4fd6-9908-be399a6ca464)

```bash
# Generate PNG files for each cycle
for file in cycle-*.dot; do
    dot -Tpng "$file" -o "${file%.dot}.png"
done

# Create animated GIF (requires ImageMagick)
convert -delay 100 -loop 0 cycle-*.png mesh-animation.gif
```

>[!NOTE]
>* Component colors change based on their activation state (green=success, yellow=no input, red=error, etc.)
>* The legend provides real-time statistics for each cycle
>* Layout may change when components enter waiting states

<img src="https://github.com/user-attachments/assets/3d315e9b-e920-46b8-b626-3061c259e9eb" width="400"/>

## Configuration

Customize the visual appearance using `NewDotExporterWithConfig`:

```go
config := &dot.Config{
    MainGraph: map[string]string{
        "layout": "neato",  // Try different layouts: dot, neato, fdp, circo
        "splines": "curved",
    },
    Component: dot.ComponentConfig{
        Node: map[string]string{
            "shape": "ellipse",
            "color": "#ffcc00",
        },
    },
}

exporter := dot.NewDotExporterWithConfig(config)
```

