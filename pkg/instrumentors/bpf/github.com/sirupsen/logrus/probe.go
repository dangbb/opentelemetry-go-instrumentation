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
	"go.opentelemetry.io/auto/pkg/instrumentors/gmap"
	"golang.org/x/xerrors"
	"os"
	"sync"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/perf"
	logrus_lib "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sys/unix"

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
	Level     uint64
	Log       [100]byte
	_         [4]byte
	Goid      uint64
	CurThread uint64
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

			goid, ok := gmap.GetCurThread2GoId(event.CurThread)
			if !ok {
				logger.Info("Not found goroutine id for thread: %d", event.CurThread)
				continue
			}

			sc, ok := gmap.GetGoId2Sc(goid)
			if ok {
				event.SpanContext.TraceID = sc.TraceID
			}

			psc, ok := gmap.GetAncestorSc(goid)
			if !ok {
				gmap.SetGoId2Sc(goid, event.SpanContext)
			} else {
				event.SpanContext.TraceID = psc.TraceID
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

			if event.Type != 4 {
				logger.Error(xerrors.Errorf("Invalid"), "Event error, type not CURTHREAD_SC")
				continue
			}

			goid, ok := gmap.GetCurThread2GoId(event.Key)
			if !ok {
				logger.Info(fmt.Sprintf("Goroutine id for thread %d not found", event.Key))
				continue
			}

			// if goroutine id already taken, then skip
			sc, ok := gmap.GetGoId2Sc(goid)
			if ok {
				event.Sc.TraceID = sc.TraceID
				continue
			}

			psc, ok := gmap.GetAncestorSc(goid)
			if ok {
				event.Sc.TraceID = psc.TraceID
			}

			gmap.SetGoId2Sc(goid, event.Sc)
			logger.Info("[DEBUG] - Create map: %d - TraceID: %s - SpanID: %s\n",
				goid,
				event.Sc.TraceID.String(),
				event.Sc.SpanID.String())
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

	log.Logger.V(0).Info(fmt.Sprintf("logrus: Value of default parent span, trace ID %s - span ID %s",
		e.ParentSpanContext.TraceID,
		e.ParentSpanContext.SpanID))

	log.Logger.V(0).Info(fmt.Sprintf("logrus: Value of default span, trace ID %s - span ID %s",
		e.SpanContext.TraceID,
		e.SpanContext.SpanID))

	msgKey := attribute.Key("message")
	levelKey := attribute.Key("level")

	return &events.Event{
		Library:     i.LibraryName(),
		Name:        fmt.Sprintf("Logrus level: %s", Level),
		Kind:        trace.SpanKindServer,
		StartTime:   int64(e.StartTime),
		EndTime:     int64(e.EndTime),
		SpanContext: &sc,
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
