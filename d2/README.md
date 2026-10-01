# d2

Export an [F-Mesh](https://github.com/hovsep/fmesh) mesh as [D2](https://d2lang.com) diagram source.

- **Static diagram:** components (as containers), their ports, and pipes.
- **Per-cycle diagrams:** one diagram per cycle. Each component is colored by its activation
  result. An error shows next to the component that failed.
- The output is deterministic: equal meshes give byte-identical source.
- The export only reads the mesh. It never changes it.

`New(opts...)` returns an `Exporter`. It implements fmesh's
[`export.Exporter`](https://pkg.go.dev/github.com/hovsep/fmesh/export), like every format in this
module and `export.JSON()` in fmesh. It holds only its options: reuse one value for many meshes.

## Install

```sh
go get github.com/hovsep/fmesh-export/d2
```

## Static diagram

```go
import "github.com/hovsep/fmesh-export/d2"

e := d2.New()
// ... build fm: add components and pipes ...

src, err := e.Export(fm)
if err != nil {
    return err
}
if err := os.WriteFile("mesh.d2", src, 0o644); err != nil {
    return err
}
```

## Per-cycle diagrams

```go
e := d2.New()
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
    if err := os.WriteFile(fmt.Sprintf("cycle-%03d.d2", i+1), src, 0o644); err != nil {
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
- A component with no activation result in a cycle counts as "no input".
- Default colors: green = OK, gold = no input, red = error, hot pink = panic, orange = hook
  failed, blue / purple = waiting (dropping / keeping inputs).
- Change one color with `WithResultColor`. The others keep their defaults.

## Options

| Option | Effect |
|---|---|
| `WithDirection("down")` | Layout direction: `right` (default), `left`, `down`, `up` |
| `WithResultColor(code, color)` | Stroke color for one activation result code (a CSS color name or `#rrggbb`) |

`New` does not check the direction. `Export` and `ExportCycle` return an error for an unknown one.

## Render

- With the [D2 CLI](https://d2lang.com/tour/install): `d2 mesh.d2 mesh.svg` (also `.png`, `.pdf`).
- Online: paste the source into [play.d2lang.com](https://play.d2lang.com).
