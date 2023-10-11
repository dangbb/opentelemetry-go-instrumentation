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

// Package server provides an instrumentor for the server in the net/http
// package.
//
// Deprecated: This package is no longer supported.
package server

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"go.opentelemetry.io/auto/pkg/instrumentors/gmap"
	"golang.org/x/xerrors"
	"os"
	"sync"

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

const instrumentedPkg = "net/http"

type GMapEvent struct {
	Key   uint64
	Value uint64
	Sc    context.EBPFSpanContext
	Type  uint64
}

// Event represents an event in an HTTP server during an HTTP
// request-response.
type Event struct {
	context.BaseSpanProperties
	Method    [7]byte
	Path      [100]byte
	_         [5]byte
	Goid      uint64
	CurThread uint64
}

// Instrumentor is the net/http instrumentor.
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

// LibraryName returns the net/http package name.
func (h *Instrumentor) LibraryName() string {
	return instrumentedPkg
}

// FuncNames returns the function names from "net/http" that are instrumented.
func (h *Instrumentor) FuncNames() []string {
	return []string{"net/http.HandlerFunc.ServeHTTP"}
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
		{
			VarName:    "ctx_ptr_pos",
			StructName: "net/http.Request",
			Field:      "ctx",
		},
		{
			VarName:    "headers_ptr_pos",
			StructName: "net/http.Request",
			Field:      "Header",
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
	logger := log.Logger.WithName("net/http-instrumentor").WithValues("function", funcName)
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

	up, err := ctx.Executable.Uprobe("", h.bpfObjects.UprobeServerMuxServeHTTP, &link.UprobeOptions{
		Address: offset,
	})
	if err != nil {
		logger.V(1).Info("could not insert start uprobe. Skipping",
			"error", err.Error())
		return
	}

	h.uprobes = append(h.uprobes, up)

	for _, ret := range retOffsets {
		retProbe, err := ctx.Executable.Uprobe("", h.bpfObjects.UprobeServerMuxServeHTTP_Returns, &link.UprobeOptions{
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
	logger := log.Logger.WithName("net/http-instrumentor")
	wg := sync.WaitGroup{}
	wg.Add(2)

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

			fmt.Printf("Server write trace sc.tid: %s - sc.sid: %s - thread: %d - expected goid: %d\n",
				event.SpanContext.TraceID.String(),
				event.SpanContext.SpanID.String(),
				event.CurThread,
				event.Goid)

			goid := event.Goid

			sc, ok := gmap.GetGoId2Sc(goid)
			if ok {
				event.SpanContext.TraceID = sc.TraceID
				fmt.Printf("Server - sc for goid %d exist\n", goid)
			} else {
				psc, ok := gmap.GetAncestorSc(goid)
				fmt.Printf("Server - get from ancestor for %d\n", goid)
				if ok {
					event.ParentSpanContext = psc
					event.SpanContext.TraceID = psc.TraceID
					fmt.Printf("Server - ancestor exist. take value of ancestor. TraceID: %s - SpanID: %s\n",
						psc.TraceID.String(),
						psc.SpanID.String())
				}
			}

			eventsChan <- h.convertEvent(&event)
		}
	}()

	go func() {
		defer wg.Done()
		var event GMapEvent
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

			fmt.Printf("Server get sample type: %d - key: %d - value: %d - sc.tid: %s - sc.sid: %s\n",
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
				fmt.Printf("Server sc for goid %d exist\n", goid)
				event.Sc.TraceID = sc.TraceID
				continue
			} else {
				psc, ok := gmap.GetAncestorSc(goid)
				if ok {
					event.Sc.TraceID = psc.TraceID
					fmt.Printf("Server found ancestor for %d\n", goid)
				} else {
					gmap.SetGoId2Sc(goid, event.Sc)
					fmt.Printf("Type 4 server set sc for %d\n", goid)
				}
			}
			logger.Info(fmt.Sprintf("[DEBUG] - Server create map: %d - TraceID: %s - SpanID: %s\n",
				goid,
				event.Sc.TraceID.String(),
				event.Sc.SpanID.String()))
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

	var pscPtr *trace.SpanContext
	if e.ParentSpanContext.TraceID.IsValid() {
		psc := trace.NewSpanContext(trace.SpanContextConfig{
			TraceID:    e.ParentSpanContext.TraceID,
			SpanID:     e.ParentSpanContext.SpanID,
			TraceFlags: trace.FlagsSampled,
			Remote:     true,
		})
		pscPtr = &psc
	} else {
		pscPtr = nil
	}

	return &events.Event{
		Library: h.LibraryName(),
		// Do not include the high-cardinality path here (there is no
		// templatized path manifest to reference).
		Name:              method,
		Kind:              trace.SpanKindServer,
		StartTime:         int64(e.StartTime),
		EndTime:           int64(e.EndTime),
		SpanContext:       &sc,
		ParentSpanContext: pscPtr,
		Attributes: []attribute.KeyValue{
			semconv.HTTPMethodKey.String(method),
			semconv.HTTPTargetKey.String(path),
			attribute.Key("go-id").Int64(int64(e.Goid)),
		},
	}
}

// Close stops the Instrumentor.
func (h *Instrumentor) Close() {
	log.Logger.V(0).Info("closing net/http instrumentor")
	if h.eventsReader != nil {
		h.eventsReader.Close()
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
