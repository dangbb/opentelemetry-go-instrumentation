package gmap

import (
	"go.opentelemetry.io/auto/pkg/instrumentors/context"
	"sync"
)

/**
GMAP stands for goroutine map.

Using to store correlation between:
1. current thread id and goroutine id.
2. goroutine id and parent goroutine id.
3. goroutine to span context.

And store following action:
1. Trace back graph to get value of span context.
*/

var (
	gopc2PGoid     = map[uint64]uint64{}
	gopc2PGoidLock = sync.Mutex{}

	threadId2Goid     = map[uint64]uint64{}
	threadId2GoidLock = sync.Mutex{}

	goid2Pgoid     = map[uint64]uint64{}
	goid2PgoidLock = sync.Mutex{}

	goid2Sc     = map[uint64]context.BaseSpanProperties{}
	goid2ScLock = sync.Mutex{}
)

// Gopc to goid
func NewGopc2PGoid(gopc, goid uint64) {
	gopc2PGoidLock.Lock()
	defer gopc2PGoidLock.Unlock()

	gopc2PGoid[gopc] = goid
}

func GetGopc2PGoid(gopc uint64) (uint64, bool) {
	gopc2PGoidLock.Lock()
	defer gopc2PGoidLock.Unlock()

	res, ok := gopc2PGoid[gopc]
	return res, ok
}

// Thread ID 2 GoID
func NewThreadGoIdMapping(threadId uint64, goId uint64) {
	threadId2GoidLock.Lock()
	defer threadId2GoidLock.Unlock()
	threadId2Goid[threadId] = goId
}

func GetThreadGoIdMapping(threadId uint64) (uint64, bool) {
	threadId2GoidLock.Lock()
	defer threadId2GoidLock.Unlock()
	res, ok := threadId2Goid[threadId]
	return res, ok
}

// GoID to Parent Goid
func NewGoid2Pgoid(goid uint64, pgoid uint64) {
	goid2PgoidLock.Lock()
	defer goid2PgoidLock.Unlock()
	goid2Pgoid[goid] = pgoid
}

func GetGoid2Pgoid(goid uint64) (uint64, bool) {
	goid2PgoidLock.Lock()
	defer goid2PgoidLock.Unlock()
	res, ok := goid2Pgoid[goid]
	return res, ok
}

// Go ID to SC
func NewGoid2Sc(goid uint64, sc context.BaseSpanProperties) {
	goid2ScLock.Lock()
	defer goid2ScLock.Unlock()
	goid2Sc[goid] = sc
}

func GetGoid2Sc(goid uint64) (context.BaseSpanProperties, bool) {
	goid2ScLock.Lock()
	defer goid2ScLock.Unlock()
	res, ok := goid2Sc[goid]
	return res, ok
}
