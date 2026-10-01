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
go get github.com/hovsep/fmesh-export
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

## License

See [LICENSE](LICENSE).
