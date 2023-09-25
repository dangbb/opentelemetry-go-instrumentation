package main

import (
	"context"
	"fmt"
	"github.com/IBM/sarama"
	"log"
	"os"
)

type TrivialHandler struct{}

func (t *TrivialHandler) Setup(sarama.ConsumerGroupSession) error {
	return nil
}

func (t *TrivialHandler) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

func (t *TrivialHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case message := <-claim.Messages():
			fmt.Printf("message claimed: timestamp = %v, topic = %s, offset: %d, partition: %d\n", message.Timestamp, message.Topic, message.Offset, message.Partition)
			fmt.Printf("Value of key %b\n", message.Key)
			fmt.Printf("Value of key %b\n", message.Value)

			session.MarkMessage(message, "")
		case <-session.Context().Done():
			return nil
		}
	}
}

func newConsumerGroup() (sarama.ConsumerGroup, error) {
	version := sarama.V2_1_0_0

	cfg := sarama.NewConfig()
	cfg.Version = version

	brokers := []string{"localhost:9092"}
	groupId := "sarama"

	return sarama.NewConsumerGroup(brokers, groupId, cfg)
}

func main() {
	sarama.Logger = log.New(os.Stdout, "[sarama] ", log.LstdFlags)

	cs, err := newConsumerGroup()
	if err != nil {
		panic(err)
	}

	for {
		select {
		default:
			err = cs.Consume(context.Background(), []string{"sarama-test"}, &TrivialHandler{})
			if err != nil {
				panic(err)
			}
		}
	}
}
