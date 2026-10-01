# plantuml

Export an [F-Mesh](https://github.com/hovsep/fmesh) mesh as a [PlantUML](https://plantuml.com)
component diagram.

- **Static diagram:** components, their ports and the pipes between them.
- **Per-cycle diagrams:** one diagram per cycle. Each component is colored by its activation
  result. An error is shown as a note next to the component.
- The output is plain PlantUML source (`@startuml` … `@enduml`). Equal meshes give byte-identical
  output.

It is an fmesh plugin: attach it with `fmesh.WithPlugins`.

## Install

```sh
go get github.com/hovsep/fmesh-export/plantuml
```

## Static diagram

```go
import "github.com/hovsep/fmesh-export/plantuml"

diagram := plantuml.New()
fm, err := fmesh.New("mesh", fmesh.WithPlugins(diagram))
if err != nil {
    return err
}
// ... add components and pipes ...

src, err := diagram.Export() // PlantUML source
if err != nil {
    return err
}
if err := os.WriteFile("mesh.puml", src, 0o644); err != nil {
    return err
}
```

For a mesh that is already built without the plugin, call the package function:
`plantuml.Export(fm)`. It takes the same options.

## Per-cycle diagrams

```go
diagram := plantuml.New(plantuml.WithCycles())
fm, err := fmesh.New("mesh", fmesh.WithPlugins(diagram))
if err != nil {
    return err
}
// ... build and seed the mesh ...

if _, err := fm.Run(ctx); err != nil {
    return err
}
diagrams, err := diagram.ExportCycles() // one diagram per cycle, in order
```

- The title shows the cycle number.
- Default border colors: green = OK, gold = no input, red = error, hot pink = panic,
  orange = hook failed, blue / purple = waiting (dropping / keeping inputs).
- Change one color with `WithResultColor`. The others keep their defaults.

## Options

| Option | Effect |
|---|---|
| `WithDirection("top to bottom")` | Layout direction: `"left to right"` (default) or `"top to bottom"` |
| `WithCycles()` | Record every cycle of the latest run, for `ExportCycles` |
| `WithResultColor(code, color)` | Border color for one activation result code: a name (`"teal"`) or hex (`"#00AA00"`) |

## Render

- **PlantUML CLI:** `plantuml -tsvg mesh.puml` (or `-tpng`). Needs Java.
- **Online:** paste the source into the server at [plantuml.com/plantuml](https://www.plantuml.com/plantuml).
- Many IDEs and wikis also render `.puml` files with a PlantUML plugin.
