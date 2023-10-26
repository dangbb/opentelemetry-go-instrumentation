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

// Package mux provides an instrumentation probe for the github.com/gorilla/mux
// package.
//
// Deprecated: This package will be removed in the next release.
package mux

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"go.opentelemetry.io/auto/pkg/instrumentors/gmap"
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

const instrumentedPkg = "github.com/gorilla/mux"

// Event represents an event in the gorilla/mux server during an HTTP
// request-response.
type Event struct {
	context.BaseSpanProperties
	Method    [7]byte
	Path      [100]byte
	_         [5]byte
	Goid      uint64
	CurThread uint64
}

// Instrumentor is the gorilla/mux instrumentor.
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

// LibraryName returns the gorilla/mux package import path.
func (g *Instrumentor) LibraryName() string {
	return instrumentedPkg
}

// FuncNames returns the function names from "github.com/gorilla/mux" that are
// instrumented.
func (g *Instrumentor) FuncNames() []string {
	return []string{"github.com/gorilla/mux.(*Router).ServeHTTP"}
}

// Load loads all instrumentation offsets.
func (g *Instrumentor) Load(ctx *context.InstrumentorContext) error {
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

	g.bpfObjects = &bpfObjects{}
	err = utils.LoadEBPFObjects(spec, g.bpfObjects, &ebpf.CollectionOptions{
		Maps: ebpf.MapOptions{
			PinPath: bpffs.PathForTargetApplication(ctx.TargetDetails),
		},
	})
	if err != nil {
		return err
	}

	for _, funcName := range g.FuncNames() {
		g.registerProbes(ctx, funcName)
	}
	rd, err := perf.NewReader(g.bpfObjects.Events, os.Getpagesize())
	if err != nil {
		return err
	}
	g.eventsReader = rd

	gmrd, err := perf.NewReader(g.bpfObjects.GmapEvents, os.Getpagesize())
	if err != nil {
		return err
	}
	g.gmapEventReader = gmrd

	return nil
}

func (g *Instrumentor) registerProbes(ctx *context.InstrumentorContext, funcName string) {
	logger := log.Logger.WithName("gorilla/mux-instrumentor").WithValues("function", funcName)
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

	up, err := ctx.Executable.Uprobe("", g.bpfObjects.UprobeGorillaMuxServeHTTP, &link.UprobeOptions{
		Address: offset,
	})
	if err != nil {
		logger.Error(err, "could not insert start uprobe. Skipping")
		return
	}

	g.uprobes = append(g.uprobes, up)

	for _, ret := range retOffsets {
		retProbe, err := ctx.Executable.Uprobe("", g.bpfObjects.UprobeGorillaMuxServeHTTP_Returns, &link.UprobeOptions{
			Address: ret,
		})
		if err != nil {
			logger.Error(err, "could not insert return uprobe. Skipping")
			return
		}
		g.returnProbs = append(g.returnProbs, retProbe)
	}
}

// Run runs the events processing loop.
func (g *Instrumentor) Run(eventsChan chan<- *events.Event) {
	logger := log.Logger.WithName("gorilla/mux-instrumentor")
	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		var event Event
		for {
			record, err := g.eventsReader.Read()
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

			gmap.MustEnrichSpan(&event, event.Goid, g.LibraryName())

			fmt.Printf("%s - write trace psc.tid: %s - psc.sid: %s\nsc.tid: %s - sc.sid: %s - thread: %d - expected goid: %d\n",
				g.LibraryName(),
				event.ParentSpanContext.TraceID.String(),
				event.ParentSpanContext.SpanID.String(),
				event.SpanContext.TraceID.String(),
				event.SpanContext.SpanID.String(),
				event.CurThread,
				event.Goid)

			eventsChan <- g.convertEvent(&event)
		}
	}()

	go func() {
		defer wg.Done()
		var event gmap.GMapEvent
		for {
			record, err := g.gmapEventReader.Read()
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

			// gorilla/mux is deprecated
			enrichEvent := gmap.ConvertEnrichEvent(event)
			gmap.RegisterSpan(&enrichEvent, g.LibraryName(), false)

			if enrichEvent.Psc.TraceID.IsValid() {
				// middleware created
				eventsChan <- gmap.ConvertEvent(enrichEvent)
			}
		}
	}()

	wg.Wait()
}

func (g *Instrumentor) convertEvent(e *Event) *events.Event {
	method := unix.ByteSliceToString(e.Method[:])
	path := unix.ByteSliceToString(e.Path[:])

	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    e.SpanContext.TraceID,
		SpanID:     e.SpanContext.SpanID,
		TraceFlags: trace.FlagsSampled,
	})

	return &events.Event{
		Library: g.LibraryName(),
		// Do not include the high-cardinality path here (there is no
		// templatized path manifest to reference).
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
func (g *Instrumentor) Close() {
	log.Logger.V(0).Info("closing gorilla/mux instrumentor")
	if g.eventsReader != nil {
		g.eventsReader.Close()
	}

	for _, r := range g.uprobes {
		r.Close()
	}

	for _, r := range g.returnProbs {
		r.Close()
	}

	if g.bpfObjects != nil {
		g.bpfObjects.Close()
	}
}
