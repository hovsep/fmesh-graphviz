package mermaid

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
	t.Run("flowchart of the structure", func(t *testing.T) {
		plugin := New()
		pairMesh(t, plugin, false)

		got, err := plugin.Export()
		require.NoError(t, err)
		assert.Equal(t, `---
title: "pair"
---
flowchart LR
  subgraph s1["dst"]
    c2["dst"]
    p3(("in"))
    p3 --> c2
  end
  subgraph s4["src"]
    c5["says #quot;hi#quot;"]
    p6(("in"))
    p6 --> c5
    p7(("out"))
    c5 --> p7
  end
  p7 ==> p3
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

		charts, err := plugin.ExportCycles()
		require.NoError(t, err)
		require.Len(t, charts, ri.Cycles.Len())

		first, second := string(charts[0]), string(charts[1])
		assert.Contains(t, first, `title: "pair — cycle 1"`)
		assert.Contains(t, first, "style s1 stroke:gold") // dst had no input yet
		assert.Contains(t, first, "style s4 stroke:green")
		assert.Contains(t, second, "style s1 stroke:red")
		assert.Contains(t, second, "boom")
	})

	t.Run("WithResultColor changes one code and keeps the rest", func(t *testing.T) {
		plugin := New(WithCycles(), WithResultColor(component.ActivationCodeOK, "teal"))
		fm := pairMesh(t, plugin, true)
		_, err := fm.Run(t.Context())
		require.NoError(t, err)

		charts, err := plugin.ExportCycles()
		require.NoError(t, err)
		first := string(charts[0])
		assert.Contains(t, first, "style s4 stroke:teal")
		assert.Contains(t, first, "style s1 stroke:gold", "codes the option does not name keep their defaults")
	})

	t.Run("recording is opt-in", func(t *testing.T) {
		plugin := New()
		pairMesh(t, plugin, false)
		_, err := plugin.ExportCycles()
		require.ErrorIs(t, err, ErrCyclesNotRecorded)
	})
}

func TestExport(t *testing.T) {
	// The package function renders exactly what the plugin renders, options included.
	plugin := New(WithDirection("TB"))
	fm := pairMesh(t, plugin, false)

	want, err := plugin.Export()
	require.NoError(t, err)
	got, err := Export(fm, WithDirection("TB"))
	require.NoError(t, err)
	assert.Equal(t, string(want), string(got))
}

func TestExport_RejectsUnknownDirection(t *testing.T) {
	// The package function must check options as Init does, or it emits an
	// invalid flowchart.
	fm := mustNewFMesh(t, "m")
	_, err := Export(fm, WithDirection("sideways"))
	require.ErrorContains(t, err, `unknown direction "sideways"`)
}

func TestPlugin_Init(t *testing.T) {
	t.Run("one mesh per instance", func(t *testing.T) {
		plugin := New()
		mustNewFMesh(t, "first", fmesh.WithPlugins(plugin))
		_, err := fmesh.New("second", fmesh.WithPlugins(plugin))
		require.ErrorContains(t, err, "already attached")
	})

	t.Run("rejects an unknown direction", func(t *testing.T) {
		_, err := fmesh.New("m", fmesh.WithPlugins(New(WithDirection("sideways"))))
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
