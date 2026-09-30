package dot

import (
	"context"
	"regexp"
	"testing"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh/component"
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

// calcMesh builds adder -> multiplier with the plugin attached and seeds the adder.
func calcMesh(t *testing.T, plugin *Plugin, opts ...fmesh.Option) *fmesh.FMesh {
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

	fm := mustNewFMesh(t, "fm", append(opts, fmesh.WithDescription("calculator"), fmesh.WithPlugins(plugin))...)
	require.NoError(t, fm.AddComponents(multiplier, adder))
	require.NoError(t, adder.InputByName("num1").PutSignals(signal.New(15)))
	require.NoError(t, adder.InputByName("num2").PutSignals(signal.New(12)))
	return fm
}

func TestPlugin_Export(t *testing.T) {
	t.Run("empty mesh exports nothing", func(t *testing.T) {
		plugin := New()
		mustNewFMesh(t, "fm", fmesh.WithPlugins(plugin))

		got, err := plugin.Export()
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("draws components, ports and pipes", func(t *testing.T) {
		plugin := New()
		calcMesh(t, plugin)

		got, err := plugin.Export()
		require.NoError(t, err)
		graph := string(got)
		assert.Contains(t, graph, "adds 2 numbers")
		assert.Contains(t, graph, `label="result"`)
		// Exactly one pipe edge, drawn with the pipe style.
		assert.Len(t, regexp.MustCompile(`n\d+->n\d+\[color="#e437ea"`).FindAllString(graph, -1), 1)
	})

	t.Run("output is stable", func(t *testing.T) {
		// Two exports of equal meshes must be byte-identical.
		first, second := New(), New()
		calcMesh(t, first)
		calcMesh(t, second)

		a, err := first.Export()
		require.NoError(t, err)
		b, err := second.Export()
		require.NoError(t, err)
		assert.Equal(t, string(a), string(b))
	})

	t.Run("does not change the mesh", func(t *testing.T) {
		plugin := New()
		fm := calcMesh(t, plugin)

		_, err := plugin.Export()
		require.NoError(t, err)
		for _, c := range fm.Components().AllOrdered() {
			for _, p := range append(c.Inputs().AllOrdered(), c.Outputs().AllOrdered()...) {
				assert.True(t, p.Meta().IsEmpty(), "port %s.%s got metadata", c.Name(), p.Name())
			}
		}
	})

	t.Run("not attached", func(t *testing.T) {
		_, err := New().Export()
		require.ErrorIs(t, err, ErrNotAttached)
	})
}

func TestPlugin_ExportCycles(t *testing.T) {
	t.Run("one graph per cycle, with stats", func(t *testing.T) {
		plugin := New(WithCycles())
		fm := calcMesh(t, plugin)

		ri, err := fm.Run(t.Context())
		require.NoError(t, err)

		graphs, err := plugin.ExportCycles()
		require.NoError(t, err)
		require.Len(t, graphs, ri.Cycles.Len())
		assert.Contains(t, string(graphs[0]), "Hook failed")
		// Cycle 1: only the adder has input; the multiplier counts as "No input".
		assert.Regexp(t, `No input:</td><td>1<`, string(graphs[0]))
	})

	t.Run("a cycles history limit does not cut the replay", func(t *testing.T) {
		plugin := New(WithCycles())
		fm := calcMesh(t, plugin, fmesh.WithCyclesHistoryLimit(1))

		_, err := fm.Run(t.Context())
		require.NoError(t, err)

		graphs, err := plugin.ExportCycles()
		require.NoError(t, err)
		assert.Len(t, graphs, 3)
	})

	t.Run("recording is opt-in", func(t *testing.T) {
		plugin := New()
		calcMesh(t, plugin)

		_, err := plugin.ExportCycles()
		require.ErrorIs(t, err, ErrCyclesNotRecorded)
	})
}

func TestPlugin_Options(t *testing.T) {
	t.Run("WithAttrs merges over the defaults", func(t *testing.T) {
		plugin := New(WithAttrs(Pipe, map[string]string{attrColor: "blue"}))
		calcMesh(t, plugin)

		got, err := plugin.Export()
		require.NoError(t, err)
		graph := string(got)
		assert.Contains(t, graph, `color="blue"`)
		assert.NotContains(t, graph, "#e437ea", "the overridden default must be gone")
		assert.Contains(t, graph, `minlen="3"`, "attributes the option does not name keep their defaults")
	})

	t.Run("WithResultAttrs styles a cycle graph by result", func(t *testing.T) {
		plugin := New(WithCycles(), WithResultAttrs(component.ActivationCodeOK, map[string]string{attrColor: "gold"}))
		fm := calcMesh(t, plugin)
		_, err := fm.Run(t.Context())
		require.NoError(t, err)

		graphs, err := plugin.ExportCycles()
		require.NoError(t, err)
		assert.Contains(t, string(graphs[0]), `color="gold"`)
	})

	t.Run("WithComponentLabel labels components with no description", func(t *testing.T) {
		plugin := New(WithComponentLabel("fn"))
		fm := mustNewFMesh(t, "fm", fmesh.WithPlugins(plugin))
		require.NoError(t, fm.AddComponents(mustNewComponent(t, "plain",
			component.WithInputs("in"),
			component.WithActivationFunc(func(context.Context, *component.Component) error { return nil }))))

		got, err := plugin.Export()
		require.NoError(t, err)
		assert.Contains(t, string(got), `label="fn"`)
	})
}

func TestExport(t *testing.T) {
	// The package function renders exactly what the plugin renders, options included.
	plugin := New(WithAttrs(Pipe, map[string]string{attrColor: "navy"}))
	fm := calcMesh(t, plugin)

	want, err := plugin.Export()
	require.NoError(t, err)
	got, err := Export(fm, WithAttrs(Pipe, map[string]string{attrColor: "navy"}))
	require.NoError(t, err)
	assert.Equal(t, string(want), string(got))
}

func TestPlugin_Init(t *testing.T) {
	plugin := New()
	mustNewFMesh(t, "first", fmesh.WithPlugins(plugin))

	_, err := fmesh.New("second", fmesh.WithPlugins(plugin))
	require.ErrorContains(t, err, "already attached")
}

func TestPlugin_ExportRejectsPipeOutOfMesh(t *testing.T) {
	plugin := New()
	fm := mustNewFMesh(t, "fm", fmesh.WithPlugins(plugin))
	c := mustNewComponent(t, "c", component.WithOutputs("out"),
		component.WithActivationFunc(func(context.Context, *component.Component) error { return nil }))
	require.NoError(t, fm.AddComponents(c))
	stray, err := port.NewInput("stray")
	require.NoError(t, err)
	require.NoError(t, c.OutputByName("out").PipeTo(stray))

	_, err = plugin.Export()
	require.ErrorContains(t, err, "not in the mesh")
}
