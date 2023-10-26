package gmap

import (
	"fmt"
	"go.opentelemetry.io/auto/pkg/instrumentors/events"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/exp/rand"
	"sync"
	"time"

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

// GMapEvent Define gmap event
type GMapEvent struct {
	Key   uint64
	Value uint64
	Sc    context.EBPFSpanContext
	Type  uint64
}

// EnrichGMapEvent Define gmap event with addition field
type EnrichGMapEvent struct {
	Key uint64
	Sc  context.EBPFSpanContext
	Psc context.EBPFSpanContext
}

func RegisterSpan(event *EnrichGMapEvent, lib string, replace bool) {
	goid := event.Key

	// if goroutine id already taken, then skip
	_, ok := GetGoId2Sc(goid)
	if ok {
		// for server, replace whenever got new event
		if replace {
			fmt.Printf("Replace goid %d, with pid: %s - sid: %s",
				goid,
				event.Sc.TraceID,
				event.Sc.SpanID)
			SetGoId2Sc(goid, event.Sc)
		}
	} else {
		if psc, ok := GetAncestorSc(goid); ok {
			// create new middleware for goroutine, p is founded ancestor, c is created new
			event.Psc = psc
			event.Sc.TraceID = event.Psc.TraceID
			event.Sc.SpanID = GenRandomSpanId()
		}

		// set value of goroutine in current node to middleware
		// all request after this will be the child of this middleware
		fmt.Printf("Set goid %d, with pid: %s - sid: %s\n",
			goid,
			event.Sc.TraceID,
			event.Sc.SpanID)
		SetGoId2Sc(goid, event.Sc)
	}
}

func EnrichSpan(event context.IBaseSpan, goid uint64, lib string) {
	currentSc := event.GetSpanContext()
	sc, ok := GetGoId2Sc(goid)
	if ok { // same goroutine sc exist
		if currentSc.SpanID.String() != sc.SpanID.String() {
			currentSc.TraceID = sc.TraceID
			event.SetSpanContext(currentSc)
			event.SetParentSpanContext(sc)
		}
	} else {
		psc, ok := GetAncestorSc(goid)
		if ok { // parent goroutine sc exist
			event.SetParentSpanContext(psc)
			currentSc.TraceID = psc.TraceID
			event.SetSpanContext(currentSc)
		}
	}
}

func ConvertEnrichEvent(event GMapEvent) EnrichGMapEvent {
	return EnrichGMapEvent{
		Key: event.Key,
		Sc:  event.Sc,
		Psc: context.EBPFSpanContext{},
	}
}

// ConvertEvent convert new goroutine event
func ConvertEvent(event EnrichGMapEvent) *events.Event {
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    event.Sc.TraceID,
		SpanID:     event.Sc.SpanID,
		TraceFlags: trace.FlagsSampled,
	})

	psc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    event.Psc.TraceID,
		SpanID:     event.Psc.SpanID,
		TraceFlags: trace.FlagsSampled,
	})

	return &events.Event{
		Library: "go.opentelemetry.io/auto/pkg/instrumentors/gmap",
		// Do not include the high-cardinality path here (there is no
		// templatized path manifest to reference, given we are instrumenting
		// Engine.ServeHTTP which is not passed a Gin Context).
		Name:              "goroutine",
		Kind:              trace.SpanKindInternal,
		StartTime:         time.Now().Unix(), // TODO check
		EndTime:           time.Now().Unix(),
		SpanContext:       &sc,
		ParentSpanContext: &psc,
	}
}

func GenRandomSpanId() trace.SpanID {
	rand.Seed(uint64(time.Now().UnixNano()))
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

func GenRandomTraceId() trace.TraceID {
	rand.Seed(uint64(time.Now().UnixNano()))
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
