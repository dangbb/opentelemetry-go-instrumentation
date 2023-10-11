package gmap

import (
	"fmt"
	"sync"

	"go.opentelemetry.io/auto/pkg/instrumentors/context"
)

// using map for PoC
var (
	goPc2PGoId     = map[uint64]uint64{}
	goPc2PGoIdLock = sync.Mutex{}

	curThread2GoId     = map[uint64]uint64{}
	curThread2GoIdLock = sync.Mutex{}

	goId2PGoId     = map[uint64]uint64{}
	goId2PGoIdLock = sync.Mutex{}

	goId2Sc     = map[uint64]context.EBPFSpanContext{}
	goId2ScLock = sync.Mutex{}

	maxRange = 20
)

func GetAncestorSc(goid uint64) (context.EBPFSpanContext, bool) {
	for i := 0; i < maxRange; i++ {
		pgoid, ok := GetGoId2PGoId(goid)
		if !ok {
			return context.EBPFSpanContext{}, false
		}

		sc, ok := GetGoId2Sc(pgoid)
		if !ok {
			goid = pgoid
			continue
		}

		return sc, true
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

func SetCurThread2GoId(key, value uint64) {
	curThread2GoIdLock.Lock()
	defer curThread2GoIdLock.Unlock()

	fmt.Printf("Map thread %d to %d\n", key, value)

	curThread2GoId[key] = value
}

func GetCurThread2GoId(key uint64) (uint64, bool) {
	curThread2GoIdLock.Lock()
	defer curThread2GoIdLock.Unlock()

	res, ok := curThread2GoId[key]
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
