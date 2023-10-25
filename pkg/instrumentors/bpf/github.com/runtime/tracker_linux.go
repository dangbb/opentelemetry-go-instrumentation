package runtime

import (
	"bytes"
	"encoding/binary"
	"errors"
	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/perf"
	"go.opentelemetry.io/auto/pkg/inject"
	"go.opentelemetry.io/auto/pkg/instrumentors/bpffs"
	"go.opentelemetry.io/auto/pkg/instrumentors/context"
	"go.opentelemetry.io/auto/pkg/instrumentors/events"
	"go.opentelemetry.io/auto/pkg/instrumentors/gmap"
	"go.opentelemetry.io/auto/pkg/instrumentors/utils"
	"go.opentelemetry.io/auto/pkg/log"
	"os"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64,arm64 -cc clang -cflags $CFLAGS bpf ./bpf/tracker.bpf.c

const (
	instrumentedPkg  = "runtime"
	instrumentorName = "runtime-instrumentor"
)

type GmapEvent struct {
	Key   uint64
	Value uint64
	Sc    context.EBPFSpanContext
	Type  uint64
}

type Instrumentor struct {
	bpfObjects   *bpfObjects
	uprobes      []link.Link
	eventsReader *perf.Reader
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

	rd, err := perf.NewReader(i.bpfObjects.GmapEvents, os.Getpagesize())
	if err != nil {
		return err
	}
	i.eventsReader = rd

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

func (i *Instrumentor) Run(eventsChan chan<- *events.Event) {
	logger := log.Logger.WithName("net/http-instrumentor")
	var event GmapEvent
	for {
		record, err := i.eventsReader.Read()
		if err != nil {
			if errors.Is(err, perf.ErrClosed) {
				return
			}
			logger.Error(err, "error reading from perf reader")
			continue
		}

		if record.LostSamples != 0 {
			logger.V(0).Info("perf event ring buffer full", "dropped", record.LostSamples)
			continue
		}

		if err := binary.Read(bytes.NewBuffer(record.RawSample), binary.LittleEndian, &event); err != nil {
			logger.Error(err, "error parsing perf event")
			continue
		}

		switch event.Type {
		case gmap.GoPc2PGoId:
			gmap.SetGoPc2GoId(event.Key, event.Value)
		case gmap.GoId2GoPc:
			pgoid, ok := gmap.GetGoPc2GoId(event.Value)
			if !ok {
				continue
			}
			gmap.SetGoId2PGoId(event.Key, pgoid)
		}
	}
}

func (i *Instrumentor) Close() {
	log.Logger.V(0).Info("closing runtime tracking")
	for _, r := range i.uprobes {
		r.Close()
	}

	if i.bpfObjects != nil {
		i.bpfObjects.Close()
	}

	if i.eventsReader != nil {
		i.eventsReader.Close()
	}
}
