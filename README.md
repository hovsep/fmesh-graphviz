# fmesh-export

Export an [F-Mesh](https://github.com/hovsep/fmesh) mesh as data or as a diagram. One Go module,
one package per format:

| Format | Package | Render with |
|---|---|---|
| JSON | [`json`](json) | any JSON tool; unmarshal into `json.Mesh` / `json.Cycle` |
| YAML | [`yaml`](yaml) | any YAML tool; unmarshal into `yaml.Mesh` / `yaml.Cycle` |
| [Graphviz DOT](https://graphviz.org) | [`dot`](dot) | `dot -Tpng`, [edotor.net](https://edotor.net) |
| [Mermaid](https://mermaid.js.org) | [`mermaid`](mermaid) | GitHub markdown, [mermaid.live](https://mermaid.live) |
| [D2](https://d2lang.com) | [`d2`](d2) | `d2`, [play.d2lang.com](https://play.d2lang.com) |
| [PlantUML](https://plantuml.com) | [`plantuml`](plantuml) | `plantuml -tpng`, plantuml.com server |

Every exporter implements [`export.Exporter`](https://pkg.go.dev/github.com/hovsep/fmesh-export),
the interface in this module's root package, so one format can replace another.

## Install

```bash
go get github.com/hovsep/fmesh-export/<format>  # e.g. /mermaid
```

## Usage

Every format has the same shape:

```go
import "github.com/hovsep/fmesh-export/mermaid"

e := mermaid.New() // options go here, e.g. mermaid.WithDirection("TB")

// ... build and seed fm ...

static, err := e.Export(fm) // the structure
if err != nil {
    return err
}

ri, err := fm.Run(ctx)
if err != nil {
    return err
}
for _, c := range ri.Cycles.All() {
    frame, err := e.ExportCycle(fm, c) // the structure, colored by the results of cycle c
    if err != nil {
        return err
    }
    // ... write frame ...
}
```

- `New(opts...)` creates an exporter. It cannot fail.
- `Export(fm)` returns the structure: components, ports, pipes, descriptions.
- `ExportCycle(fm, c)` returns the structure in the state of one cycle. Components are colored by
  their activation result. A component with no result in `c` had no input.
- An invalid option (for example an unknown direction) is returned as an error by `Export` and
  `ExportCycle`.
- Options set one thing on top of the defaults. Anything you do not set keeps its default. See
  each package's README.
- An exporter holds only its options. Reuse one value for many meshes.
- An exporter never changes the mesh. Equal meshes give byte-identical output.

### Streaming: one frame per cycle while the mesh runs

`fmesh.WithCyclesHistoryLimit` keeps only the last cycles in `ri.Cycles`. To get every cycle, or to
see frames while the mesh runs, export from an `AfterCycle` hook:

```go
e := mermaid.New()
fm.SetupHooks(func(h *fmesh.Hooks) {
    h.AfterCycle(func(ctx context.Context, cc *fmesh.CycleContext) error {
        frame, err := e.ExportCycle(cc.FMesh, cc.Cycle)
        if err != nil {
            return err
        }
        // ... write or send frame ...
        return nil
    })
})
```

### Any format through one interface

Code that does not care about the format can take an `export.Exporter`:

```go
import "github.com/hovsep/fmesh-export" // package export

func save(e export.Exporter, fm *fmesh.FMesh) ([]byte, error) {
    return e.Export(fm)
}

save(dot.New(), fm)
save(json.New(), fm)
```

## Live examples

After every merge, CI renders the showcase mesh in [`internal/cmd/showcase`](internal/cmd/showcase/main.go)
(fan-out, fan-in, an error and a wait) in every diagram format, and shows the pictures on the run's
summary page. One workflow per format: [DOT](.github/workflows/dot.yml), [Mermaid](.github/workflows/mermaid.yml),
[D2](.github/workflows/d2.yml), [PlantUML](.github/workflows/plantuml.yml).

| Format | Structure | Run replay |
|---|---|---|
| DOT | ![DOT structure](https://raw.githubusercontent.com/hovsep/fmesh-export/ci-graphs/latest/dot/static.png) | ![DOT cycles](https://raw.githubusercontent.com/hovsep/fmesh-export/ci-graphs/latest/dot/cycles.gif) |
| Mermaid | ![Mermaid structure](https://raw.githubusercontent.com/hovsep/fmesh-export/ci-graphs/latest/mermaid/static.png) | ![Mermaid cycles](https://raw.githubusercontent.com/hovsep/fmesh-export/ci-graphs/latest/mermaid/cycles.gif) |
| D2 | ![D2 structure](https://raw.githubusercontent.com/hovsep/fmesh-export/ci-graphs/latest/d2/static.png) | ![D2 cycles](https://raw.githubusercontent.com/hovsep/fmesh-export/ci-graphs/latest/d2/cycles.gif) |
| PlantUML | ![PlantUML structure](https://raw.githubusercontent.com/hovsep/fmesh-export/ci-graphs/latest/plantuml/static.png) | ![PlantUML cycles](https://raw.githubusercontent.com/hovsep/fmesh-export/ci-graphs/latest/plantuml/cycles.gif) |

Run it locally: `go run ./internal/cmd/showcase -format d2 out`.

## Writing a new exporter

Copy the [`d2`](d2) package: it is a complete exporter with the full set of tests. A new data
format is smaller: build the documents with `internal/doc` and only encode them (add the format's
struct tags to the types there). A new format `xyz` needs:

1. **A package** `xyz/` with `xyz.go`, `xyz_test.go` and a `README.md`.
2. **The shared shape**, the same as every other format:
   - `type Exporter struct{...}` that holds only the options.
   - `type Option func(*Exporter)`, and single options that change one thing on top of the
     defaults. No config struct that replaces everything.
   - `func New(opts ...Option) *Exporter`. It cannot fail.
   - `Export(fm *fmesh.FMesh) ([]byte, error)` for the structure.
   - `ExportCycle(fm *fmesh.FMesh, c *cycle.Cycle) ([]byte, error)` for one cycle.
   - `var _ export.Exporter = (*Exporter)(nil)`, so the compiler checks the interface.
3. **Rendering through `fm.Walk(visitor)`.** Implement `VisitMesh`, `VisitComponent`, `VisitPort`
   and `VisitPipe`. Rules:
   - Never change the mesh.
   - Keep no state in the `Exporter` between calls. Put per-export state in the visitor.
   - Check the options in `Export` and `ExportCycle`, and return an error for a bad one.
   - The output must be byte-identical for equal meshes.
   - Use generated IDs (`c1`, `p2`, ...) and put names only in quoted, escaped labels.
   - In a cycle graph, a component with no result had no input (`ActivationCodeNoInput`).
4. **Tests:**
   - one exact expected output for `Export`;
   - cycle colors and an error text for `ExportCycle`;
   - determinism;
   - awkward names (quotes, dots, colons, newlines);
   - each option, and option checks in both `Export` and `ExportCycle`;
   - one `Exporter` reused for two meshes;
   - `ExportCycle` from an `AfterCycle` hook gives one frame per cycle.

   The `d2` tests cover all of these.
5. **The showcase:** add the format to `formats` in [`internal/cmd/showcase`](internal/cmd/showcase/main.go).
6. **CI** (diagram formats only; data formats have nothing to render): copy
   `.github/workflows/d2.yml` to `xyz.yml`, and add an `xyz)` case to the
   "Install renderer" and "Render images" steps of `render.yml`. Pin the renderer version and check
   its SHA-256 there.
7. **Docs:** add a row to the format table and the live-example table above.

Run `make check` before you open a pull request. The PR's checks upload the rendered images as an
artifact, so you can look at your format before it is merged.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

See [LICENSE](LICENSE).
