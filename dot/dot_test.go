package dot

import (
	"context"
	"regexp"
	"testing"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/cycle"
	"github.com/hovsep/fmesh/port"
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

// calcMesh builds adder -> multiplier and seeds the adder.
func calcMesh(t *testing.T) *fmesh.FMesh {
	t.Helper()
	adder := mustNewComponent(t, "adder",
		component.WithDescription("adds 2 numbers"),
		component.WithInputs("num1", "num2"),
		component.WithOutputs("result"),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			num1, err := this.InputByName("num1").Signals().FirstAs[int]()
			if err != nil {
				return err
			}
			num2, err := this.InputByName("num2").Signals().FirstAs[int]()
			if err != nil {
				return err
			}
			return this.OutputByName("result").PutSignals(signal.New(num1 + num2))
		}))
	multiplier := mustNewComponent(t, "multiplier",
		component.WithDescription("multiplies by 3"),
		component.WithInputs("num"),
		component.WithOutputs("result"),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			num, err := this.InputByName("num").Signals().FirstAs[int]()
			if err != nil {
				return err
			}
			return this.OutputByName("result").PutSignals(signal.New(num * 3))
		}))
	require.NoError(t, adder.OutputByName("result").PipeTo(multiplier.InputByName("num")))

	fm := mustNewFMesh(t, "fm", fmesh.WithDescription("calculator"))
	require.NoError(t, fm.AddComponents(multiplier, adder))
	require.NoError(t, adder.InputByName("num1").PutSignals(signal.New(15)))
	require.NoError(t, adder.InputByName("num2").PutSignals(signal.New(12)))
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
	graphs := make([]string, 0, ri.Cycles.Len())
	for _, c := range ri.Cycles.All() {
		graph, err := e.ExportCycle(fm, c)
		require.NoError(t, err)
		graphs = append(graphs, string(graph))
	}
	return graphs
}

func TestExporter_Export(t *testing.T) {
	t.Run("empty mesh exports nothing", func(t *testing.T) {
		fm := mustNewFMesh(t, "fm")

		got, err := New().Export(fm)
		require.NoError(t, err)
		assert.Empty(t, got)
		got, err = New().ExportCycle(fm, cycle.New())
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("draws components, ports and pipes", func(t *testing.T) {
		got, err := New().Export(calcMesh(t))
		require.NoError(t, err)
		graph := string(got)
		assert.Contains(t, graph, "adds 2 numbers")
		assert.Contains(t, graph, `label="result"`)
		// Exactly one pipe edge, drawn with the pipe style.
		assert.Len(t, regexp.MustCompile(`n\d+->n\d+\[color="#e437ea"`).FindAllString(graph, -1), 1)
	})

	t.Run("output is stable", func(t *testing.T) {
		// Two exports of equal meshes must be byte-identical.
		a, err := New().Export(calcMesh(t))
		require.NoError(t, err)
		b, err := New().Export(calcMesh(t))
		require.NoError(t, err)
		assert.Equal(t, string(a), string(b))
	})

	t.Run("does not change the mesh", func(t *testing.T) {
		fm := calcMesh(t)

		_, err := New().Export(fm)
		require.NoError(t, err)
		for _, c := range fm.Components().AllOrdered() {
			for _, p := range append(c.Inputs().AllOrdered(), c.Outputs().AllOrdered()...) {
				assert.True(t, p.Meta().IsEmpty(), "port %s.%s got metadata", c.Name(), p.Name())
			}
		}
	})

	t.Run("rejects a pipe out of the mesh", func(t *testing.T) {
		fm := mustNewFMesh(t, "fm")
		c := mustNewComponent(t, "c", component.WithOutputs("out"),
			component.WithActivationFunc(func(context.Context, *component.Component) error { return nil }))
		require.NoError(t, fm.AddComponents(c))
		stray, err := port.NewInput("stray")
		require.NoError(t, err)
		require.NoError(t, c.OutputByName("out").PipeTo(stray))

		_, err = New().Export(fm)
		require.ErrorContains(t, err, "not in the mesh")
	})
}

func TestExporter_ExportCycle(t *testing.T) {
	graphs := runAndExportCycles(t, New(), calcMesh(t))
	require.NotEmpty(t, graphs)
	assert.Contains(t, graphs[0], "Hook failed")
	// Cycle 1: only the adder has input; the multiplier counts as "No input".
	assert.Regexp(t, `No input:</td><td>1<`, graphs[0])
	assert.Regexp(t, `Cycle:</td><td>1<`, graphs[0])
}

func TestExporter_Options(t *testing.T) {
	t.Run("WithAttrs merges over the defaults", func(t *testing.T) {
		got, err := New(WithAttrs(Pipe, map[string]string{attrColor: "blue"})).Export(calcMesh(t))
		require.NoError(t, err)
		graph := string(got)
		assert.Contains(t, graph, `color="blue"`)
		assert.NotContains(t, graph, "#e437ea", "the overridden default must be gone")
		assert.Contains(t, graph, `minlen="3"`, "attributes the option does not name keep their defaults")
	})

	t.Run("WithResultAttrs styles a cycle graph by result", func(t *testing.T) {
		e := New(WithResultAttrs(component.ActivationCodeOK, map[string]string{attrColor: "gold"}))
		graphs := runAndExportCycles(t, e, calcMesh(t))
		assert.Contains(t, graphs[0], `color="gold"`)
	})

	t.Run("WithComponentLabel labels components with no description", func(t *testing.T) {
		got, err := New(WithComponentLabel("fn")).Export(oneMesh(t))
		require.NoError(t, err)
		assert.Contains(t, string(got), `label="fn"`)
	})
}

func TestExporter_Reuse(t *testing.T) {
	// An exporter keeps no state between calls, so one value serves many meshes.
	e := New(WithAttrs(Pipe, map[string]string{attrColor: "navy"}))
	calc, other := calcMesh(t), oneMesh(t)

	first, err := e.Export(calc)
	require.NoError(t, err)
	fromOther, err := e.Export(other)
	require.NoError(t, err)
	again, err := e.Export(calc)
	require.NoError(t, err)
	fresh, err := New(WithAttrs(Pipe, map[string]string{attrColor: "navy"})).Export(other)
	require.NoError(t, err)

	assert.Equal(t, string(first), string(again))
	assert.Equal(t, string(fresh), string(fromOther))
	assert.Contains(t, string(first), `color="navy"`, "options apply to every export")
}

func TestExporter_ExportCycleFromHook(t *testing.T) {
	// Streaming: one graph per cycle while the mesh runs, equal to exporting
	// the recorded cycles afterwards.
	e := New()
	fm := calcMesh(t)
	var frames []string
	fm.SetupHooks(func(h *fmesh.Hooks) {
		h.AfterCycle(func(_ context.Context, cc *fmesh.CycleContext) error {
			graph, err := e.ExportCycle(cc.FMesh, cc.Cycle)
			frames = append(frames, string(graph))
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
