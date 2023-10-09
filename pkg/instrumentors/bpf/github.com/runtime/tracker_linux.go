package runtime

import (
	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"go.opentelemetry.io/auto/pkg/inject"
	"go.opentelemetry.io/auto/pkg/instrumentors/bpffs"
	"go.opentelemetry.io/auto/pkg/instrumentors/context"
	"go.opentelemetry.io/auto/pkg/instrumentors/events"
	"go.opentelemetry.io/auto/pkg/instrumentors/utils"
	"go.opentelemetry.io/auto/pkg/log"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64,arm64 -cc clang -cflags $CFLAGS bpf ./bpf/tracker.bpf.c

const (
	instrumentedPkg  = "runtime"
	instrumentorName = "runtime-instrumentor"
)

type Instrumentor struct {
	bpfObjects *bpfObjects
	uprobes    []link.Link
}

// New returns a new [Instrumentor].
func New() *Instrumentor {
	return &Instrumentor{}
}

func (i *Instrumentor) LibraryName() string {
	return instrumentedPkg
}

func (i *Instrumentor) FuncNames() []string {
	return []string{"runtime.casgstatus", "runtime.newproc1"}
}

func (i *Instrumentor) Load(ctx *context.InstrumentorContext) error {
	spec, err := ctx.Injector.Inject(loadBpf, "go", ctx.TargetDetails.GoVersion.Original(), []*inject.StructField{
		{
			VarName:    "goid_pos",
			StructName: "runtime.g",
			Field:      "goid",
		},
	}, nil, false)

	if err != nil {
		return err
	}

	i.bpfObjects = &bpfObjects{}
	err = utils.LoadEBPFObjects(spec, i.bpfObjects, &ebpf.CollectionOptions{
		Maps: ebpf.MapOptions{
			PinPath: bpffs.PathForTargetApplication(ctx.TargetDetails),
		},
	})
	if err != nil {
		return err
	}

	for _, funcName := range i.FuncNames() {
		i.registerProbes(ctx, funcName)
	}

	return nil
}

func (i *Instrumentor) registerProbes(ctx *context.InstrumentorContext, funcName string) {
	logger := log.Logger.WithName(instrumentorName).
		WithValues("function", funcName)
	offset, err := ctx.TargetDetails.GetFunctionOffset(funcName)
	if err != nil {
		logger.Error(err, "could not find function start offset. Skipping")
		return
	}

	var up link.Link

	switch funcName {
	case "runtime.casgstatus":
		up, err = ctx.Executable.Uprobe("", i.bpfObjects.UprobeRuntimeCasgstatusByRegisters, &link.UprobeOptions{
			Address: offset,
		})
	case "runtime.newproc1":
		up, err = ctx.Executable.Uprobe("", i.bpfObjects.UprobeRuntimeNewproc1, &link.UprobeOptions{
			Address: offset,
		})
	}

	if err != nil {
		logger.Error(err, "could not insert start uprobe. Skipping")
		return
	}

	i.uprobes = append(i.uprobes, up)
}

func (i *Instrumentor) Run(eventsChan chan<- *events.Event) {}

func (i *Instrumentor) Close() {
	log.Logger.V(0).Info("closing runtime tracking")
	for _, r := range i.uprobes {
		r.Close()
	}

	if i.bpfObjects != nil {
		i.bpfObjects.Close()
	}
}
