# fmesh-export

Export an [F-Mesh](https://github.com/hovsep/fmesh) mesh as a diagram. One Go module, one package
per format:

| Format | Package | Render with |
|---|---|---|
| [Graphviz DOT](https://graphviz.org) | [`dot`](dot) | `dot -Tpng`, [edotor.net](https://edotor.net) |
| [Mermaid](https://mermaid.js.org) | [`mermaid`](mermaid) | GitHub markdown, [mermaid.live](https://mermaid.live) |
| [D2](https://d2lang.com) | [`d2`](d2) | `d2`, [play.d2lang.com](https://play.d2lang.com) |
| [PlantUML](https://plantuml.com) | [`plantuml`](plantuml) | `plantuml -tpng`, plantuml.com server |

JSON export is bundled with fmesh itself: [`plugin/jsonexport`](https://github.com/hovsep/fmesh/tree/main/plugin/jsonexport).

## Install

```bash
go get github.com/hovsep/fmesh-export/mermaid  # or /dot, /d2, /plantuml
```

## Usage

Every format has the same shape. Each exporter is an fmesh plugin:

```go
import "github.com/hovsep/fmesh-export/mermaid"

chart := mermaid.New(mermaid.WithCycles())
fm, err := fmesh.New("mesh", fmesh.WithPlugins(chart))
if err != nil {
    return err
}
// ... add components and pipes, seed, fm.Run(ctx) ...

static, err := chart.Export()       // the structure
cycles, err := chart.ExportCycles() // one diagram per cycle, colored by activation result
```

- `New(opts...)` creates the plugin; attach it with `fmesh.WithPlugins`. One plugin serves one mesh.
- `Export()` returns the structure: components, ports, pipes, descriptions.
- `WithCycles()` records every cycle of the next run; `ExportCycles()` then returns one diagram per
  cycle, with components colored by their activation result.
- `<pkg>.Export(fm, opts...)` exports a mesh that was built without the plugin.
- Options set one thing on top of the defaults; anything you do not set keeps its default. See
  each package's README.

## Live examples

After every merge, CI renders the showcase mesh in [`internal/cmd/showcase`](internal/cmd/showcase/main.go)
(fan-out, fan-in, an error and a wait) in every format, and shows the pictures on the run's summary
page. One workflow per format: [DOT](.github/workflows/dot.yml), [Mermaid](.github/workflows/mermaid.yml),
[D2](.github/workflows/d2.yml), [PlantUML](.github/workflows/plantuml.yml).

| Format | Structure | Run replay |
|---|---|---|
| DOT | ![DOT structure](https://raw.githubusercontent.com/hovsep/fmesh-export/ci-graphs/latest/dot/static.png) | ![DOT cycles](https://raw.githubusercontent.com/hovsep/fmesh-export/ci-graphs/latest/dot/cycles.gif) |
| Mermaid | ![Mermaid structure](https://raw.githubusercontent.com/hovsep/fmesh-export/ci-graphs/latest/mermaid/static.png) | ![Mermaid cycles](https://raw.githubusercontent.com/hovsep/fmesh-export/ci-graphs/latest/mermaid/cycles.gif) |
| D2 | ![D2 structure](https://raw.githubusercontent.com/hovsep/fmesh-export/ci-graphs/latest/d2/static.png) | ![D2 cycles](https://raw.githubusercontent.com/hovsep/fmesh-export/ci-graphs/latest/d2/cycles.gif) |
| PlantUML | ![PlantUML structure](https://raw.githubusercontent.com/hovsep/fmesh-export/ci-graphs/latest/plantuml/static.png) | ![PlantUML cycles](https://raw.githubusercontent.com/hovsep/fmesh-export/ci-graphs/latest/plantuml/cycles.gif) |

Run it locally: `go run ./internal/cmd/showcase -format d2 out`.

## Writing a new exporter

Copy the [`d2`](d2) package: it is a complete exporter with the full set of tests. A new format
`xyz` needs:

1. **A package** `xyz/` with `xyz.go`, `xyz_test.go` and a `README.md`.
2. **The shared plugin API**, the same as every other format:
   - `New(opts ...Option) *Plugin`, `Name() string`, `Init(*fmesh.FMesh) error`
   - `Export() ([]byte, error)` and `ExportCycles() ([][]byte, error)`
   - a package function `Export(fm *fmesh.FMesh, opts ...Option) ([]byte, error)`
   - `WithCycles()`, plus single options that change one thing on top of the defaults. No config
     struct that replaces everything.
3. **Rendering through `fm.Walk(visitor)`.** Implement `VisitMesh`, `VisitComponent`, `VisitPort`
   and `VisitPipe`. Rules:
   - Never change the mesh.
   - The output must be byte-identical for equal meshes.
   - Use generated IDs (`c1`, `p2`, ...) and put names only in quoted, escaped labels.
   - In a cycle graph, a component with no result had no input (`ActivationCodeNoInput`).
4. **The `Init` rules** that every exporter shares:
   - Validate the options in `Init` **and** in the package `Export`.
   - A second `Init` with the same mesh is a no-op; a different mesh is an error.
   - With `WithCycles`, reset the record in `BeforeRun` and record in `BeforeCycle`. Not in
     `AfterCycle`: another plugin's failing `AfterCycle` hook could skip it.
5. **Tests:** one exact expected output, cycle colors with an error, determinism, awkward names
   (quotes, dots, colons, newlines), option checks, both error values, a second `Init`, and a
   failing `AfterCycle` hook from another plugin. The `d2` tests cover all of these.
6. **The showcase:** add the format to `formats` in [`internal/cmd/showcase`](internal/cmd/showcase/main.go).
7. **CI:** copy `.github/workflows/d2.yml` to `xyz.yml`, and add an `xyz)` case to the
   "Install renderer" and "Render images" steps of `render.yml`. Pin the renderer version and check
   its SHA-256 there.
8. **Docs:** add a row to the format table and the live-example table above.

Run `make check` before you open a pull request. The PR's checks upload the rendered images as an
artifact, so you can look at your format before it is merged.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

See [LICENSE](LICENSE).
