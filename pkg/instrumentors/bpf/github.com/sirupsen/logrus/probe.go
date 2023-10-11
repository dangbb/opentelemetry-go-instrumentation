// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package logrus provides an instrumentation probe for the github.com/sirupsen/logrus
// package.

// //Ngo Hai Dang (Dangbb)'s thesis contribution:
// //- Implement eBPF instrumentation for logrus library.
package logrus

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/perf"
	logrus_lib "github.com/sirupsen/logrus"
	"go.opentelemetry.io/auto/pkg/instrumentors/gmap"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sys/unix"
	"golang.org/x/xerrors"
	"os"
	"sync"

	"go.opentelemetry.io/auto/pkg/inject"
	"go.opentelemetry.io/auto/pkg/instrumentors/bpffs"
	"go.opentelemetry.io/auto/pkg/instrumentors/context"
	"go.opentelemetry.io/auto/pkg/instrumentors/events"
	"go.opentelemetry.io/auto/pkg/instrumentors/utils"
	"go.opentelemetry.io/auto/pkg/log"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64,arm64 -cc clang -cflags $CFLAGS bpf ./bpf/probe.bpf.c

const (
	instrumentedPkg  = "sirupsen/logrus"
	instrumentorName = "sirupsen/logrus-instrumentor"
)

type GMapEvent struct {
	Key   uint64
	Value uint64
	Sc    context.EBPFSpanContext
	Type  uint64
}

type Event struct {
	context.BaseSpanProperties
	Level       uint64
	Log         [100]byte
	_           [4]byte
	Goid        uint64
	IsGoroutine uint64
	CurThread   uint64
}

type Instrumentor struct {
	bpfObjects      *bpfObjects
	uprobes         []link.Link
	returnProbes    []link.Link
	eventsReader    *perf.Reader
	gmapEventReader *perf.Reader
}

// New returns a new [Instrumentor].
func New() *Instrumentor {
	return &Instrumentor{}
}

func (i *Instrumentor) LibraryName() string {
	return instrumentedPkg
}

func (i *Instrumentor) FuncNames() []string {
	return []string{"github.com/sirupsen/logrus.(*Entry).write"}
}

func (i *Instrumentor) Load(ctx *context.InstrumentorContext) error {
	spec, err := ctx.Injector.Inject(loadBpf, "go", ctx.TargetDetails.GoVersion.Original(), []*inject.StructField{
		{
			VarName:    "level_ptr_pos",
			StructName: "logrus.Entry",
			Field:      "Level",
		},
		{
			VarName:    "message_ptr_pos",
			StructName: "logrus.Entry",
			Field:      "Message",
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
	rd, err := perf.NewReader(i.bpfObjects.Events, os.Getpagesize())
	if err != nil {
		return err
	}
	i.eventsReader = rd

	gmrd, err := perf.NewReader(i.bpfObjects.GmapEvents, os.Getpagesize())
	if err != nil {
		return err
	}
	i.gmapEventReader = gmrd

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

	retOffsets, err := ctx.TargetDetails.GetFunctionReturns(funcName)
	if err != nil {
		logger.Error(err, "could not find function end offset. Skipping")
		return
	}

	up, err := ctx.Executable.Uprobe("", i.bpfObjects.UprobeLogrusEntryWrite, &link.UprobeOptions{
		Address: offset,
	})
	if err != nil {
		logger.Error(err, "could not insert start uprobe. Skipping")
		return
	}

	i.uprobes = append(i.uprobes, up)

	for _, ret := range retOffsets {
		retProbe, err := ctx.Executable.Uprobe("", i.bpfObjects.UprobeLogrusEntryWriteReturns, &link.UprobeOptions{
			Address: ret,
		})
		if err != nil {
			logger.Error(err, "could not insert return uprobe. Skipping")
			return
		}
		i.returnProbes = append(i.returnProbes, retProbe)
	}
}

func (i *Instrumentor) Run(eventsChan chan<- *events.Event) {
	logger := log.Logger.WithName(instrumentorName)
	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		var event Event

		for {
			record, err := i.eventsReader.Read()
			if err != nil {
				if errors.Is(err, perf.ErrClosed) {
					logger.Info("[DEBUG] - Perf channel closed.")
					return
				}
				logger.Error(err, "error reading from perf reader")
				// Add metric to count error from perf reader
				continue
			}

			if record.LostSamples != 0 {
				logger.V(0).Info("perf event rung buffer full", "dropped", record.LostSamples)
				continue
			}

			if err := binary.Read(bytes.NewBuffer(record.RawSample), binary.LittleEndian, &event); err != nil {
				logger.Error(err, "error parsing perf event")
				continue
			}

			fmt.Printf("Logrus write trace sc.tid: %s - sc.sid: %s - thread: %d - expected goid: %d\n\n",
				event.SpanContext.TraceID.String(),
				event.SpanContext.SpanID.String(),
				event.CurThread,
				event.Goid)

			goid := event.Goid

			sc, ok := gmap.GetGoId2Sc(goid)
			if ok { // same goroutine sc exist
				event.SpanContext.TraceID = sc.TraceID
				fmt.Printf("Logrus - sc for goid %d exist\n", goid)
			} else {
				psc, ok := gmap.GetAncestorSc(goid)
				fmt.Printf("Logrus - get from ancestor for %d\n", goid)
				if ok { // parent goroutine sc exist
					event.ParentSpanContext = psc
					event.SpanContext.TraceID = psc.TraceID
					fmt.Printf("Logrus - ancestor exist. take value of ancestor. TraceID: %s - SpanID: %s\n",
						psc.TraceID.String(),
						psc.SpanID.String())
				}
			}

			eventsChan <- i.convertEvent(&event)
		}
	}()

	go func() {
		defer wg.Done()
		var event GMapEvent
		for {
			record, err := i.gmapEventReader.Read()
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

			fmt.Printf("Logrus get sample type: %d - key: %d - value: %d - sc.tid: %s - sc.sid: %s\n",
				event.Type,
				event.Key,
				event.Value,
				event.Sc.TraceID.String(),
				event.Sc.SpanID.String())

			if event.Type != gmap.GoId2Sc {
				logger.Error(xerrors.Errorf("Invalid"), "Event error, type not GOID_SC")
				continue
			}

			goid := event.Key

			// if goroutine id already taken, then skip
			sc, ok := gmap.GetGoId2Sc(goid)
			if ok {
				event.Sc.TraceID = sc.TraceID
				fmt.Printf("logrus sc for goid %d exist\n", goid)
				continue
			} else {
				psc, ok := gmap.GetAncestorSc(goid)
				if ok {
					fmt.Printf("logrus found ancestor for %d\n", goid)
					event.Sc.TraceID = psc.TraceID
				} else {
					gmap.SetGoId2Sc(goid, event.Sc)
					fmt.Printf("Type 4 logrus set sc for %d\n", goid)
				}
			}
			logger.Info(fmt.Sprintf("[DEBUG] - Logrus create map: %d - TraceID: %s - SpanID: %s\n",
				goid,
				event.Sc.TraceID.String(),
				event.Sc.SpanID.String()))
		}
	}()

	wg.Wait()
}

func convertLevel(level uint64) string {
	logrusLevel := logrus_lib.Level(level)
	return logrusLevel.String()
}

func (i *Instrumentor) convertEvent(e *Event) *events.Event {
	Log := unix.ByteSliceToString(e.Log[:])
	Level := convertLevel(e.Level)

	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    e.SpanContext.TraceID,
		SpanID:     e.SpanContext.SpanID,
		TraceFlags: trace.FlagsSampled,
	})

	var psc *trace.SpanContext = nil

	if e.ParentSpanContext.TraceID.IsValid() {
		// cross goroutine is considered to be remote
		tmp := trace.NewSpanContext(trace.SpanContextConfig{
			TraceID:    e.ParentSpanContext.TraceID,
			SpanID:     e.ParentSpanContext.SpanID,
			TraceFlags: trace.FlagsSampled,
			Remote:     true,
		})
		psc = &tmp
	}

	msgKey := attribute.Key("message")
	levelKey := attribute.Key("level")

	return &events.Event{
		Library:           i.LibraryName(),
		Name:              fmt.Sprintf("Logrus level: %s", Level),
		Kind:              trace.SpanKindServer,
		StartTime:         int64(e.StartTime),
		EndTime:           int64(e.EndTime),
		SpanContext:       &sc,
		ParentSpanContext: psc,
		Attributes: []attribute.KeyValue{
			msgKey.String(Log),
			levelKey.String(Level),
			attribute.Key("go-id").Int64(int64(e.Goid)),
		},
	}
}

func (i *Instrumentor) Close() {
	log.Logger.V(0).Info("closing sirupsen/logrus instrumentor")
	if i.eventsReader != nil {
		i.eventsReader.Close()
	}

	for _, r := range i.uprobes {
		r.Close()
	}

	for _, r := range i.returnProbes {
		r.Close()
	}

	if i.bpfObjects != nil {
		i.bpfObjects.Close()
	}
}
