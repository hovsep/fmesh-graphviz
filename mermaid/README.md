# mermaid

Export an [F-Mesh](https://github.com/hovsep/fmesh) mesh as a [Mermaid](https://mermaid.js.org)
flowchart. Mermaid renders directly in GitHub and GitLab markdown, so the chart can live in your
README with no extra tools.

- **Static chart:** components (as subgraphs), ports and pipes.
- **Per-cycle charts:** one chart per cycle, with components colored by activation result and
  errors shown next to the component.

`New(opts...)` returns an `Exporter`. It implements this module's
[`export.Exporter`](https://pkg.go.dev/github.com/hovsep/fmesh-export), like every format in this
module. It holds only its options: reuse one value for many meshes.

## Install

```sh
go get github.com/hovsep/fmesh-export/mermaid
```

## Static chart

```go
import "github.com/hovsep/fmesh-export/mermaid"

e := mermaid.New()
// ... build fm: add components and pipes ...

src, err := e.Export(fm)
if err != nil {
    return err
}
if err := os.WriteFile("mesh.mmd", src, 0o644); err != nil {
    return err
}
```

## Per-cycle charts

```go
e := mermaid.New()
// ... build and seed fm ...

ri, err := fm.Run(ctx)
if err != nil {
    return err
}
for i, c := range ri.Cycles.All() {
    src, err := e.ExportCycle(fm, c) // one chart for cycle c
    if err != nil {
        return err
    }
    if err := os.WriteFile(fmt.Sprintf("cycle-%03d.mmd", i+1), src, 0o644); err != nil {
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

Default colors: green = OK, gold = no input, red = error, hot pink = panic, orange = hook failed,
blue / purple = waiting (dropping / keeping inputs). Change one with `WithResultColor`; the
others keep their defaults.

## Options

| Option | Effect |
|---|---|
| `WithDirection("TB")` | Flowchart direction: `LR` (default), `RL`, `TB`, `BT` |
| `WithResultColor(code, color)` | Stroke color for one activation result code |

`New` does not check the direction. `Export` and `ExportCycle` return an error for an unknown one.

## Render

- In markdown: paste the source into a ```` ```mermaid ```` block. GitHub and GitLab render it.
- With [mermaid-cli](https://github.com/mermaid-js/mermaid-cli): `mmdc -i mesh.mmd -o mesh.svg`
  (also `.png`, `.pdf`).
- Online: paste the source into [mermaid.live](https://mermaid.live).
