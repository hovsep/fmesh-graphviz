// Command showcase builds and runs an order-processing mesh, then writes its
// graphs in one export format to a directory: static.<ext> and one
// cycle-NNN.<ext> per cycle. CI renders them, so every change to an exporter
// shows up as a picture.
//
// The mesh covers what an exporter has to draw: fan-out, fan-in, a component
// that fails, and one that waits for inputs arriving in different cycles.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-export/d2"
	"github.com/hovsep/fmesh-export/dot"
	"github.com/hovsep/fmesh-export/mermaid"
	"github.com/hovsep/fmesh-export/plantuml"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/export"
	"github.com/hovsep/fmesh/port"
)

// format is an export format: its file extension and its exporter.
type format struct {
	ext      string
	exporter export.Exporter
}

var formats = map[string]format{
	"dot":      {"dot", dot.New()},
	"mermaid":  {"mmd", mermaid.New()},
	"d2":       {"d2", d2.New()},
	"plantuml": {"puml", plantuml.New()},
}

func main() {
	names := slices.Sorted(maps.Keys(formats))
	formatName := flag.String("format", "dot", "export format: "+strings.Join(names, ", "))
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: showcase [-format name] <output dir>")
		os.Exit(2)
	}
	f, ok := formats[*formatName]
	if !ok {
		fmt.Fprintf(os.Stderr, "showcase: unknown format %q (want one of %s)\n", *formatName, strings.Join(names, ", "))
		os.Exit(2)
	}
	if err := run(f, flag.Arg(0)); err != nil {
		fmt.Fprintln(os.Stderr, "showcase:", err)
		os.Exit(1)
	}
}

func run(f format, dir string) error {
	fm, err := buildMesh()
	if err != nil {
		return err
	}
	if err = fm.ComponentByName("orders").InputByName("start").PutPayloads(true); err != nil {
		return err
	}
	ri, err := fm.Run(context.Background())
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}

	static, err := f.exporter.Export(fm)
	if err != nil {
		return err
	}
	cycles := ri.Cycles.All()

	// The output dir is the caller's own argument; os.Root keeps every file
	// inside it.
	if err = os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()

	if err = root.WriteFile("static."+f.ext, static, 0o600); err != nil {
		return err
	}
	for i, c := range cycles {
		var graph []byte
		if graph, err = f.exporter.ExportCycle(fm, c); err != nil {
			return fmt.Errorf("cycle %d: %w", c.Number(), err)
		}
		if err = root.WriteFile(fmt.Sprintf("cycle-%03d.%s", i+1, f.ext), graph, 0o600); err != nil {
			return err
		}
	}
	fmt.Printf("wrote static.%s and %d cycle graphs to %s\n", f.ext, len(cycles), dir)
	return nil
}

var errNegativeAmount = errors.New("negative amount")

// forward returns an activation function that sends every input signal,
// transformed by fn, out of the named outputs.
func forward(fn func(amount float64) float64, outputs ...string) component.ActivationFunc {
	return func(_ context.Context, this *component.Component) error {
		for _, in := range this.Inputs().AllOrdered() {
			for _, amount := range in.Signals().AllPayloads() {
				for _, out := range outputs {
					if err := this.OutputByName(out).PutPayloads(fn(amount.(float64))); err != nil {
						return err
					}
				}
			}
		}
		return nil
	}
}

func same(amount float64) float64 { return amount }

func buildMesh() (*fmesh.FMesh, error) {
	var components []*component.Component
	add := func(name, description string, opts ...component.Option) {
		c, err := component.New(name, append(opts, component.WithDescription(description))...)
		if err != nil {
			panic(err) // static wiring: an error here is a bug in this file
		}
		components = append(components, c)
	}

	add("orders", "emits a batch of orders",
		component.WithInputs("start"), component.WithOutputs("order"),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			return this.OutputByName("order").PutPayloads(12.0, 480.0, -5.0, 35.0, 900.0)
		}))
	add("router", "routes orders by amount",
		component.WithInputs("order"), component.WithOutputs("small", "large", "invalid"),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			for _, amount := range this.InputByName("order").Signals().AllPayloads() {
				out := "small"
				switch a := amount.(float64); {
				case a < 0:
					out = "invalid"
				case a >= 100:
					out = "large"
				}
				if err := this.OutputByName(out).PutPayloads(amount); err != nil {
					return err
				}
			}
			return nil
		}))
	add("validator", "rejects invalid orders",
		component.WithInputs("order"),
		component.WithActivationFunc(func(context.Context, *component.Component) error {
			return errNegativeAmount
		}))
	add("small-handler", "packs small orders",
		component.WithInputs("order"), component.WithOutputs("billing", "audit"),
		component.WithActivationFunc(forward(same, "billing", "audit")))
	add("large-handler", "packs large orders",
		component.WithInputs("order"), component.WithOutputs("billing", "audit"),
		component.WithActivationFunc(forward(same, "billing", "audit")))
	add("tax", "adds 20% tax",
		component.WithInputs("small", "large"), component.WithOutputs("tax"),
		component.WithActivationFunc(forward(func(a float64) float64 { return a * 0.2 }, "tax")))
	add("shipping", "quotes shipping",
		component.WithInputs("small", "large"), component.WithOutputs("quote"),
		component.WithActivationFunc(forward(func(float64) float64 { return 7.5 }, "quote")))
	add("carrier", "confirms the quote one cycle later",
		component.WithInputs("quote"), component.WithOutputs("shipping"),
		component.WithActivationFunc(forward(same, "shipping")))
	add("invoice", "waits for tax and shipping",
		component.WithInputs("tax", "shipping"), component.WithOutputs("invoice"),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			if !this.Inputs().AllHaveSignals() {
				return component.ErrWaitKeepingInputs
			}
			total := this.InputByName("tax").Signals().ReducePayloads(0.0, func(acc float64, p any) float64 { return acc + p.(float64) }) +
				this.InputByName("shipping").Signals().ReducePayloads(0.0, func(acc float64, p any) float64 { return acc + p.(float64) })
			return this.OutputByName("invoice").PutPayloads(total)
		}))
	add("audit", "logs every packed order",
		component.WithInputs("small", "large"),
		component.WithActivationFunc(func(context.Context, *component.Component) error { return nil }))
	add("printer", "prints invoices",
		component.WithInputs("invoice"),
		component.WithActivationFunc(func(context.Context, *component.Component) error { return nil }))

	fm, err := fmesh.New("order-processing",
		fmesh.WithDescription("Order processing: fan-out, fan-in, an error and a wait"),
		fmesh.WithErrorHandlingStrategy(fmesh.IgnoreAll))
	if err != nil {
		return nil, err
	}
	if err := fm.AddComponents(components...); err != nil {
		return nil, err
	}

	byName := fm.ComponentByName
	return fm, port.MultiPipe(
		port.Pipe{From: byName("orders").OutputByName("order"), To: byName("router").InputByName("order")},
		port.Pipe{From: byName("router").OutputByName("invalid"), To: byName("validator").InputByName("order")},
		port.Pipe{From: byName("router").OutputByName("small"), To: byName("small-handler").InputByName("order")},
		port.Pipe{From: byName("router").OutputByName("large"), To: byName("large-handler").InputByName("order")},
		port.Pipe{From: byName("small-handler").OutputByName("billing"), To: byName("tax").InputByName("small")},
		port.Pipe{From: byName("small-handler").OutputByName("billing"), To: byName("shipping").InputByName("small")},
		port.Pipe{From: byName("large-handler").OutputByName("billing"), To: byName("tax").InputByName("large")},
		port.Pipe{From: byName("large-handler").OutputByName("billing"), To: byName("shipping").InputByName("large")},
		port.Pipe{From: byName("small-handler").OutputByName("audit"), To: byName("audit").InputByName("small")},
		port.Pipe{From: byName("large-handler").OutputByName("audit"), To: byName("audit").InputByName("large")},
		port.Pipe{From: byName("shipping").OutputByName("quote"), To: byName("carrier").InputByName("quote")},
		port.Pipe{From: byName("tax").OutputByName("tax"), To: byName("invoice").InputByName("tax")},
		port.Pipe{From: byName("carrier").OutputByName("shipping"), To: byName("invoice").InputByName("shipping")},
		port.Pipe{From: byName("invoice").OutputByName("invoice"), To: byName("printer").InputByName("invoice")},
	)
}
