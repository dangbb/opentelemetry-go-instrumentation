package gmap

import (
	"fmt"
	"sync"

	"go.opentelemetry.io/auto/pkg/instrumentors/constant"
	"go.opentelemetry.io/auto/pkg/instrumentors/context"
)

const (
	GoPc2PGoId = iota + 1
	GoId2GoPc
	GoId2Sc
)

// using map for PoC
var (
	goPc2PGoId     = map[uint64]uint64{}
	goPc2PGoIdLock = sync.Mutex{}

	goId2PGoId     = map[uint64]uint64{}
	goId2PGoIdLock = sync.Mutex{}

	goId2Sc     = map[uint64]context.EBPFSpanContext{}
	goId2ScLock = sync.Mutex{}

	maxRange = 20
)

func getAncestorSc(goid uint64) (context.EBPFSpanContext, bool, bool) {
	for {
		pgoid, ok := GetGoId2PGoId(goid)
		if !ok {
			// only when reach root, or span is consider to be incomplete and should be rerun
			return context.EBPFSpanContext{}, false, goid == 1
		}

		sc, ok := GetGoId2Sc(pgoid)
		if !ok {
			goid = pgoid
			continue
		}

		if pgoid == 1 {
			// ignore case where pgoid = 1. No connect to root cause.
			continue
		}

		return sc, true, false
	}
}

func GetAncestorSc(goid uint64) (context.EBPFSpanContext, bool) {
	for i := 0; i < constant.MAX_RETRY; i++ {
		// TODO add prometheus metric for counting number of retry
		sc, ok, retry := getAncestorSc(goid)
		if !retry {
			return sc, ok
		}
	}

	return context.EBPFSpanContext{}, false
}

func SetGoPc2GoId(key, value uint64) {
	goPc2PGoIdLock.Lock()
	defer goPc2PGoIdLock.Unlock()

	goPc2PGoId[key] = value

	fmt.Printf("Map pc 2 pgoid %d to %d\n", key, value)
}

func GetGoPc2GoId(key uint64) (uint64, bool) {
	goPc2PGoIdLock.Lock()
	defer goPc2PGoIdLock.Unlock()

	res, ok := goPc2PGoId[key]
	return res, ok
}

func SetGoId2PGoId(key, value uint64) {
	goId2PGoIdLock.Lock()
	defer goId2PGoIdLock.Unlock()

	goId2PGoId[key] = value
	fmt.Printf("Create egde %d - %d\n", key, value)
}

func GetGoId2PGoId(key uint64) (uint64, bool) {
	goId2PGoIdLock.Lock()
	defer goId2PGoIdLock.Unlock()

	res, ok := goId2PGoId[key]
	return res, ok
}

func SetGoId2Sc(key uint64, value context.EBPFSpanContext) {
	goId2ScLock.Lock()
	defer goId2ScLock.Unlock()

	goId2Sc[key] = value
}

func GetGoId2Sc(key uint64) (context.EBPFSpanContext, bool) {
	goId2ScLock.Lock()
	defer goId2ScLock.Unlock()

	res, ok := goId2Sc[key]
	return res, ok
}

// Define gmap event
type GMapEvent struct {
	Key   uint64
	Value uint64
	Sc    context.EBPFSpanContext
	Type  uint64
}

func RegisterSpan(event GMapEvent, lib string) {
	goid := event.Key

	// if goroutine id already taken, then skip
	sc, ok := GetGoId2Sc(goid)
	if ok {
		event.Sc.TraceID = sc.TraceID
		fmt.Printf("sc for goid %d exist - %s\n", goid, lib)
		return
	} else {
		_, ok := GetAncestorSc(goid)
		if ok {
			fmt.Printf("logrus found ancestor for %d - %s\n", goid, lib)
		} else {
			SetGoId2Sc(goid, event.Sc)
			fmt.Printf("Type 4 %s set sc for %d - trace id: %s - span id: %s\n",
				lib,
				goid,
				event.Sc.TraceID,
				event.Sc.SpanID)
		}
	}
}

func EnrichSpan(event context.IBaseSpan, goid uint64, lib string) {
	currentSc := event.GetSpanContext()
	sc, ok := GetGoId2Sc(goid)
	if ok { // same goroutine sc exist
		currentSc.TraceID = sc.TraceID
		event.SetSpanContext(currentSc)
		event.SetParentSpanContext(sc)
		fmt.Printf("sc for goid %d exist - %s\n", goid, lib)
	} else {
		psc, ok := GetAncestorSc(goid)
		fmt.Printf("get from ancestor for %d - %s\n", goid, lib)
		if ok { // parent goroutine sc exist
			event.SetParentSpanContext(psc)
			currentSc.TraceID = psc.TraceID
			event.SetSpanContext(currentSc)
			fmt.Printf("ancestor exist %s. take value of ancestor. TraceID: %s - SpanID: %s\n",
				lib,
				psc.TraceID.String(),
				psc.SpanID.String())
		}
	}
}
