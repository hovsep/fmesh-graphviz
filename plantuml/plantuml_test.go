package plantuml

import (
	"context"
	"errors"
	"testing"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-export"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/cycle"
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

const pairDiagram = `@startuml
title pair
left to right direction
component "dst" as c1 {
  portin "in" as p2
}
component "src\nsays <U+0022>hi<U+0022>" as c3 {
  portin "in" as p4
  portout "out" as p5
}
p5 --> p2
@enduml
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
		second, err := New().Export(pairMesh(t, false))
		require.NoError(t, err)
		assert.Equal(t, string(first), string(second))
	})

	t.Run("escapes awkward names", func(t *testing.T) {
		c := mustNewComponent(t, `my "comp": v1.2`,
			component.WithDescription("line one\n<b>line</b> two \\n"),
			component.WithInputs("in put"),
			component.WithOutputs(`out"x`),
			component.WithActivationFunc(func(context.Context, *component.Component) error { return nil }))
		fm := mustNewFMesh(t, `mesh <x>`, fmesh.WithDescription("a\nb"))
		require.NoError(t, fm.AddComponents(c))

		got, err := New(WithDirection("top to bottom")).Export(fm)
		require.NoError(t, err)
		assert.Equal(t, `@startuml
title mesh <U+003C>x>
top to bottom direction
' a b
component "my <U+0022>comp<U+0022>: v1.2\nline one <U+003C>b>line<U+003C>/b> two <U+005C>n" as c1 {
  portin "in put" as p2
  portout "out<U+0022>x" as p3
}
@enduml
`, string(got))
	})
}

func TestExporter_ExportCycle(t *testing.T) {
	t.Run("colors components by result", func(t *testing.T) {
		diagrams := runAndExportCycles(t, New(), pairMesh(t, true))
		require.GreaterOrEqual(t, len(diagrams), 2)

		first, second := diagrams[0], diagrams[1]
		assert.Contains(t, first, "title pair — cycle 1\n")
		assert.Contains(t, first, `component "dst" as c1 #line:gold;line.bold {`) // dst had no input yet
		assert.Contains(t, first, `as c3 #line:green;line.bold {`)
		assert.NotContains(t, first, "note")
		assert.Contains(t, second, "title pair — cycle 2\n")
		assert.Contains(t, second, `component "dst" as c1 #line:red;line.bold {`)
		assert.Contains(t, second, "}\nnote bottom of c1 : component returned an error: boom\n")
	})

	t.Run("WithResultColor changes one code and keeps the rest", func(t *testing.T) {
		diagrams := runAndExportCycles(t, New(WithResultColor(component.ActivationCodeOK, "#00AA00")), pairMesh(t, true))
		first := diagrams[0]
		assert.Contains(t, first, "as c3 #line:00AA00;line.bold {")
		assert.Contains(t, first, "as c1 #line:gold;line.bold {", "codes the option does not name keep their defaults")
	})

	t.Run("joins the errors of several attempts", func(t *testing.T) {
		fm := mustNewFMesh(t, "retry", fmesh.WithErrorHandlingStrategy(fmesh.IgnoreAll))
		require.NoError(t, fm.AddComponents(mustNewComponent(t, "c",
			component.WithInputs("in"),
			component.WithRetry(2),
			component.WithActivationFunc(func(context.Context, *component.Component) error {
				return errors.New("boom")
			}))))
		require.NoError(t, fm.ComponentByName("c").InputByName("in").PutSignals(signal.New(1)))

		diagrams := runAndExportCycles(t, New(), fm)
		require.NotEmpty(t, diagrams)
		assert.Contains(t, diagrams[0], "note bottom of c1 : component returned an error: attempt 1 of 2: boom; component returned an error: attempt 2 of 2: boom\n")
	})
}

func TestExporter_Options(t *testing.T) {
	t.Run("WithDirection", func(t *testing.T) {
		got, err := New(WithDirection("top to bottom")).Export(pairMesh(t, false))
		require.NoError(t, err)
		assert.Contains(t, string(got), "\ntop to bottom direction\n")
	})

	t.Run("rejects an unknown direction", func(t *testing.T) {
		e := New(WithDirection("sideways"))
		fm := pairMesh(t, false)
		_, err := e.Export(fm)
		require.ErrorContains(t, err, `unknown direction "sideways"`)
		_, err = e.ExportCycle(fm, cycle.New())
		require.ErrorContains(t, err, `unknown direction "sideways"`)
	})
}

func TestExporter_Reuse(t *testing.T) {
	// An exporter keeps no state between calls, so one value serves many meshes.
	e := New(WithDirection("top to bottom"))
	pair, other := pairMesh(t, false), oneMesh(t)

	first, err := e.Export(pair)
	require.NoError(t, err)
	fromOther, err := e.Export(other)
	require.NoError(t, err)
	again, err := e.Export(pair)
	require.NoError(t, err)
	fresh, err := New(WithDirection("top to bottom")).Export(other)
	require.NoError(t, err)

	assert.Equal(t, string(first), string(again))
	assert.Equal(t, string(fresh), string(fromOther))
	assert.Contains(t, string(first), "\ntop to bottom direction\n", "options apply to every export")
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

func TestExporter_RejectsNilMesh(t *testing.T) {
	_, err := New().Export(nil)
	require.ErrorIs(t, err, export.ErrNilMesh)
	_, err = New().ExportCycle(nil, cycle.New())
	require.ErrorIs(t, err, export.ErrNilMesh)
}
