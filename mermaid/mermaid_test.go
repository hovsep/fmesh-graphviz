package mermaid

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-export"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/cycle"
	"github.com/hovsep/fmesh/signal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
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
	charts := make([]string, 0, ri.Cycles.Len())
	for _, c := range ri.Cycles.All() {
		chart, err := e.ExportCycle(fm, c)
		require.NoError(t, err)
		charts = append(charts, string(chart))
	}
	return charts
}

const pairChart = `---
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
`

func TestExporter_Export(t *testing.T) {
	t.Run("flowchart of the structure", func(t *testing.T) {
		got, err := New().Export(pairMesh(t, false))
		require.NoError(t, err)
		assert.Equal(t, pairChart, string(got))
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
			component.WithDescription("line one\n<b>line</b> two"),
			component.WithInputs("in put"),
			component.WithOutputs(`a->b`),
			component.WithActivationFunc(func(context.Context, *component.Component) error { return nil }))
		fm := mustNewFMesh(t, `mesh <x> \n`, fmesh.WithDescription("a\nb"))
		require.NoError(t, fm.AddComponents(c))

		got, err := New().Export(fm)
		require.NoError(t, err)
		assert.Equal(t, `---
title: "mesh <x> \\n"
---
flowchart LR
  %% a b
  subgraph s1["my #quot;comp#quot;: v1.2"]
    c2["line one #lt;b#gt;line#lt;/b#gt; two"]
    p3(("in put"))
    p3 --> c2
    p4(("a-#gt;b"))
    c2 --> p4
  end
`, string(got))
	})

	t.Run("title is valid YAML", func(t *testing.T) {
		for _, name := range []string{`a\q`, `C:\new`, `say "hi"`, "tab\there", "#quot; & é"} {
			got, err := New().Export(mustNewFMesh(t, name))
			require.NoError(t, err)

			frontMatter, _, ok := strings.Cut(strings.TrimPrefix(string(got), "---\n"), "---\n")
			require.True(t, ok, "front matter of %q", name)
			var parsed struct{ Title string }
			require.NoError(t, yaml.Unmarshal([]byte(frontMatter), &parsed), "front matter of %q", name)
			assert.Equal(t, strings.Join(strings.Fields(name), " "), parsed.Title)
		}
	})
}

func TestExporter_ExportCycle(t *testing.T) {
	t.Run("colors components by result", func(t *testing.T) {
		charts := runAndExportCycles(t, New(), pairMesh(t, true))
		require.GreaterOrEqual(t, len(charts), 2)

		first, second := charts[0], charts[1]
		assert.Contains(t, first, `title: "pair — cycle 1"`)
		assert.Contains(t, first, "style s1 stroke:gold") // dst had no input yet
		assert.Contains(t, first, "style s4 stroke:green")
		assert.Contains(t, second, "style s1 stroke:red")
		assert.Contains(t, second, "boom")
	})

	t.Run("WithResultColor changes one code and keeps the rest", func(t *testing.T) {
		charts := runAndExportCycles(t, New(WithResultColor(component.ActivationCodeOK, "teal")), pairMesh(t, true))
		assert.Contains(t, charts[0], "style s4 stroke:teal")
		assert.Contains(t, charts[0], "style s1 stroke:gold", "codes the option does not name keep their defaults")
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

		charts := runAndExportCycles(t, New(), fm)
		require.NotEmpty(t, charts)
		assert.Contains(t, charts[0], "    c2 -.- e3>\"component returned an error: attempt 1 of 2: boom; component returned an error: attempt 2 of 2: boom\"]\n")
	})
}

func TestExporter_Options(t *testing.T) {
	t.Run("WithDirection", func(t *testing.T) {
		got, err := New(WithDirection("TB")).Export(pairMesh(t, false))
		require.NoError(t, err)
		assert.Contains(t, string(got), "flowchart TB\n")
	})

	t.Run("rejects an unknown direction", func(t *testing.T) {
		// An unchecked direction would emit an invalid flowchart.
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
	e := New(WithDirection("TB"))
	pair, other := pairMesh(t, false), oneMesh(t)

	first, err := e.Export(pair)
	require.NoError(t, err)
	fromOther, err := e.Export(other)
	require.NoError(t, err)
	again, err := e.Export(pair)
	require.NoError(t, err)
	fresh, err := New(WithDirection("TB")).Export(other)
	require.NoError(t, err)

	assert.Equal(t, string(first), string(again))
	assert.Equal(t, string(fresh), string(fromOther))
	assert.Contains(t, string(first), "flowchart TB\n", "options apply to every export")
}

func TestExporter_ExportCycleFromHook(t *testing.T) {
	// Streaming: one chart per cycle while the mesh runs, equal to exporting
	// the recorded cycles afterwards.
	e := New()
	fm := pairMesh(t, true)
	var frames []string
	fm.SetupHooks(func(h *fmesh.Hooks) {
		h.AfterCycle(func(_ context.Context, cc *fmesh.CycleContext) error {
			chart, err := e.ExportCycle(cc.FMesh, cc.Cycle)
			frames = append(frames, string(chart))
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
