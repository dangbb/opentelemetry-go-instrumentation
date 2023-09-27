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

package sarama

import (
	"bytes"
	"encoding/binary"
	"errors"
	logrus_lib "github.com/sirupsen/logrus"
	"os"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/perf"
	"go.opentelemetry.io/auto/pkg/inject"
	"go.opentelemetry.io/auto/pkg/instrumentors/bpffs"
	"go.opentelemetry.io/auto/pkg/instrumentors/context"
	"go.opentelemetry.io/auto/pkg/instrumentors/events"
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
}

type Instrumentor struct {
	bpfObjects   *bpfObjects
	uprobes      []link.Link
	returnProbes []link.Link
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
			VarName:    "offset_ptr_pos",
			StructName: "sarama.ProducerMessage",
			Field:      "Offset",
		},
		{
			VarName:    "partition_ptr_pos",
			StructName: "sarama.ProducerMessage",
			Field:      "Partition",
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

	up, err := ctx.Executable.Uprobe("", i.bpfObjects.UprobeSyncProducerSendMessage, &link.UprobeOptions{
		Address: offset,
	})
	if err != nil {
		logger.Error(err, "could not insert start uprobe. Skipping")
		return
	}

	i.uprobes = append(i.uprobes, up)
}

func (i *Instrumentor) Run(eventsChan chan<- *events.Event) {
	logger := log.Logger.WithName(instrumentorName)
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

		eventsChan <- i.convertEvent(&event)
	}
}

func convertLevel(level uint64) string {
	logrusLevel := logrus_lib.Level(level)
	return logrusLevel.String()
}

func (i *Instrumentor) convertEvent(e *Event) *events.Event {
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    e.SpanContext.TraceID,
		SpanID:     e.SpanContext.SpanID,
		TraceFlags: trace.FlagsSampled,
	})

	return &events.Event{
		Library:     i.LibraryName(),
		Kind:        trace.SpanKindServer,
		StartTime:   int64(e.StartTime),
		EndTime:     int64(e.EndTime),
		SpanContext: &sc,
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

	// no ret uprobe

	if i.bpfObjects != nil {
		i.bpfObjects.Close()
	}
}
