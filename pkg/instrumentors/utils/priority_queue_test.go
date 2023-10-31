package utils

import (
	"container/heap"
	"fmt"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/auto/pkg/instrumentors/gmap"
)

func runPriorityQueue(nGoroutines int, nLoops int) {
	eventChan := make(chan gmap.GMapEvent, nGoroutines)
	queue := make(PriorityQueue, 0)

	heap.Init(&queue)

	wg := sync.WaitGroup{}
	wg.Add(3)

	lock := sync.Mutex{}

	outputs := uint64(0)

	go func() {
		defer func() {
			wg.Done()

			fmt.Printf("Event creator done\n")

			close(eventChan)
		}()
		for n := 0; n < nLoops; n++ {
			event := gmap.GMapEvent{
				StartTime: uint64(time.Now().UnixNano()),
			}
			eventChan <- event
		}
	}()

	go func() {
		defer func() {
			wg.Done()

			fmt.Printf("Receiver done\n")
		}()
		wg2 := sync.WaitGroup{}
		wg2.Add(nGoroutines)

		for n := 0; n < nGoroutines; n++ {
			go func() {
				defer func() {
					wg2.Done()
				}()

				for {
					select {
					case event, alive := <-eventChan:
						if !alive {
							return
						}

						func() {
							lock.Lock()
							defer lock.Unlock()

							heap.Push(&queue, &Item{
								value:    event,
								priority: event.StartTime,
								arriveAt: uint64(time.Now().UnixNano()),
							})
						}()
					}
				}
			}()
		}

		wg2.Wait()
	}()

	eventCount := 0

	go func() {
		defer func() {
			wg.Done()

			fmt.Printf("Priority queue checker done\n")
		}()

		for eventCount < nLoops {
			func() {
				lock.Lock()
				defer lock.Unlock()

				if queue.Len() > 0 {
					event := heap.Pop(&queue)

					if event.(*Item).arriveAt > uint64(time.Now().Add(-2*time.Second).UnixNano()) {
						heap.Push(&queue, event)
						return
					}

					eventParsed := event.(*Item)
					eventCount += 1

					if outputs > eventParsed.priority {
						panic("Wrong order")

					}
					outputs = eventParsed.priority
				}
			}()
		}
	}()

	wg.Wait()
}

func TestPQueue(t *testing.T) {
	runPriorityQueue(5, 10)
}

func BenchmarkPQueue1(b *testing.B)   { runPriorityQueue(1, b.N) }
func BenchmarkPQueue5(b *testing.B)   { runPriorityQueue(5, b.N) }
func BenchmarkPQueue10(b *testing.B)  { runPriorityQueue(10, b.N) }
func BenchmarkPQueue20(b *testing.B)  { runPriorityQueue(20, b.N) }
func BenchmarkPQueue50(b *testing.B)  { runPriorityQueue(50, b.N) }
func BenchmarkPQueue100(b *testing.B) { runPriorityQueue(100, b.N) }
