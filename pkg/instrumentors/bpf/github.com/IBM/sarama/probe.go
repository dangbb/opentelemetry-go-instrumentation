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

// //Ngo Hai Dang (Dangbb)'s thesis contribution:
// //- Implement eBPF instrumentation for sarama library.

package sarama

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/exp/rand"
	"golang.org/x/sys/unix"
	"golang.org/x/xerrors"
	"os"
	"sync"

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
	"go.opentelemetry.io/otel/trace"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64,arm64 -cc clang -cflags $CFLAGS bpf ./bpf/probe.bpf.c

const (
	instrumentedPkg  = "IBM/sarama"
	instrumentorName = "IBM/sarama-instrumentor"
)

type Event struct {
	context.BaseSpanProperties
	Topic       [30]byte
	Key         [20]byte
	Value       [50]byte
	_           [4]byte
	Goid        uint64
	Header1     [25]byte
	Value1      [25]byte
	Header2     [25]byte
	Value2      [25]byte
	_           [4]byte
	IsGoroutine uint64
	CurThread   uint64
	//Header3 [25]byte
	//Value3  [25]byte
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
	return []string{"github.com/IBM/sarama.(*syncProducer).SendMessage"}
}

func (i *Instrumentor) Load(ctx *context.InstrumentorContext) error {
	spec, err := ctx.Injector.Inject(loadBpf, "go", ctx.TargetDetails.GoVersion.Original(), []*inject.StructField{
		{
			VarName:    "topic_ptr_pos",
			StructName: "sarama.ProducerMessage",
			Field:      "Topic",
		},
		{
			VarName:    "key_ptr_pos",
			StructName: "sarama.ProducerMessage",
			Field:      "Key",
		},
		{
			VarName:    "value_ptr_pos",
			StructName: "sarama.ProducerMessage",
			Field:      "Value",
		},
		{
			VarName:    "headers_arr_ptr_pos",
			StructName: "sarama.ProducerMessage",
			Field:      "Headers",
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

	up, err := ctx.Executable.Uprobe("", i.bpfObjects.UprobeSyncProducerSendMessage, &link.UprobeOptions{
		Address: offset,
	})
	if err != nil {
		logger.Error(err, "could not insert start uprobe. Skipping")
		return
	}

	i.uprobes = append(i.uprobes, up)

	for _, ret := range retOffsets {
		retProbe, err := ctx.Executable.Uprobe("", i.bpfObjects.UprobeSyncProducerSendMessageReturns, &link.UprobeOptions{
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
		defer wg.Done()
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

			fmt.Printf("Receive event from lib %s - psc.tid: %s - psc.sid: %s\nsc.tid: %s - sc.sid: %s - thread: %d - expected goid: %d\n",
				i.LibraryName(),
				event.ParentSpanContext.TraceID.String(),
				event.ParentSpanContext.SpanID.String(),
				event.SpanContext.TraceID.String(),
				event.SpanContext.SpanID.String(),
				event.CurThread,
				event.Goid)
			gmap.EnrichSpan(&event, event.Goid, i.LibraryName())

			fmt.Printf("After enrich at lib %s - write trace psc.tid: %s - psc.sid: %s\nsc.tid: %s - sc.sid: %s - thread: %d - expected goid: %d\n",
				i.LibraryName(),
				event.ParentSpanContext.TraceID.String(),
				event.ParentSpanContext.SpanID.String(),
				event.SpanContext.TraceID.String(),
				event.SpanContext.SpanID.String(),
				event.CurThread,
				event.Goid)

			eventsChan <- i.convertEvent(&event)
		}
	}()

	go func() {
		defer wg.Done()
		var event gmap.GMapEvent
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

			fmt.Printf("Sarama get sample type: %d - key: %d - value: %d - sc.tid: %s - sc.sid: %s\n",
				event.Type,
				event.Key,
				event.Value,
				event.Sc.TraceID.String(),
				event.Sc.SpanID.String())

			if event.Type != gmap.GoId2Sc {
				logger.Error(xerrors.Errorf("Invalid"), "Event error, type not GOID_SC")
				continue
			}

			gmap.RegisterSpan(event, i.LibraryName())
		}
	}()

	wg.Wait()
}

func genRandomSpanId() trace.SpanID {
	buff := trace.SpanID{}
	for i := 0; i < 2; i++ {
		random := rand.Int31()
		buff[(4 * i)] = byte((random >> 24) & 0xFF)
		buff[(4*i)+1] = byte((random >> 16) & 0xFF)
		buff[(4*i)+2] = byte((random >> 8) & 0xFF)
		buff[(4*i)+3] = byte(random & 0xFF)
	}

	return buff
}

func genRandomTraceId() trace.TraceID {
	buff := trace.TraceID{}
	for i := 0; i < 4; i++ {
		random := rand.Int31()
		buff[(4 * i)] = byte((random >> 24) & 0xFF)
		buff[(4*i)+1] = byte((random >> 16) & 0xFF)
		buff[(4*i)+2] = byte((random >> 8) & 0xFF)
		buff[(4*i)+3] = byte(random & 0xFF)
	}

	return buff
}

func (i *Instrumentor) convertEvent(e *Event) *events.Event {
	topic := unix.ByteSliceToString(e.Topic[:])
	key := unix.ByteSliceToString(e.Key[:])
	value := unix.ByteSliceToString(e.Value[:])

	//headerKey1 := unix.ByteSliceToString(e.Header1[:])
	//headerKey2 := unix.ByteSliceToString(e.Header2[:])
	//headerKey3 := unix.ByteSliceToString(e.Header3[:])
	//headerValue1 := unix.ByteSliceToString(e.Value1[:])
	//headerValue2 := unix.ByteSliceToString(e.Value2[:])
	//headerValue3 := unix.ByteSliceToString(e.Value3[:])

	psc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    e.ParentSpanContext.TraceID,
		SpanID:     e.ParentSpanContext.SpanID,
		TraceFlags: trace.FlagsSampled,
		Remote:     e.IsGoroutine > 0,
	})

	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    e.SpanContext.TraceID,
		SpanID:     e.SpanContext.SpanID,
		TraceFlags: trace.FlagsSampled,
	})

	return &events.Event{
		Name:              fmt.Sprintf("Sarama topic: %s", topic),
		Library:           i.LibraryName(),
		Kind:              trace.SpanKindServer,
		StartTime:         int64(e.StartTime),
		EndTime:           int64(e.EndTime),
		SpanContext:       &sc,
		ParentSpanContext: &psc,
		Attributes: []attribute.KeyValue{
			attribute.Key("key").String(key),
			attribute.Key("value").String(value),
			attribute.Key("go-id").Int64(int64(e.Goid)),
			//// Header 1
			//attribute.Key("header key 1").String(headerKey1),
			//attribute.Key("header value 1").String(headerValue1),
			//// Header 2
			//attribute.Key("header key 2").String(headerKey2),
			//attribute.Key("header value 2").String(headerValue2),
			//// Header 3
			//attribute.Key("header key 3").String(headerKey3),
			//attribute.Key("header value 3").String(headerValue3),
		},
	}
}

func (i *Instrumentor) Close() {
	log.Logger.V(0).Info("closing IBM/sarama instrumentor")
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
