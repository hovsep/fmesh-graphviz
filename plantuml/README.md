# plantuml

Export an [F-Mesh](https://github.com/hovsep/fmesh) mesh as a [PlantUML](https://plantuml.com)
component diagram.

- **Static diagram:** components, their ports and the pipes between them.
- **Per-cycle diagrams:** one diagram per cycle. Each component is colored by its activation
  result. An error is shown as a note next to the component.
- The output is plain PlantUML source (`@startuml` … `@enduml`). Equal meshes give byte-identical
  output.

`New(opts...)` returns an `Exporter`. It implements fmesh's
[`export.Exporter`](https://pkg.go.dev/github.com/hovsep/fmesh/export), like every format in this
module and `export.JSON()` in fmesh. It holds only its options: reuse one value for many meshes.

## Install

```sh
go get github.com/hovsep/fmesh-export/plantuml
```

## Static diagram

```go
import "github.com/hovsep/fmesh-export/plantuml"

e := plantuml.New()
// ... build fm: add components and pipes ...

src, err := e.Export(fm)
if err != nil {
    return err
}
if err := os.WriteFile("mesh.puml", src, 0o644); err != nil {
    return err
}
```

## Per-cycle diagrams

```go
e := plantuml.New()
// ... build and seed fm ...

ri, err := fm.Run(ctx)
if err != nil {
    return err
}
for i, c := range ri.Cycles.All() {
    src, err := e.ExportCycle(fm, c) // one diagram for cycle c
    if err != nil {
        return err
    }
    if err := os.WriteFile(fmt.Sprintf("cycle-%03d.puml", i+1), src, 0o644); err != nil {
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

- The title shows the cycle number.
- Default border colors: green = OK, gold = no input, red = error, hot pink = panic,
  orange = hook failed, blue / purple = waiting (dropping / keeping inputs).
- Change one color with `WithResultColor`. The others keep their defaults.

## Options

| Option | Effect |
|---|---|
| `WithDirection("top to bottom")` | Layout direction: `"left to right"` (default) or `"top to bottom"` |
| `WithResultColor(code, color)` | Border color for one activation result code: a name (`"teal"`) or hex (`"#00AA00"`) |

`New` does not check the direction. `Export` and `ExportCycle` return an error for an unknown one.

## Render

- **PlantUML CLI:** `plantuml -tsvg mesh.puml` (or `-tpng`). Needs Java.
- **Online:** paste the source into the server at [plantuml.com/plantuml](https://www.plantuml.com/plantuml).
- Many IDEs and wikis also render `.puml` files with a PlantUML plugin.
