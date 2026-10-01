package yaml

import (
	"context"
	stdjson "encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	goyaml "go.yaml.in/yaml/v3"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-export"
	"github.com/hovsep/fmesh-export/json"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/port"
	"github.com/hovsep/fmesh/signal"
)

func noop(context.Context, *component.Component) error { return nil }

func mustNewFMesh(t *testing.T, name string, opts ...fmesh.Option) *fmesh.FMesh {
	t.Helper()
	fm, err := fmesh.New(name, opts...)
	require.NoError(t, err)
	return fm
}

func mustNewComponent(t *testing.T, name string, opts ...component.Option) *component.Component {
	t.Helper()
	c, err := component.New(name, opts...)
	require.NoError(t, err)
	return c
}

// pricingMesh is src -> dst with descriptions and metadata on every level.
func pricingMesh(t *testing.T, dstFunc component.ActivationFunc) *fmesh.FMesh {
	t.Helper()
	fm := mustNewFMesh(t, "pricing",
		fmesh.WithDescription("computes prices"),
		fmesh.WithMeta("env", "prod"),
		fmesh.WithErrorHandlingStrategy(fmesh.IgnoreAll))
	src := mustNewComponent(t, "src",
		component.WithDescription("emits prices"),
		component.WithMeta("weight", 2.0),
		component.WithInputs("start"),
		component.WithOutputs("out"),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			return this.OutputByName("out").PutPayloads(10)
		}))
	in, err := port.NewInput("in", port.WithDescription("prices in"))
	require.NoError(t, err)
	dst := mustNewComponent(t, "dst", component.WithActivationFunc(dstFunc))
	require.NoError(t, dst.AttachInputPorts(in))
	require.NoError(t, fm.AddComponents(dst, src))
	require.NoError(t, src.OutputByName("out").PipeTo(dst.InputByName("in")))
	return fm
}

const pricingStructure = `name: pricing
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
`

func TestExporter_Export(t *testing.T) {
	t.Run("structure, descriptions and metadata", func(t *testing.T) {
		got, err := New().Export(pricingMesh(t, noop))
		require.NoError(t, err)
		assert.Equal(t, pricingStructure, string(got))
	})

	t.Run("empty mesh exports empty lists", func(t *testing.T) {
		got, err := New().Export(mustNewFMesh(t, "empty"))
		require.NoError(t, err)
		assert.Equal(t, "name: empty\ncomponents: []\npipes: []\n", string(got))
	})

	t.Run("deterministic", func(t *testing.T) {
		a, err := New().Export(pricingMesh(t, noop))
		require.NoError(t, err)
		b, err := New().Export(pricingMesh(t, noop))
		require.NoError(t, err)
		assert.Equal(t, string(a), string(b))
	})

	t.Run("names that need quoting survive a round trip", func(t *testing.T) {
		// YAML gives meaning to ": ", "#", "- ", quotes and words like "yes".
		awkward := []string{`a: b`, `#hash`, `- dash`, `"quoted"`, `it's`, `yes`, `null`, "two\nlines"}
		fm := mustNewFMesh(t, "m")
		for _, name := range awkward {
			require.NoError(t, fm.AddComponents(mustNewComponent(t, name,
				component.WithDescription(name), component.WithActivationFunc(noop))))
		}

		got, err := New().Export(fm)
		require.NoError(t, err)
		var mesh Mesh
		require.NoError(t, goyaml.Unmarshal(got, &mesh))
		names := make([]string, 0, len(mesh.Components))
		for _, c := range mesh.Components {
			names = append(names, c.Name)
			assert.Equal(t, c.Name, c.Description)
		}
		assert.ElementsMatch(t, awkward, names)
	})

	t.Run("same document as the json exporter", func(t *testing.T) {
		fm := pricingMesh(t, noop)
		y, err := New().Export(fm)
		require.NoError(t, err)
		j, err := json.New().Export(fm)
		require.NoError(t, err)

		var fromYAML, fromJSON any
		require.NoError(t, goyaml.Unmarshal(y, &fromYAML))
		require.NoError(t, stdjson.Unmarshal(j, &fromJSON))
		// Re-encode both as JSON to compare data, not syntax (YAML reads 2 as an int).
		a, err := stdjson.Marshal(fromYAML)
		require.NoError(t, err)
		b, err := stdjson.Marshal(fromJSON)
		require.NoError(t, err)
		assert.JSONEq(t, string(b), string(a))
	})
}

func TestExporter_ExportCycle(t *testing.T) {
	fm := pricingMesh(t, func(context.Context, *component.Component) error { return errors.New("boom") })
	require.NoError(t, fm.ComponentByName("src").InputByName("start").PutSignals(signal.New(1)))
	ri, err := fm.Run(t.Context())
	require.NoError(t, err)
	// Cycle 1: src runs. Cycle 2: dst fails. Cycle 3: nothing runs, the mesh stops.
	require.Equal(t, 3, ri.Cycles.Len())

	t.Run("results per component, a missing one as no input", func(t *testing.T) {
		got, err := New().ExportCycle(fm, ri.Cycles.All()[0])
		require.NoError(t, err)
		var doc Cycle
		require.NoError(t, goyaml.Unmarshal(got, &doc))

		assert.Equal(t, 1, doc.Number)
		assert.Equal(t, "pricing", doc.Mesh.Name)
		assert.Equal(t, []Result{
			{Component: "dst", Code: component.ActivationCodeNoInput.String()},
			{Component: "src", Code: component.ActivationCodeOK.String(), Activated: true},
		}, doc.Results)
	})

	t.Run("errors are listed", func(t *testing.T) {
		got, err := New().ExportCycle(fm, ri.Cycles.All()[1])
		require.NoError(t, err)
		var doc Cycle
		require.NoError(t, goyaml.Unmarshal(got, &doc))

		require.Len(t, doc.Results, 2)
		dst := doc.Results[0]
		assert.Equal(t, component.ActivationCodeReturnedError.String(), dst.Code)
		assert.True(t, dst.Activated)
		require.Len(t, dst.Errors, 1)
		assert.Contains(t, dst.Errors[0], "boom")
	})

	t.Run("a nil cycle is an error", func(t *testing.T) {
		_, err := New().ExportCycle(fm, nil)
		require.ErrorIs(t, err, export.ErrNilCycle)
	})

	t.Run("works from an AfterCycle hook", func(t *testing.T) {
		// Streaming: export each cycle as it ends, with no history kept.
		var frames [][]byte
		live := pricingMesh(t, noop)
		live.SetupHooks(func(h *fmesh.Hooks) {
			h.AfterCycle(func(_ context.Context, cc *fmesh.CycleContext) error {
				frame, err := New().ExportCycle(cc.FMesh, cc.Cycle)
				frames = append(frames, frame)
				return err
			})
		})
		require.NoError(t, live.ComponentByName("src").InputByName("start").PutSignals(signal.New(1)))
		ri, err := live.Run(t.Context())
		require.NoError(t, err)
		assert.Len(t, frames, ri.Cycles.Len())
	})
}

func TestExporter_Reuse(t *testing.T) {
	// No state: one exporter serves any number of meshes.
	e := New()
	a, err := e.Export(pricingMesh(t, noop))
	require.NoError(t, err)
	b, err := e.Export(mustNewFMesh(t, "empty"))
	require.NoError(t, err)
	assert.Equal(t, pricingStructure, string(a))
	assert.Equal(t, "name: empty\ncomponents: []\npipes: []\n", string(b))
}
