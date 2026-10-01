# fmesh-graphviz

Export an [F-Mesh](https://github.com/hovsep/fmesh) mesh as a [Graphviz DOT](https://graphviz.org/doc/info/lang.html) graph.

- **Static graph:** components, ports, pipes and descriptions.
- **Per-cycle graphs:** one graph per cycle. Components are colored by their activation result,
  and a legend shows the cycle stats. Put them together as an animation of the run.
- **Configurable:** colors, shapes and layout.

It is an fmesh plugin: attach it with `fmesh.WithPlugins`. See the
[dot package docs](https://pkg.go.dev/github.com/hovsep/fmesh-graphviz/dot) for the API.

## Live example

After every merge, CI renders the showcase mesh in
[`internal/cmd/showcase`](internal/cmd/showcase/main.go) (fan-out, fan-in, an error and a wait) and
shows the pictures on the run's summary page. The latest ones:

![Showcase mesh](https://raw.githubusercontent.com/hovsep/fmesh-graphviz/ci-graphs/latest/static.png)

![Showcase run, one frame per cycle](https://raw.githubusercontent.com/hovsep/fmesh-graphviz/ci-graphs/latest/cycles.gif)

Run it locally: `go run ./internal/cmd/showcase out`, then render the `.dot` files with `dot -Tpng`.

## Static graph

```go
import "github.com/hovsep/fmesh-graphviz/dot"

graphviz := dot.New()
fm, err := fmesh.New("mesh", fmesh.WithPlugins(graphviz))
if err != nil {
    return err
}
// ... add components and pipes ...

graph, err := graphviz.Export() // DOT source; nil for an empty mesh
if err != nil {
    return err
}
if err := os.WriteFile("mesh.dot", graph, 0o644); err != nil {
    return err
}
```

For a mesh that is already built without the plugin, call the package function: `dot.Export(fm)`
(it takes the same options).

View it on [edotor.net](https://edotor.net), or render it with Graphviz:

```bash
dot -Tsvg mesh.dot -o mesh.svg
```

<img src="https://github.com/user-attachments/assets/b27bd458-c03d-4cc6-bea3-542f0e839697" width="500px">

## Per-cycle graphs

Create the plugin with `WithCycles`. It records every cycle of the latest run, so it works even with
`fmesh.WithCyclesHistoryLimit`.

```go
graphviz := dot.New(dot.WithCycles())
fm, err := fmesh.New("mesh", fmesh.WithPlugins(graphviz))
if err != nil {
    return err
}
// ... build and seed the mesh ...

if _, err := fm.Run(ctx); err != nil {
    return err
}
graphs, err := graphviz.ExportCycles() // one graph per cycle, in order
if err != nil {
    return err
}
for i, graph := range graphs {
    if err := os.WriteFile(fmt.Sprintf("cycle-%03d.dot", i+1), graph, 0o644); err != nil {
        return err
    }
}
```

Each legend counts the components in every activation state (OK, No input, error, panic, hook failed,
waiting). Colors: green = OK, yellow = no input, red = error, pink = panic, orange = hook failed,
blue / purple = waiting (dropping / keeping inputs).

Make an animation (needs ImageMagick):

```bash
for f in cycle-*.dot; do dot -Tpng "$f" -o "${f%.dot}.png"; done
convert -delay 100 -loop 0 cycle-*.png mesh.gif
```

![](https://github.com/user-attachments/assets/3ac501e7-b62f-4fd6-9908-be399a6ca464)

## Configuration

Each option sets attributes on top of the defaults; anything it does not name keeps its default.

```go
graphviz := dot.New(
    dot.WithAttrs(dot.Graph, map[string]string{"layout": "neato"}), // dot, neato, fdp, circo, ...
    dot.WithAttrs(dot.ComponentNode, map[string]string{"color": "#ffcc00"}),
    dot.WithResultAttrs(component.ActivationCodeOK, map[string]string{"color": "darkgreen"}),
    dot.WithComponentLabel("fn"),
)
```

| Option | Styles |
|---|---|
| `WithAttrs(element, attrs)` | one element: `Graph`, `Component`, `ComponentNodes`, `ComponentNode`, `ErrorNode`, `Port`, `Pipe`, `Legend`, `LegendNode` |
| `WithResultAttrs(code, attrs)` | a component's cluster in a cycle graph, by activation result |
| `WithComponentLabel(label)` | the node of a component with no description (default `𝑓`) |
| `WithCycles()` | records every cycle for `ExportCycles` |
