// Code generated by bpf2go; DO NOT EDIT.
//go:build arm64

package sarama

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"

	"github.com/cilium/ebpf"
)

type bpfPublisherMessageT struct {
	StartTime uint64
	EndTime   uint64
	Sc        bpfSpanContext
	Psc       bpfSpanContext
	Topic     [30]int8
	Key       [20]int8
	Value     [100]int8
	_         [2]byte
	Goid      uint64
}

type bpfSpanContext struct {
	TraceID [16]uint8
	SpanID  [8]uint8
}

// loadBpf returns the embedded CollectionSpec for bpf.
func loadBpf() (*ebpf.CollectionSpec, error) {
	reader := bytes.NewReader(_BpfBytes)
	spec, err := ebpf.LoadCollectionSpecFromReader(reader)
	if err != nil {
		return nil, fmt.Errorf("can't load bpf: %w", err)
	}

	return spec, err
}

// loadBpfObjects loads bpf and converts it into a struct.
//
// The following types are suitable as obj argument:
//
//	*bpfObjects
//	*bpfPrograms
//	*bpfMaps
//
// See ebpf.CollectionSpec.LoadAndAssign documentation for details.
func loadBpfObjects(obj interface{}, opts *ebpf.CollectionOptions) error {
	spec, err := loadBpf()
	if err != nil {
		return err
	}

	return spec.LoadAndAssign(obj, opts)
}

// bpfSpecs contains maps and programs before they are loaded into the kernel.
//
// It can be passed ebpf.CollectionSpec.Assign.
type bpfSpecs struct {
	bpfProgramSpecs
	bpfMapSpecs
}

// bpfSpecs contains programs before they are loaded into the kernel.
//
// It can be passed ebpf.CollectionSpec.Assign.
type bpfProgramSpecs struct {
	UprobeSyncProducerSendMessage        *ebpf.ProgramSpec `ebpf:"uprobe_syncProducer_SendMessage"`
	UprobeSyncProducerSendMessageReturns *ebpf.ProgramSpec `ebpf:"uprobe_syncProducer_SendMessage_Returns"`
}

// bpfMapSpecs contains maps before they are loaded into the kernel.
//
// It can be passed ebpf.CollectionSpec.Assign.
type bpfMapSpecs struct {
	Events                 *ebpf.MapSpec `ebpf:"events"`
	GoroutinesMap          *ebpf.MapSpec `ebpf:"goroutines_map"`
	PublisherMessageEvents *ebpf.MapSpec `ebpf:"publisher_message_events"`
	TrackedSpans           *ebpf.MapSpec `ebpf:"tracked_spans"`
	TrackedSpansBySc       *ebpf.MapSpec `ebpf:"tracked_spans_by_sc"`
}

// bpfObjects contains all objects after they have been loaded into the kernel.
//
// It can be passed to loadBpfObjects or ebpf.CollectionSpec.LoadAndAssign.
type bpfObjects struct {
	bpfPrograms
	bpfMaps
}

func (o *bpfObjects) Close() error {
	return _BpfClose(
		&o.bpfPrograms,
		&o.bpfMaps,
	)
}

// bpfMaps contains all maps after they have been loaded into the kernel.
//
// It can be passed to loadBpfObjects or ebpf.CollectionSpec.LoadAndAssign.
type bpfMaps struct {
	Events                 *ebpf.Map `ebpf:"events"`
	GoroutinesMap          *ebpf.Map `ebpf:"goroutines_map"`
	PublisherMessageEvents *ebpf.Map `ebpf:"publisher_message_events"`
	TrackedSpans           *ebpf.Map `ebpf:"tracked_spans"`
	TrackedSpansBySc       *ebpf.Map `ebpf:"tracked_spans_by_sc"`
}

func (m *bpfMaps) Close() error {
	return _BpfClose(
		m.Events,
		m.GoroutinesMap,
		m.PublisherMessageEvents,
		m.TrackedSpans,
		m.TrackedSpansBySc,
	)
}

// bpfPrograms contains all programs after they have been loaded into the kernel.
//
// It can be passed to loadBpfObjects or ebpf.CollectionSpec.LoadAndAssign.
type bpfPrograms struct {
	UprobeSyncProducerSendMessage        *ebpf.Program `ebpf:"uprobe_syncProducer_SendMessage"`
	UprobeSyncProducerSendMessageReturns *ebpf.Program `ebpf:"uprobe_syncProducer_SendMessage_Returns"`
}

func (p *bpfPrograms) Close() error {
	return _BpfClose(
		p.UprobeSyncProducerSendMessage,
		p.UprobeSyncProducerSendMessageReturns,
	)
}

func _BpfClose(closers ...io.Closer) error {
	for _, closer := range closers {
		if err := closer.Close(); err != nil {
			return err
		}
	}
	return nil
}

// Do not access this directly.
//
//go:embed bpf_bpfel_arm64.o
var _BpfBytes []byte
