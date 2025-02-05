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

// Package gin provides an instrumentor for the github.com/gin-gonic/gin
// package.
//
// Deprecated: This package is no longer supported.
package gin

import (
	"bytes"
	"encoding/binary"
	"errors"
	"go.opentelemetry.io/auto/pkg/instrumentors/gmap"
	"golang.org/x/xerrors"
	"sync"

	"os"

	"go.opentelemetry.io/auto/pkg/instrumentors/bpffs" // nolint:staticcheck  // Atomic deprecation.

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/perf"
	"golang.org/x/sys/unix"

	"go.opentelemetry.io/auto/pkg/inject"                // nolint:staticcheck  // Atomic deprecation.
	"go.opentelemetry.io/auto/pkg/instrumentors/context" // nolint:staticcheck  // Atomic deprecation.
	"go.opentelemetry.io/auto/pkg/instrumentors/events"  // nolint:staticcheck  // Atomic deprecation.
	"go.opentelemetry.io/auto/pkg/instrumentors/utils"   // nolint:staticcheck  // Atomic deprecation.
	"go.opentelemetry.io/auto/pkg/log"                   // nolint:staticcheck  // Atomic deprecation.
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.18.0"
	"go.opentelemetry.io/otel/trace"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64,arm64 -cc clang -cflags $CFLAGS bpf ./bpf/probe.bpf.c

const instrumentedPkg = "github.com/gin-gonic/gin"

// Event represents an event in the gin-gonic/gin server during an HTTP
// request-response.
type Event struct {
	context.BaseSpanProperties
	Method    [7]byte
	Path      [100]byte
	_         [5]byte
	Goid      uint64
	CurThread uint64
}

// Instrumentor is the gin-gonic/gin instrumentor.
type Instrumentor struct {
	bpfObjects      *bpfObjects
	uprobes         []link.Link
	returnProbs     []link.Link
	eventsReader    *perf.Reader
	gmapEventReader *perf.Reader
}

// New returns a new [Instrumentor].
func New() *Instrumentor {
	return &Instrumentor{}
}

// LibraryName returns the gin-gonic/gin package import path.
func (h *Instrumentor) LibraryName() string {
	return instrumentedPkg
}

// FuncNames returns the function names from "github.com/gin-gonic/gin" that are
// instrumented.
func (h *Instrumentor) FuncNames() []string {
	return []string{"github.com/gin-gonic/gin.(*Engine).ServeHTTP"}
}

// Load loads all instrumentation offsets.
func (h *Instrumentor) Load(ctx *context.InstrumentorContext) error {
	spec, err := ctx.Injector.Inject(loadBpf, "go", ctx.TargetDetails.GoVersion.Original(), []*inject.StructField{
		{
			VarName:    "method_ptr_pos",
			StructName: "net/http.Request",
			Field:      "Method",
		},
		{
			VarName:    "url_ptr_pos",
			StructName: "net/http.Request",
			Field:      "URL",
		},
		{
			VarName:    "ctx_ptr_pos",
			StructName: "net/http.Request",
			Field:      "ctx",
		},
		{
			VarName:    "path_ptr_pos",
			StructName: "net/url.URL",
			Field:      "Path",
		},
	}, nil, false)

	if err != nil {
		return err
	}

	h.bpfObjects = &bpfObjects{}
	err = utils.LoadEBPFObjects(spec, h.bpfObjects, &ebpf.CollectionOptions{
		Maps: ebpf.MapOptions{
			PinPath: bpffs.PathForTargetApplication(ctx.TargetDetails),
		},
	})
	if err != nil {
		return err
	}

	for _, funcName := range h.FuncNames() {
		h.registerProbes(ctx, funcName)
	}

	rd, err := perf.NewReader(h.bpfObjects.Events, os.Getpagesize())
	if err != nil {
		return err
	}
	h.eventsReader = rd

	gmrd, err := perf.NewReader(h.bpfObjects.GmapEvents, os.Getpagesize())
	if err != nil {
		return err
	}
	h.gmapEventReader = gmrd

	return nil
}

func (h *Instrumentor) registerProbes(ctx *context.InstrumentorContext, funcName string) {
	logger := log.Logger.WithName("gin-gonic/gin-instrumentor").WithValues("function", funcName)
	offset, err := ctx.TargetDetails.GetFunctionOffset(funcName)
	if err != nil {
		logger.Error(err, "could not find function start offset. Skipping")
		return
	}
	retOffsets, err := ctx.TargetDetails.GetFunctionReturns(funcName)
	if err != nil {
		logger.Error(err, "could not find function end offsets. Skipping")
		return
	}

	up, err := ctx.Executable.Uprobe("", h.bpfObjects.UprobeGinEngineServeHTTP, &link.UprobeOptions{
		Address: offset,
	})
	if err != nil {
		logger.V(1).Info("could not insert start uprobe. Skipping",
			"error", err.Error())
		return
	}

	h.uprobes = append(h.uprobes, up)

	for _, ret := range retOffsets {
		retProbe, err := ctx.Executable.Uprobe("", h.bpfObjects.UprobeGinEngineServeHTTP_Returns, &link.UprobeOptions{
			Address: ret,
		})
		if err != nil {
			logger.Error(err, "could not insert return uprobe. Skipping")
			return
		}
		h.returnProbs = append(h.returnProbs, retProbe)
	}
}

// Run runs the events processing loop.
func (h *Instrumentor) Run(eventsChan chan<- *events.Event) {
	logger := log.Logger.WithName("gin-gonic/gin-instrumentor")
	wg := sync.WaitGroup{}
	wg.Add(2)

	ginMainEventType := utils.ItemType("gin_main_event")
	ginPlaceholderEventType := utils.ItemType("gin_placeholder_event")

	utils.EventProrityQueueSingleton.Register(ginMainEventType, func(rawEvent interface{}) {
		event := rawEvent.(Event)

		gmap.MustEnrichSpan(&event, event.Goid, h.LibraryName())

		eventsChan <- h.convertEvent(&event)
	})

	utils.EventProrityQueueSingleton.Register(ginPlaceholderEventType, func(rawEvent interface{}) {
		event := rawEvent.(gmap.GMapEvent)

		if event.Type != gmap.GoId2Sc {
			logger.Error(xerrors.Errorf("Invalid"), "Event error, type not GOID_SC")
			return
		}

		// Gin gonic using one goroutine for all process. Should only keep same site on eBPF
		enrichEvent := gmap.ConvertEnrichEvent(event)
		gmap.RegisterSpan(&enrichEvent, h.LibraryName(), true)
	})

	go func() {
		defer wg.Done()
		var event Event
		for {
			record, err := h.eventsReader.Read()
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

			utils.EventProrityQueueSingleton.Push(event, event.StartTime, ginMainEventType)
		}
	}()

	go func() {
		defer wg.Done()
		var event gmap.GMapEvent
		for {
			record, err := h.gmapEventReader.Read()
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

			utils.EventProrityQueueSingleton.Push(event, event.StartTime-1, ginPlaceholderEventType)
		}
	}()

	wg.Wait()
}

func (h *Instrumentor) convertEvent(e *Event) *events.Event {
	method := unix.ByteSliceToString(e.Method[:])
	path := unix.ByteSliceToString(e.Path[:])

	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    e.SpanContext.TraceID,
		SpanID:     e.SpanContext.SpanID,
		TraceFlags: trace.FlagsSampled,
	})

	return &events.Event{
		Library: h.LibraryName(),
		// Do not include the high-cardinality path here (there is no
		// templatized path manifest to reference, given we are instrumenting
		// Engine.ServeHTTP which is not passed a Gin Context).
		Name:        method,
		Kind:        trace.SpanKindServer,
		StartTime:   int64(e.StartTime),
		EndTime:     int64(e.EndTime),
		SpanContext: &sc,
		Attributes: []attribute.KeyValue{
			semconv.HTTPMethodKey.String(method),
			semconv.HTTPTargetKey.String(path),
			attribute.Key("go-id").Int64(int64(e.Goid)),
		},
	}
}

// Close stops the Instrumentor.
func (h *Instrumentor) Close() {
	log.Logger.V(0).Info("closing gin-gonic/gin instrumentor")
	if h.eventsReader != nil {
		h.eventsReader.Close()
	}

	if h.gmapEventReader != nil {
		h.gmapEventReader.Close()
	}

	for _, r := range h.uprobes {
		r.Close()
	}

	for _, r := range h.returnProbs {
		r.Close()
	}

	if h.bpfObjects != nil {
		h.bpfObjects.Close()
	}
}
