package main

import (
	"context"
	"fmt"
	"github.com/IBM/sarama"
	"github.com/sirupsen/logrus"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// soft inject ?
func goid() int {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	idField := strings.Fields(strings.TrimPrefix(string(buf[:n]), "goroutine "))[0]
	id, err := strconv.Atoi(idField)
	if err != nil {
		panic(fmt.Sprintf("cannot get goroutine id: %v", err))
	}
	return id
}

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

	return sarama.NewSyncProducer(brokers, cfg)
}

func sendKafka(id string) {
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

	_, _, err := producer.SendMessage(msg)
	if err != nil {
		panic(err)
	}
}

func SendKafka(ctx context.Context, id string) {
	if err != nil {
		panic(err)
	}

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	cmax := 999
	count := 0

	for {
		select {
		case <-ticker.C:
			logrus.Infof("Trigger send to kafka for goroutine %s\n", id)
			sendKafka(id)
			count += 1

			if count > cmax {
				logrus.Infof("Stop goroutine %s\n", id)
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

//func logLogrus() {
//	logrus.SetLevel(logrus.DebugLevel)
//
//	logrus.Trace("Something very low level.")
//	logrus.Debug("Useful debugging information.")
//	logrus.Info("Something noteworthy happened!")
//	logrus.Warn("You should probably take a look at this.")
//}

func main() {
	producer, err = newSyncPublisher()
	defer producer.Close()

	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()

	fmt.Println("main", goid())
	ticker := time.NewTicker(time.Second * 10)

	go func() {
		for {
			select {
			case <-ticker.C:
				ctx := ctx
				go func() {
					logrus.Info(fmt.Sprintf("Value of goroutine id %d\n", goid()))
					id := goid()
					SendKafka(ctx, fmt.Sprint(id))
				}()
			case <-ctx.Done():
				break
			}
		}
	}()

	// Trap SIGINT to trigger a shutdown.
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)

	<-signals // Blocks here until either SIGINT or SIGTERM is received.
	logrus.Info("Graceful shutdown")
}
