// Code generated by bpf2go; DO NOT EDIT.
//go:build 386 || amd64

package runtime

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"

	"github.com/cilium/ebpf"
)

type bpfGmapT struct {
	Key   uint64
	Value uint64
	Sc    bpfSpanContext
	Type  uint64
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
	UprobeRuntimeCasgstatusByRegisters *ebpf.ProgramSpec `ebpf:"uprobe_runtime_casgstatus_ByRegisters"`
	UprobeRuntimeNewproc1              *ebpf.ProgramSpec `ebpf:"uprobe_runtime_newproc1"`
}

// bpfMapSpecs contains maps before they are loaded into the kernel.
//
// It can be passed ebpf.CollectionSpec.Assign.
type bpfMapSpecs struct {
	GmapEvents      *ebpf.MapSpec `ebpf:"gmap_events"`
	GopcToPgoid     *ebpf.MapSpec `ebpf:"gopc_to_pgoid"`
	GoroutinesMap   *ebpf.MapSpec `ebpf:"goroutines_map"`
	P_goroutinesMap *ebpf.MapSpec `ebpf:"p_goroutines_map"`
	PlaceholderMap  *ebpf.MapSpec `ebpf:"placeholder_map"`
	ScMap           *ebpf.MapSpec `ebpf:"sc_map"`
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
	GmapEvents      *ebpf.Map `ebpf:"gmap_events"`
	GopcToPgoid     *ebpf.Map `ebpf:"gopc_to_pgoid"`
	GoroutinesMap   *ebpf.Map `ebpf:"goroutines_map"`
	P_goroutinesMap *ebpf.Map `ebpf:"p_goroutines_map"`
	PlaceholderMap  *ebpf.Map `ebpf:"placeholder_map"`
	ScMap           *ebpf.Map `ebpf:"sc_map"`
}

func (m *bpfMaps) Close() error {
	return _BpfClose(
		m.GmapEvents,
		m.GopcToPgoid,
		m.GoroutinesMap,
		m.P_goroutinesMap,
		m.PlaceholderMap,
		m.ScMap,
	)
}

// bpfPrograms contains all programs after they have been loaded into the kernel.
//
// It can be passed to loadBpfObjects or ebpf.CollectionSpec.LoadAndAssign.
type bpfPrograms struct {
	UprobeRuntimeCasgstatusByRegisters *ebpf.Program `ebpf:"uprobe_runtime_casgstatus_ByRegisters"`
	UprobeRuntimeNewproc1              *ebpf.Program `ebpf:"uprobe_runtime_newproc1"`
}

func (p *bpfPrograms) Close() error {
	return _BpfClose(
		p.UprobeRuntimeCasgstatusByRegisters,
		p.UprobeRuntimeNewproc1,
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
//go:embed bpf_bpfel_x86.o
var _BpfBytes []byte
