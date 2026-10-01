# dot

Export an [F-Mesh](https://github.com/hovsep/fmesh) mesh as a [Graphviz DOT](https://graphviz.org/doc/info/lang.html) graph.

- **Static graph:** components, ports, pipes and descriptions.
- **Per-cycle graphs:** one graph per cycle. Components are colored by their activation result,
  and a legend shows the cycle stats. Put them together as an animation of the run.
- **Configurable:** colors, shapes and layout.

`New(opts...)` returns an `Exporter`. It implements fmesh's
[`export.Exporter`](https://pkg.go.dev/github.com/hovsep/fmesh/export), like every format in this
module and `export.JSON()` in fmesh. It holds only its options: reuse one value for many meshes.
See the [dot package docs](https://pkg.go.dev/github.com/hovsep/fmesh-export/dot) for the API.

## Static graph

```go
import "github.com/hovsep/fmesh-export/dot"

e := dot.New()
// ... build fm: add components and pipes ...

src, err := e.Export(fm) // DOT source; nil for an empty mesh
if err != nil {
    return err
}
if err := os.WriteFile("mesh.dot", src, 0o644); err != nil {
    return err
}
```

View it on [edotor.net](https://edotor.net), or render it with Graphviz:

```bash
dot -Tsvg mesh.dot -o mesh.svg
```

<img src="https://github.com/user-attachments/assets/b27bd458-c03d-4cc6-bea3-542f0e839697" width="500px">

## Per-cycle graphs

```go
e := dot.New()
// ... build and seed fm ...

ri, err := fm.Run(ctx)
if err != nil {
    return err
}
for i, c := range ri.Cycles.All() {
    src, err := e.ExportCycle(fm, c) // one graph for cycle c
    if err != nil {
        return err
    }
    if err := os.WriteFile(fmt.Sprintf("cycle-%03d.dot", i+1), src, 0o644); err != nil {
        return err
    }
}
```

To export while the mesh runs, call `ExportCycle` from an `AfterCycle` hook. This also gets every
cycle when `fmesh.WithCyclesHistoryLimit` drops old ones from `ri.Cycles`:

```go
fm.SetupHooks(func(h *fmesh.Hooks) {
    h.AfterCycle(func(ctx context.Context, cc *fmesh.CycleContext) error {
        src, err := e.ExportCycle(cc.FMesh, cc.Cycle)
        if err != nil {
            return err
        }
        // ... write or send src ...
        return nil
    })
})
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
e := dot.New(
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
