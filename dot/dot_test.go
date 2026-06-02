package dot

import (
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

func Test_dotExporter_Export(t *testing.T) {
	type args struct {
		fm *fmesh.FMesh
	}
	tests := []struct {
		name       string
		args       args
		assertions func(t *testing.T, data []byte, err error)
	}{
		{
			name: "empty f-mesh",
			args: args{
				fm: mustNewFMesh(t, "fm"),
			},
			assertions: func(t *testing.T, data []byte, err error) {
				require.NoError(t, err)
				assert.Empty(t, data)
			},
		},
		{
			name: "happy path",
			args: args{
				fm: func() *fmesh.FMesh {
					adder := mustNewComponent(t, "adder",
						component.WithDescription("This component adds 2 numbers"),
						component.WithInputs("num1", "num2"),
						component.WithOutputs("result"),
						component.WithActivationFunc(func(this *component.Component) error {
							return nil
						}),
					)

					multiplier := mustNewComponent(t, "multiplier",
						component.WithDescription("This component multiplies number by 3"),
						component.WithInputs("num"),
						component.WithOutputs("result"),
						component.WithActivationFunc(func(this *component.Component) error {
							return nil
						}),
					)

					require.NoError(t, adder.OutputByName("result").PipeTo(multiplier.InputByName("num")))

					fm := mustNewFMesh(t, "fm", fmesh.WithDescription("This f-mesh has just one component"))
					require.NoError(t, fm.AddComponents(adder, multiplier))
					return fm
				}(),
			},
			assertions: func(t *testing.T, data []byte, err error) {
				require.NoError(t, err)
				assert.NotEmpty(t, data)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exporter := NewDotExporter()

			got, err := exporter.Export(tt.args.fm)
			if tt.assertions != nil {
				tt.assertions(t, got, err)
			}
		})
	}
}

func Test_dotExporter_ExportWithCycles(t *testing.T) {
	type args struct {
		fm *fmesh.FMesh
	}
	tests := []struct {
		name       string
		args       args
		assertions func(t *testing.T, data [][]byte, err error)
	}{
		{
			name: "happy path",
			args: args{
				fm: func() *fmesh.FMesh {
					adder := mustNewComponent(t, "adder",
						component.WithDescription("This component adds 2 numbers"),
						component.WithInputs("num1", "num2"),
						component.WithOutputs("result"),
						component.WithActivationFunc(func(this *component.Component) error {
							num1, err := this.InputByName("num1").Signals().FirstPayload()
							if err != nil {
								return err
							}

							num2, err := this.InputByName("num2").Signals().FirstPayload()
							if err != nil {
								return err
							}

							return this.OutputByName("result").PutSignals(signal.New(num1.(int) + num2.(int)))
						}),
					)

					multiplier := mustNewComponent(t, "multiplier",
						component.WithDescription("This component multiplies number by 3"),
						component.WithInputs("num"),
						component.WithOutputs("result"),
						component.WithActivationFunc(func(this *component.Component) error {
							num, err := this.InputByName("num").Signals().FirstPayload()
							if err != nil {
								return err
							}
							return this.OutputByName("result").PutSignals(signal.New(num.(int) * 3))
						}),
					)

					require.NoError(t, adder.OutputByName("result").PipeTo(multiplier.InputByName("num")))

					fm := mustNewFMesh(t, "fm", fmesh.WithDescription("This f-mesh has just one component"))
					require.NoError(t, fm.AddComponents(adder, multiplier))

					require.NoError(t, adder.InputByName("num1").PutSignals(signal.New(15)))
					require.NoError(t, adder.InputByName("num2").PutSignals(signal.New(12)))

					return fm
				}(),
			},
			assertions: func(t *testing.T, data [][]byte, err error) {
				require.NoError(t, err)
				assert.NotEmpty(t, data)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runResult, err := tt.args.fm.Run()
			require.NoError(t, err)

			exporter := NewDotExporter()

			got, err := exporter.ExportWithCycles(tt.args.fm, runResult.Cycles)
			if tt.assertions != nil {
				tt.assertions(t, got, err)
			}
		})
	}
}
