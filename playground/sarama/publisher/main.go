package main

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/IBM/sarama"
)

var producer sarama.SyncProducer
var err error

func newSyncPublisher() (sarama.SyncProducer, error) {
	cfg := sarama.NewConfig()

	cfg.Producer.RequiredAcks = sarama.WaitForAll
	cfg.Version = sarama.V2_1_0_0
	cfg.Net.MaxOpenRequests = 1

	cfg.Producer.Compression = sarama.CompressionLZ4
	cfg.Producer.Idempotent = true
	cfg.Producer.Return.Successes = true

	cfg.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategySticky()}

	brokers := []string{"localhost:9092"}

	logrus.Info("Broker at localhost:9092")

	return sarama.NewSyncProducer(brokers, cfg)
}

func sendKafka(id string) {
	if err != nil {
		panic(err)
	}

	msg := &sarama.ProducerMessage{
		Topic: "123",
		Key:   sarama.ByteEncoder(fmt.Sprintf("key %s", id)),
		Value: sarama.ByteEncoder(fmt.Sprintf("value 2 %s", id)),
		Headers: []sarama.RecordHeader{
			{
				Key:   []byte("header-key"),
				Value: []byte("header-value"),
			},
			{
				Key:   []byte("header-key-2"),
				Value: []byte("header-value-2"),
			},
		},
		Metadata:  nil,
		Offset:    11,
		Partition: 13,
		Timestamp: time.Time{},
	}

	fmt.Println("Send message")
	partition, offset, err := producer.SendMessage(msg)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Value of partition %d , offset %d\n", partition, offset)
}

func logLogrus() {
	logrus.SetLevel(logrus.DebugLevel)

	logrus.Trace("Something very low level.")
	logrus.Debug("Useful debugging information.")
	logrus.Info("Something noteworthy happened!")
	logrus.Warn("You should probably take a look at this.")
}

//go:noinline
func computeE(iterations int64) float64 {
	res := 2.0
	fact := 1.0

	for i := int64(2); i < iterations; i++ {
		fact *= float64(i)
		res += 1 / fact
	}

	// test library IBM/sarama
	wg := sync.WaitGroup{}
	wg.Add(3)

	go func() {
		defer wg.Done()
		sendKafka("1")
	}()
	//// check if 2 different function produce same consistent key
	go func() {
		defer wg.Done()
		sendKafka("2")
	}()

	go func() {
		defer wg.Done()
		sendKafka("3")
	}()

	wg.Wait()

	// test library sirupsen/logrus
	go logLogrus()

	logrus.SetLevel(logrus.DebugLevel)

	logrus.Trace("Same goroutine")
	logrus.Debug("Same goroutine")
	logrus.Info("Same goroutine")
	logrus.Warn("Same goroutine")

	return res
}

func main() {
	producer, err = newSyncPublisher()
	if err != nil {
		panic(err)
	}

	addr := ":9090"
	http.HandleFunc("/e", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		iters := int64(100)
		keys, ok := r.URL.Query()["iters"]
		if ok && len(keys[0]) >= 1 {
			val, err := strconv.ParseInt(keys[0], 10, 64)
			if err != nil || val <= 0 {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			iters = val
		}

		w.Write([]byte(fmt.Sprintf("e = %0.4f\n", computeE(iters))))
	})

	fmt.Printf("Starting server on: %+v\n", addr)
	err := http.ListenAndServe(addr, nil)
	if err != nil && err != http.ErrServerClosed {
		fmt.Printf("Failed to run http server: %v\n", err)
	}

	producer.Close()
}
