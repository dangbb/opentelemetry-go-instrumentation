package utils

import (
	"container/heap"
	"fmt"
	"go.opentelemetry.io/auto/pkg/log"
	"sync"
	"time"
)

type ItemType string

// An Item is something we manage in a priority queue.
type Item struct {
	value interface{} // The value of the item; arbitrary.
	iType ItemType

	arriveAt uint64 // the time at which record is arrived.
	priority uint64 // The priority of the item in the queue.
	index    int    // The index of the item in the heap.
}

type PriorityQueue []*Item

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
	return pq[i].priority < pq[j].priority
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *PriorityQueue) Push(x any) {
	n := len(*pq)
	item := x.(*Item)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *PriorityQueue) Pop() any {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // avoid memory leak
	item.index = -1 // for safety
	*pq = old[0 : n-1]
	return item
}

// Define a wrapper for main queue
type EventPriorityQueue struct {
	queue         PriorityQueue
	delayDuration time.Duration
	maxSize       uint64
	mu            sync.Mutex

	handlerMap map[ItemType]func(interface{})

	done chan struct{}
}

var (
	EventProrityQueueSingleton *EventPriorityQueue
)

func Initialize(delayDuration time.Duration, maxSize uint64) {
	if EventProrityQueueSingleton != nil {
		return
	}

	EventProrityQueueSingleton = &EventPriorityQueue{
		queue:         make(PriorityQueue, 0),
		delayDuration: -1 * delayDuration,
		maxSize:       maxSize,
		mu:            sync.Mutex{},
		handlerMap:    make(map[ItemType]func(interface{})),
	}

	heap.Init(&EventProrityQueueSingleton.queue)

	log.Logger.V(0).Info("Done initialize priority queue singleton")
}

func (epq *EventPriorityQueue) Push(event interface{}, priority uint64, iType ItemType) {
	EventProrityQueueSingleton.mu.Lock()
	defer EventProrityQueueSingleton.mu.Unlock()

	if epq.maxSize != 0 && epq.queue.Len() >= int(epq.maxSize) {
		// TODO add count to number of event ignored
		return
	}

	heap.Push(&epq.queue, &Item{
		value:    event,
		iType:    iType,
		arriveAt: uint64(time.Now().UnixNano()),
		priority: priority,
	})
}

func (epq *EventPriorityQueue) Register(iType ItemType, handler func(interface{})) {
	EventProrityQueueSingleton.mu.Lock()
	defer EventProrityQueueSingleton.mu.Unlock()

	epq.handlerMap[iType] = handler
}

func (epq *EventPriorityQueue) Unregister(iType ItemType) {
	EventProrityQueueSingleton.mu.Lock()
	defer EventProrityQueueSingleton.mu.Unlock()

	delete(epq.handlerMap, iType)
}

func (epq *EventPriorityQueue) Run() {
	log.Logger.Info("Start priority queue for event")

	previousPriority := uint64(0)

	go func() {
		for {
			select {
			case <-epq.done:
				break
			default:
				func() {
					EventProrityQueueSingleton.mu.Lock()
					defer EventProrityQueueSingleton.mu.Unlock()

					if epq.queue.Len() == 0 {
						return
					}

					event := heap.Pop(&epq.queue).(*Item)
					if event.arriveAt > uint64(time.Now().Add(epq.delayDuration).UnixNano()) {
						heap.Push(&epq.queue, event)
						return
					}

					if event.priority < previousPriority {
						log.Logger.Info("[ERROR] - The incoming request is not following order")
					}
					previousPriority = event.priority

					handler, ok := epq.handlerMap[event.iType]
					if !ok {
						log.Logger.Info(fmt.Sprintf("Error when find handler for %s", event.iType))
						return
					}

					handler(event.value)
				}()
			}
		}
	}()
}

func (epq *EventPriorityQueue) Close() {
	epq.done <- struct{}{}
}
