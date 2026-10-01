# mermaid

Export an [F-Mesh](https://github.com/hovsep/fmesh) mesh as a [Mermaid](https://mermaid.js.org)
flowchart. Mermaid renders directly in GitHub and GitLab markdown, so the chart can live in your
README with no extra tools.

- **Static chart:** components (as subgraphs), ports and pipes.
- **Per-cycle charts:** one chart per cycle, with components colored by activation result and
  errors shown next to the component.

It is an fmesh plugin: attach it with `fmesh.WithPlugins`.

## Static chart

```go
import "github.com/hovsep/fmesh-export/mermaid"

chart := mermaid.New()
fm, err := fmesh.New("mesh", fmesh.WithPlugins(chart))
if err != nil {
    return err
}
// ... add components and pipes ...

src, err := chart.Export() // Mermaid source
if err != nil {
    return err
}
if err := os.WriteFile("mesh.mmd", src, 0o644); err != nil {
    return err
}
```

For a mesh that is already built without the plugin, call the package function: `mermaid.Export(fm)`
(it takes the same options).

Paste the source into a ```` ```mermaid ```` block in any markdown file, or into
[mermaid.live](https://mermaid.live).

## Per-cycle charts

```go
chart := mermaid.New(mermaid.WithCycles())
fm, err := fmesh.New("mesh", fmesh.WithPlugins(chart))
if err != nil {
    return err
}
// ... build and seed the mesh ...

if _, err := fm.Run(ctx); err != nil {
    return err
}
charts, err := chart.ExportCycles() // one chart per cycle, in order
```

Default colors: green = OK, gold = no input, red = error, hot pink = panic, orange = hook failed,
blue / purple = waiting (dropping / keeping inputs). Change one with `WithResultColor`; the
others keep their defaults.

## Options

| Option | Effect |
|---|---|
| `WithDirection("TB")` | Flowchart direction: `LR` (default), `RL`, `TB`, `BT` |
| `WithCycles()` | Record every cycle of the latest run, for `ExportCycles` |
| `WithResultColor(code, color)` | Stroke color for one activation result code |
