# d2

Export an [F-Mesh](https://github.com/hovsep/fmesh) mesh as [D2](https://d2lang.com) diagram source.

- **Static diagram:** components (as containers), their ports, and pipes.
- **Per-cycle diagrams:** one diagram per cycle. Each component is colored by its activation
  result. An error shows next to the component that failed.
- The output is deterministic: equal meshes give byte-identical source.
- The export only reads the mesh. It never changes it.

It is an fmesh plugin: attach it with `fmesh.WithPlugins`.

## Install

```sh
go get github.com/hovsep/fmesh-export/d2
```

## Static diagram

```go
import "github.com/hovsep/fmesh-export/d2"

diagram := d2.New()
fm, err := fmesh.New("mesh", fmesh.WithPlugins(diagram))
if err != nil {
    return err
}
// ... add components and pipes ...

src, err := diagram.Export() // D2 source
if err != nil {
    return err
}
if err := os.WriteFile("mesh.d2", src, 0o644); err != nil {
    return err
}
```

For a mesh that is already built without the plugin, call the package function:
`d2.Export(fm)`. It takes the same options.

## Per-cycle diagrams

```go
diagram := d2.New(d2.WithCycles())
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
- A component with no activation result in a cycle counts as "no input".
- Default colors: green = OK, gold = no input, red = error, hot pink = panic, orange = hook
  failed, blue / purple = waiting (dropping / keeping inputs).
- Change one color with `WithResultColor`. The others keep their defaults.

## Options

| Option | Effect |
|---|---|
| `WithDirection("down")` | Layout direction: `right` (default), `left`, `down`, `up` |
| `WithCycles()` | Record every cycle of the latest run, for `ExportCycles` |
| `WithResultColor(code, color)` | Stroke color for one activation result code (a CSS color name or `#rrggbb`) |

## Render

- With the [D2 CLI](https://d2lang.com/tour/install): `d2 mesh.d2 mesh.svg` (also `.png`, `.pdf`).
- Online: paste the source into [play.d2lang.com](https://play.d2lang.com).
