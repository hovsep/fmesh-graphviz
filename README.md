# fmesh-graphviz

Export your [FMesh](https://pkg.go.dev/github.com/hovsep/fmesh) as DOT-graph

This module currently supports a single export format: [DOT](https://graphviz.org/doc/info/lang.html), the graph description language used by Graphviz for visualizing structured data. If needed, you can easily add your own export format by implementing the [Exporter](https://pkg.go.dev/github.com/hovsep/fmesh-graphviz#Exporter) interface:
```go
// Exporter is the common interface for all formats
type Exporter interface {
	// Export returns the F-Mesh structure in some format
	Export(fm *fmesh.FMesh) ([]byte, error)

	// ExportWithCycles returns the F-Mesh state for each activation cycle
	ExportWithCycles(fm *fmesh.FMesh, activationCycles cycle.Cycles) ([][]byte, error)
}
```

This interface is straightforward: you receive the mesh and return its representation. The **Export** method represents the static structure of the mesh, while **ExportWithCycles** provides a dynamic view, showing the state of the mesh at each activation cycle—useful for debugging and visualizing the execution process.

Check out the [dot](https://pkg.go.dev/github.com/hovsep/fmesh-graphviz/dot) package documentation for more details.


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
exporter := dot.NewDotExporter()
data, err := exporter.ExportWithCycles(fm, runResult.Cycles.CyclesOrNil())
if err != nil {
	panic("failed to export mesh")
}

for cycleNumber, cycleGraph := range data {
   os.WriteFile(fmt.Sprintf("cycle-%d.dot", cycleNumber),cycleGraph, 0755)
}
```
This code creates a separate .dot file for each cycle (e.g., cycle-0.dot, cycle-1.dot). You can use these files to create an animation of your program's execution, such as a GIF:

![](https://github.com/user-attachments/assets/3ac501e7-b62f-4fd6-9908-be399a6ca464)

>[!NOTE]
>* On Cycle-3, the layout changes because the **sum** component is waiting for inputs.
>* In the final cycle, no components are executed, and the mesh finishes naturally.


<img src="https://github.com/user-attachments/assets/3d315e9b-e920-46b8-b626-3061c259e9eb" width="400"/>

