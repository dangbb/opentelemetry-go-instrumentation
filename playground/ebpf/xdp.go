package main

import (
	"encoding/binary"
	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/perf"
)

const (
	TYPE_ENTER = 1
	TYPE_DROP  = 2
	TYPE_PASS  = 3
)

type event struct {
	TimeSinceBoot  uint64
	ProcessingTime uint32
	Type           uint8
}

const ringBufferSize = 128

type ringBuffer struct {
	data    [ringBufferSize]uint32
	start   int
	pointer int
	filled  bool
}

func main() {
	/**
	Setup part
	 */
	spec, err := ebpf.("bpf/xdp.c")
	// loading spec, parse ELF file into spec

	col, err := ebpf.NewCollection(spec)
	// create new collection out of spec.
	// Collection is combination of programs and maps
	defer col.Close()

	prog := col.Programs["xdp"]
	// extract program using mapping key name

	/**
	Attach program into XDP
	*/
	lnk, _ := link.AttachXDP(
		link.XDPOptions{
			Program:   prog,
			Interface: iface_idx.Index,
		},
	)

	/**
	Output path
	4096 ?
	`output_map` = match with SEC(".map") we've defined
	 */
	outputMap, ok := col.Maps["output_map"]
	perfEvent, err := perf.NewReader(outputMap, 4096)

	go func() {
		for {
			record, err := perfEvent.Read()

			// why 12, we have 1x u64, 1x u32, 1xu8 ?
			if len(record.RawSample) < 13 {
				// invalid sample size
				/*
				So, raw sample is an array of bytes.
				 */
				continue
			}

			// reference to  struct we define in SEC(".maps"), at `.map` section
			timeSinceBoot := binary.LittleEndian.Uint64(record.RawSample[:8])
			processingTime := binary.LittleEndian.Uint32(record.RawSample[8:12])
			recordType := uint8(record.RawSample[12])

			//printing/processing/do everything ...
		}
	}()
}
