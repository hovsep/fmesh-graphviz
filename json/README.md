# json

Export an [F-Mesh](https://github.com/hovsep/fmesh) mesh as indented JSON: its structure, or its
state in one cycle of a run. Use it to feed other tools, diff two versions of a mesh, or snapshot a
mesh in a test.

`New()` returns an `Exporter`. It implements this module's
[`export.Exporter`](https://pkg.go.dev/github.com/hovsep/fmesh-export), like every format in this
module. It has no options and no state: reuse one value for many meshes.

## Install

```bash
go get github.com/hovsep/fmesh-export/json
```

## Structure

```go
import "github.com/hovsep/fmesh-export/json"

data, err := json.New().Export(fm)
```

```json
{
  "name": "pricing",
  "meta": {"env": "prod"},
  "components": [
    {"name": "dst", "inputs": [{"name": "in"}], "outputs": []},
    {"name": "src", "description": "emits prices", "inputs": [], "outputs": [{"name": "out"}]}
  ],
  "pipes": [
    {"from": {"component": "src", "port": "out"}, "to": {"component": "dst", "port": "in"}}
  ]
}
```

- Components and ports are in name order, pipes in wiring order. The same mesh always exports the
  same bytes.
- `description` and `meta` are left out when empty. `components`, `inputs`, `outputs` and `pipes`
  are always present.
- Unmarshal into `json.Mesh` to read it back.

## One cycle

```go
ri, err := fm.Run(ctx)
for _, c := range ri.Cycles.All() {
    frame, err := json.New().ExportCycle(fm, c)
}
```

The document is a `json.Cycle`: the cycle `number`, the `mesh` structure, and one `results` entry
per component, in name order:

```json
{"component": "dst", "code": "Finished with error", "activated": true, "errors": ["component returned an error: boom"]}
```

- A component with no result in the cycle had no input: its code is `No input`.
- A nil mesh returns `export.ErrNilMesh`, and a nil cycle `export.ErrNilCycle`.
- To export each cycle while the mesh runs, call `ExportCycle` from an `AfterCycle` hook.

If you also import the standard library's `encoding/json` in the same file, give one of them an
alias.
