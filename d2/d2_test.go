package d2

import (
	"context"
	"errors"
	"testing"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh/component"
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
func pairMesh(t *testing.T, plugin *Plugin, failDst bool) *fmesh.FMesh {
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

	fm := mustNewFMesh(t, "pair", fmesh.WithPlugins(plugin),
		fmesh.WithErrorHandlingStrategy(fmesh.IgnoreAll))
	require.NoError(t, fm.AddComponents(src, dst))
	require.NoError(t, src.InputByName("in").PutSignals(signal.New(1)))
	return fm
}

func TestPlugin_Export(t *testing.T) {
	t.Run("diagram of the structure", func(t *testing.T) {
		plugin := New()
		pairMesh(t, plugin, false)

		got, err := plugin.Export()
		require.NoError(t, err)
		assert.Equal(t, `direction: right
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
`, string(got))
	})

	t.Run("deterministic", func(t *testing.T) {
		first, err := Export(pairMesh(t, New(), false))
		require.NoError(t, err)
		for range 10 {
			again, err := Export(pairMesh(t, New(), false))
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

		got, err := Export(fm)
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

	t.Run("not attached", func(t *testing.T) {
		_, err := New().Export()
		require.ErrorIs(t, err, ErrNotAttached)
	})
}

func TestPlugin_ExportCycles(t *testing.T) {
	t.Run("colors components by result", func(t *testing.T) {
		plugin := New(WithCycles())
		fm := pairMesh(t, plugin, true)
		ri, err := fm.Run(t.Context())
		require.NoError(t, err)

		diagrams, err := plugin.ExportCycles()
		require.NoError(t, err)
		require.Len(t, diagrams, ri.Cycles.Len())

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
`, string(diagrams[1]))

		first := string(diagrams[0])
		assert.Contains(t, first, `title: "pair — cycle 1"`)
		assert.Contains(t, first, "c1: \"dst\" {\n  style.stroke: \"gold\"", "no result means no input")
		assert.Contains(t, first, "style.stroke: \"green\"")
	})

	t.Run("WithResultColor changes one code and keeps the rest", func(t *testing.T) {
		plugin := New(WithCycles(), WithResultColor(component.ActivationCodeOK, "teal"))
		fm := pairMesh(t, plugin, true)
		_, err := fm.Run(t.Context())
		require.NoError(t, err)

		diagrams, err := plugin.ExportCycles()
		require.NoError(t, err)
		first := string(diagrams[0])
		assert.Contains(t, first, `style.stroke: "teal"`)
		assert.Contains(t, first, `style.stroke: "gold"`, "codes the option does not name keep their defaults")
		assert.NotContains(t, first, `"green"`)
	})

	t.Run("recording is opt-in", func(t *testing.T) {
		plugin := New()
		pairMesh(t, plugin, false)
		_, err := plugin.ExportCycles()
		require.ErrorIs(t, err, ErrCyclesNotRecorded)
	})

	t.Run("not attached", func(t *testing.T) {
		_, err := New(WithCycles()).ExportCycles()
		require.ErrorIs(t, err, ErrNotAttached)
	})
}

func TestExport(t *testing.T) {
	t.Run("equals the plugin export", func(t *testing.T) {
		plugin := New(WithDirection("down"))
		fm := pairMesh(t, plugin, false)

		want, err := plugin.Export()
		require.NoError(t, err)
		got, err := Export(fm, WithDirection("down"))
		require.NoError(t, err)
		assert.Equal(t, string(want), string(got))
		assert.Contains(t, string(got), "direction: down\n")
	})

	t.Run("rejects an unknown direction", func(t *testing.T) {
		_, err := Export(pairMesh(t, New(), false), WithDirection("sideways"))
		require.ErrorContains(t, err, "unknown direction")
	})
}

func TestPlugin_Init(t *testing.T) {
	t.Run("one mesh per instance", func(t *testing.T) {
		plugin := New()
		mustNewFMesh(t, "first", fmesh.WithPlugins(plugin))
		_, err := fmesh.New("second", fmesh.WithPlugins(plugin))
		require.ErrorContains(t, err, "already attached")
	})

	t.Run("rejects an unknown direction", func(t *testing.T) {
		_, err := fmesh.New("m", fmesh.WithPlugins(New(WithDirection("LR"))))
		require.ErrorContains(t, err, "unknown direction")
	})
}

// failingAfterCycle is a plugin whose AfterCycle hook fails. Plugins initialize
// in name order and its name sorts first, so its hook runs before any the
// exporter registers.
type failingAfterCycle struct{}

func (failingAfterCycle) Name() string { return "0-failing-after-cycle" }

func (failingAfterCycle) Init(fm *fmesh.FMesh) error {
	fm.SetupHooks(func(h *fmesh.Hooks) {
		h.AfterCycle(func(context.Context, *fmesh.CycleContext) error {
			return errors.New("after cycle failed")
		})
	})
	return nil
}

// oneComponentMesh builds a seeded single-component mesh with the given plugins.
func oneComponentMesh(t *testing.T, plugins ...fmesh.Plugin) *fmesh.FMesh {
	t.Helper()
	fm := mustNewFMesh(t, "one", fmesh.WithPlugins(plugins...))
	c := mustNewComponent(t, "c",
		component.WithInputs("in"),
		component.WithActivationFunc(func(context.Context, *component.Component) error { return nil }))
	require.NoError(t, fm.AddComponents(c))
	require.NoError(t, c.InputByName("in").PutSignals(signal.New(1)))
	return fm
}

func TestPlugin_RecordsEveryCycleOnce(t *testing.T) {
	t.Run("a second Init with the same mesh registers nothing", func(t *testing.T) {
		plugin := New(WithCycles())
		fm := oneComponentMesh(t, plugin)
		require.NoError(t, plugin.Init(fm))

		ri, err := fm.Run(t.Context())
		require.NoError(t, err)
		graphs, err := plugin.ExportCycles()
		require.NoError(t, err)
		assert.Len(t, graphs, ri.Cycles.Len())
	})

	t.Run("another plugin's failing AfterCycle hook does not drop the cycle", func(t *testing.T) {
		plugin := New(WithCycles())
		fm := oneComponentMesh(t, failingAfterCycle{}, plugin)

		ri, err := fm.Run(t.Context())
		require.Error(t, err)
		graphs, err := plugin.ExportCycles()
		require.NoError(t, err)
		assert.Len(t, graphs, ri.Cycles.Len())
	})
}
