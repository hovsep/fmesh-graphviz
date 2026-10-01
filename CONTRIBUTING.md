# Contributing

Thanks for helping. Small, focused pull requests are easiest to review.

## Setup

- Go 1.27 (see `go.mod`).
- [golangci-lint](https://golangci-lint.run) for `make lint`.
- Optional, to look at the output: Graphviz (`dot`), PlantUML, D2 and mermaid-cli (`mmdc`).

## Workflow

1. Fork the repo and create a branch.
2. Make your change, with tests.
3. Run `make check`. It runs the format check, the tests with `-race`, and the linter: the same
   checks the pull request needs.
4. Open a pull request. Explain what changed and why.

## Rules

- **Same API in every format.** Every format implements fmesh's `export.Exporter`. A change to
  the shared shape (`New`, options, `Export`, `ExportCycle`) goes into all formats in the same
  pull request.
- **No state.** An `Exporter` holds only its options, so it can be reused and called from an
  `AfterCycle` hook.
- **Deterministic output.** Equal meshes must export byte-identical files, so tests can compare
  exact strings.
- **Never change the mesh.** Exporters only read it, through `fmesh.Walk`.
- **Breaking changes** are allowed while fmesh is pre-production, but say so in the pull request
  title and description.
- **Docs:** keep the READMEs short, in plain English, and in sync with the code.

## Adding a format

See [Writing a new exporter](README.md#writing-a-new-exporter).

## Seeing your change

Every pull request renders the showcase mesh in each format it touches and uploads the images as a
`graph-<format>` artifact on the run page. After a merge, the images also appear on the run's
summary page and in the README.

To render locally:

```bash
go run ./internal/cmd/showcase -format dot out
dot -Tpng out/static.dot -o out/static.png
```

## Releases

Maintainers tag releases as plain semver (`v1.7.0`). A suffix like `v1.7.0-rc1` makes Go treat
the release as a pre-release.
