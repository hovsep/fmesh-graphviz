# yaml

Export an [F-Mesh](https://github.com/hovsep/fmesh) mesh as YAML: its structure, or its state in one
cycle of a run. Use it for config-style review, diffs in pull requests, or snapshots in tests.

`New()` returns an `Exporter`. It implements this module's
[`export.Exporter`](https://pkg.go.dev/github.com/hovsep/fmesh-export), like every format in this
module. It has no options and no state: reuse one value for many meshes.

The documents are exactly the ones the [`json`](../json) exporter writes, in YAML syntax. Both
packages share the types: `yaml.Mesh` is `json.Mesh`.

## Install

```bash
go get github.com/hovsep/fmesh-export/yaml
```

## Structure

```go
import "github.com/hovsep/fmesh-export/yaml"

data, err := yaml.New().Export(fm)
```

```yaml
name: pricing
description: computes prices
meta:
  env: prod
components:
  - name: dst
    inputs:
      - name: in
        description: prices in
    outputs: []
  - name: src
    description: emits prices
    meta:
      weight: 2
    inputs:
      - name: start
    outputs:
      - name: out
pipes:
  - from:
      component: src
      port: out
    to:
      component: dst
      port: in
```

- Components and ports are in name order, pipes in wiring order, metadata keys sorted. The same mesh
  always exports the same bytes.
- `description` and `meta` are left out when empty. `components`, `inputs`, `outputs` and `pipes`
  are always present.
- Names are quoted when YAML needs it (`a: b`, `#x`, `yes`, `null`, ...), so they read back as
  written.
- Unmarshal into `yaml.Mesh` to read it back, for example with `go.yaml.in/yaml/v3`.
- A float metadata value with no fraction is written as an integer (`2.0` → `2`), so it reads back
  as an `int` into `map[string]any`.

## One cycle

```go
ri, err := fm.Run(ctx)
for _, c := range ri.Cycles.All() {
    frame, err := yaml.New().ExportCycle(fm, c)
}
```

The document is a `yaml.Cycle`: the cycle `number`, the `mesh` structure, and one `results` entry
per component, in name order:

```yaml
results:
  - component: dst
    code: Finished with error
    activated: true
    errors:
      - 'component returned an error: boom'
  - component: src
    code: No input
    activated: false
```

- A component with no result in the cycle had no input: its code is `No input`.
- A nil mesh returns `export.ErrNilMesh`, and a nil cycle `export.ErrNilCycle`.
- To export each cycle while the mesh runs, call `ExportCycle` from an `AfterCycle` hook.
