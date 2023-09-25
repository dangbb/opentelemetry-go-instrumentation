package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/IBM/sarama"
)

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

func main() {
	sarama.Logger = log.New(os.Stdout, "[sarama] ", log.LstdFlags)

	publisher, err := newSyncPublisher()
	if err != nil {
		panic(err)
	}

	msg := &sarama.ProducerMessage{
		Topic: "sarama-test",
		Key:   sarama.ByteEncoder("key"),
		Value: sarama.ByteEncoder("value 2"),
		Headers: []sarama.RecordHeader{
			{
				Key:   []byte("header-key"),
				Value: []byte("header-value"),
			},
		},
		Metadata:  nil,
		Offset:    0,
		Partition: 0,
		Timestamp: time.Time{},
	}

	fmt.Println("Send message")
	partition, offset, err := publisher.SendMessage(msg)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Value of partition %d , offset %d\n", partition, offset)
	publisher.Close()
}
