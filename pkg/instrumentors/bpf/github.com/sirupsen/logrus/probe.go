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

//
//import (
//	"github.com/cilium/ebpf"
//	"github.com/cilium/ebpf/link"
//	"github.com/cilium/ebpf/perf"
//	"go.opentelemetry.io/auto/pkg/instrumentors/bpffs"
//	"go.opentelemetry.io/auto/pkg/instrumentors/context"
//	"go.opentelemetry.io/auto/pkg/instrumentors/utils"
//	"os"
//)
//
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64,arm64 -cc clang -cflags $CFLAGS bpf ./bpf/probe.bpf.c
//
//const instrumentedPkg = "sirupsen/logrus"
//
//type Event struct {
//	context.BaseSpanProperties
//	level uint64
//	log   [100]byte
//}
//
//type Instrumentor struct {
//	bpfObjects   *bpfObjects
//	uprobes      []link.Link
//	returnProbes []link.Link
//	eventReader  *perf.Reader
//}
//
//func New() *Instrumentor {
//	return &Instrumentor{}
//}
//
//func (i *Instrumentor) LibraryName() string {
//	return instrumentedPkg
//}
//
//func (i *Instrumentor) Load(ctx *context.InstrumentorContext) error {
//	spec, err := ctx.Injector.Inject(loadBpf, "go", ctx.TargetDetails.GoVersion.Original(), nil, nil, false)
//	if err != nil {
//		return err
//	}
//
//	i.bpfObjects = &bpfObjects{}
//	err = utils.LoadEBPFObjects(spec, i.bpfObjects, &ebpf.CollectionOptions{
//		Maps: ebpf.MapOptions{
//			PinPath: bpffs.PathForTargetApplication(ctx.TargetDetails),
//		},
//	})
//	if err != nil {
//		return err
//	}
//
//	for _, funcName := range i.FuncNames() {
//		i.registerProbes(ctx, funcName)
//	}
//	rd, err := perf.NewReader(i.bpfObjects.Events, os.Getpagesize())
//	if err != nil {
//		return err
//	}
//	i.eventsReader = rd
//
//	return nil
//}
//
//func (g *Instrumentor) registerProbes(ctx *context.InstrumentorContext, funcName string) {
//	logger := log.Logger.WithName(instrumentedPkg).WithValues("function", funcName)
//	offset, err := ctx.TargetDetails.GetFunctionOffset(funcName)
//	if err != nil {
//		logger.Error(err, "could not find function start offset. Skipping")
//		return
//	}
//	retOffsets, err := ctx.TargetDetails.GetFunctionReturns(funcName)
//	if err != nil {
//		logger.Error(err, "could not find function end offset. Skipping")
//		return
//	}
//
//	up, err := ctx.Executable.Uprobe("", g.bpfObjects.UprobeGorillaMuxServeHTTP, &link.UprobeOptions{
//		Address: offset,
//	})
//	if err != nil {
//		logger.Error(err, "could not insert start uprobe. Skipping")
//		return
//	}
//
//	g.uprobes = append(g.uprobes, up)
//
//	for _, ret := range retOffsets {
//		retProbe, err := ctx.Executable.Uprobe("", g.bpfObjects.UprobeGorillaMuxServeHTTP_Returns, &link.UprobeOptions{
//			Address: ret,
//		})
//		if err != nil {
//			logger.Error(err, "could not insert return uprobe. Skipping")
//			return
//		}
//		g.returnProbs = append(g.returnProbs, retProbe)
//	}
//}
