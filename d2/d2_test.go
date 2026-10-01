package d2

import (
	"context"
	"errors"
	"testing"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/cycle"
	"github.com/hovsep/fmesh/export"
	"github.com/hovsep/fmesh/signal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

// pairMesh builds src -> dst; dst fails when failDst is set.
func pairMesh(t *testing.T, failDst bool) *fmesh.FMesh {
	t.Helper()
	src := mustNewComponent(t, "src",
		component.WithDescription(`says "hi"`),
		component.WithInputs("in"),
		component.WithOutputs("out"),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			return this.OutputByName("out").PutSignalGroups(this.InputByName("in").Signals())
		}))
	dst := mustNewComponent(t, "dst",
		component.WithInputs("in"),
		component.WithActivationFunc(func(context.Context, *component.Component) error {
			if failDst {
				return errors.New("boom")
			}
			return nil
		}))
	require.NoError(t, src.OutputByName("out").PipeTo(dst.InputByName("in")))

	fm := mustNewFMesh(t, "pair", fmesh.WithErrorHandlingStrategy(fmesh.IgnoreAll))
	require.NoError(t, fm.AddComponents(src, dst))
	require.NoError(t, src.InputByName("in").PutSignals(signal.New(1)))
	return fm
}

// oneMesh builds a mesh with one component and no pipes.
func oneMesh(t *testing.T) *fmesh.FMesh {
	t.Helper()
	fm := mustNewFMesh(t, "one")
	require.NoError(t, fm.AddComponents(mustNewComponent(t, "c",
		component.WithInputs("in"),
		component.WithActivationFunc(func(context.Context, *component.Component) error { return nil }))))
	return fm
}

// runAndExportCycles runs fm and exports every cycle of the run, in order.
func runAndExportCycles(t *testing.T, e *Exporter, fm *fmesh.FMesh) []string {
	t.Helper()
	ri, err := fm.Run(t.Context())
	require.NoError(t, err)
	diagrams := make([]string, 0, ri.Cycles.Len())
	for _, c := range ri.Cycles.All() {
		diagram, err := e.ExportCycle(fm, c)
		require.NoError(t, err)
		diagrams = append(diagrams, string(diagram))
	}
	return diagrams
}

const pairDiagram = `direction: right
title: "pair" {
  near: top-center
  shape: text
  style.font-size: 24
}
c1: "dst" {
  p2: "in" {shape: oval}
}
c3: "src\nsays \"hi\"" {
  p4: "in" {shape: oval}
  p5: "out" {shape: oval}
}
c3.p5 -> c1.p2
`

func TestExporter_Export(t *testing.T) {
	t.Run("diagram of the structure", func(t *testing.T) {
		got, err := New().Export(pairMesh(t, false))
		require.NoError(t, err)
		assert.Equal(t, pairDiagram, string(got))
	})

	t.Run("deterministic", func(t *testing.T) {
		first, err := New().Export(pairMesh(t, false))
		require.NoError(t, err)
		for range 10 {
			again, err := New().Export(pairMesh(t, false))
			require.NoError(t, err)
			assert.Equal(t, string(first), string(again))
		}
	})

	t.Run("escapes awkward names", func(t *testing.T) {
		c := mustNewComponent(t, `a.b: "c" ${x}`,
			component.WithDescription("line\\one\n  two"),
			component.WithInputs(`in: 1.2`),
			component.WithActivationFunc(func(context.Context, *component.Component) error { return nil }))
		fm := mustNewFMesh(t, "mesh $name", fmesh.WithDescription("about\nthis"))
		require.NoError(t, fm.AddComponents(c))

		got, err := New().Export(fm)
		require.NoError(t, err)
		assert.Equal(t, `# about this
direction: right
title: "mesh \$name" {
  near: top-center
  shape: text
  style.font-size: 24
}
c1: "a.b: \"c\" \${x}\nline\\one two" {
  p2: "in: 1.2" {shape: oval}
}
`, string(got))
	})
}

func TestExporter_ExportCycle(t *testing.T) {
	t.Run("colors components by result", func(t *testing.T) {
		diagrams := runAndExportCycles(t, New(), pairMesh(t, true))
		require.GreaterOrEqual(t, len(diagrams), 2)

		assert.Equal(t, `direction: right
title: "pair — cycle 2" {
  near: top-center
  shape: text
  style.font-size: 24
}
c1: "dst" {
  style.stroke: "red"
  style.stroke-width: 3
  e2: "component returned an error: boom" {shape: callout}
  p3: "in" {shape: oval}
}
c4: "src\nsays \"hi\"" {
  style.stroke: "gold"
  style.stroke-width: 3
  p5: "in" {shape: oval}
  p6: "out" {shape: oval}
}
c4.p6 -> c1.p3
`, diagrams[1])

		first := diagrams[0]
		assert.Contains(t, first, `title: "pair — cycle 1"`)
		assert.Contains(t, first, "c1: \"dst\" {\n  style.stroke: \"gold\"", "no result means no input")
		assert.Contains(t, first, "style.stroke: \"green\"")
	})

	t.Run("WithResultColor changes one code and keeps the rest", func(t *testing.T) {
		diagrams := runAndExportCycles(t, New(WithResultColor(component.ActivationCodeOK, "teal")), pairMesh(t, true))
		first := diagrams[0]
		assert.Contains(t, first, `style.stroke: "teal"`)
		assert.Contains(t, first, `style.stroke: "gold"`, "codes the option does not name keep their defaults")
		assert.NotContains(t, first, `"green"`)
	})
}

func TestExporter_Options(t *testing.T) {
	t.Run("WithDirection", func(t *testing.T) {
		got, err := New(WithDirection("down")).Export(pairMesh(t, false))
		require.NoError(t, err)
		assert.Contains(t, string(got), "direction: down\n")
	})

	t.Run("rejects an unknown direction", func(t *testing.T) {
		e := New(WithDirection("LR"))
		fm := pairMesh(t, false)
		_, err := e.Export(fm)
		require.ErrorContains(t, err, `unknown direction "LR"`)
		_, err = e.ExportCycle(fm, cycle.New())
		require.ErrorContains(t, err, `unknown direction "LR"`)
	})
}

func TestExporter_Reuse(t *testing.T) {
	// An exporter keeps no state between calls, so one value serves many meshes.
	e := New(WithDirection("down"))
	pair, other := pairMesh(t, false), oneMesh(t)

	first, err := e.Export(pair)
	require.NoError(t, err)
	fromOther, err := e.Export(other)
	require.NoError(t, err)
	again, err := e.Export(pair)
	require.NoError(t, err)
	fresh, err := New(WithDirection("down")).Export(other)
	require.NoError(t, err)

	assert.Equal(t, string(first), string(again))
	assert.Equal(t, string(fresh), string(fromOther))
	assert.Contains(t, string(first), "direction: down\n", "options apply to every export")
}

func TestExporter_ExportCycleFromHook(t *testing.T) {
	// Streaming: one diagram per cycle while the mesh runs, equal to exporting
	// the recorded cycles afterwards.
	e := New()
	fm := pairMesh(t, true)
	var frames []string
	fm.SetupHooks(func(h *fmesh.Hooks) {
		h.AfterCycle(func(_ context.Context, cc *fmesh.CycleContext) error {
			diagram, err := e.ExportCycle(cc.FMesh, cc.Cycle)
			frames = append(frames, string(diagram))
			return err
		})
	})

	ri, err := fm.Run(t.Context())
	require.NoError(t, err)
	require.Len(t, frames, ri.Cycles.Len())
	for i, c := range ri.Cycles.All() {
		want, err := e.ExportCycle(fm, c)
		require.NoError(t, err)
		assert.Equal(t, string(want), frames[i], "cycle %d", c.Number())
	}
}

func TestExporter_ExportCycleRejectsNilCycle(t *testing.T) {
	_, err := New().ExportCycle(oneMesh(t), nil)
	require.ErrorIs(t, err, export.ErrNilCycle)
}
