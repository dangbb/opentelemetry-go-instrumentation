package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/IBM/sarama"
	"github.com/alecthomas/kong"
	"github.com/gorilla/mux"
	jsoniter "github.com/json-iterator/go"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"microservice/config"
	pb "microservice/pb/proto"
	"microservice/pkg/service"
)

type WarehouseService interface {
	InsertWarehouseHandler(w http.ResponseWriter, r *http.Request)
}

type warehouse struct {
	producer sarama.SyncProducer
	topic    string
	client   pb.AuditServiceClient
}

func (s *warehouse) InsertWarehouseHandler(w http.ResponseWriter, r *http.Request) {
	// extract request content and send to kafka
	var object service.Warehouse
	if err := json.NewDecoder(r.Body).Decode(&object); err != nil {
		responseWithJson(w, http.StatusBadRequest, map[string]string{"message": "Invalid body"})
		return
	}

	valueStr, err := jsoniter.Marshal(object)

	// send to kafka
	msg := &sarama.ProducerMessage{
		Topic: s.topic,
		Key:   sarama.ByteEncoder("key"),
		Value: sarama.ByteEncoder(valueStr),
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
	}

	partition, offset, err := s.producer.SendMessage(msg)
	if err != nil {
		responseWithJson(w, http.StatusInternalServerError, map[string]string{"message": err.Error()})
		return
	}
	logrus.Info("Done send to kafka. Value of partition %d , offset %d\n", partition, offset)

	// create audit record
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	response, err := s.client.AuditSend(ctx, &pb.AuditSendRequest{
		ServiceName: "warehouse",
		RequestType: uint64(service.WarehouseInsert),
	})
	if err != nil {
		responseWithJson(w, http.StatusInternalServerError, map[string]string{"message": err.Error()})
		return
	}

	if response.Code != 200 {
		responseWithJson(w, int(response.Code), map[string]string{"message": response.Message})
		return
	}

	logrus.Info("Done send to audit")

	responseWithJson(w, http.StatusOK, map[string]string{"message": "OK"})
}

func newWarehouseService(config config.Config) (WarehouseService, error) {
	cfg := sarama.NewConfig()

	cfg.Producer.RequiredAcks = sarama.WaitForAll
	cfg.Version = sarama.V2_1_0_0
	cfg.Net.MaxOpenRequests = 1

	cfg.Producer.Compression = sarama.CompressionLZ4
	cfg.Producer.Idempotent = true
	cfg.Producer.Return.Successes = true

	cfg.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategySticky()}

	brokers := []string{config.KafkaConfig.Broker}

	producer, err := sarama.NewSyncProducer(brokers, cfg)

	// craft grpc client instance
	conn, err := grpc.Dial(config.AuditAddress, grpc.WithTransportCredentials(
		insecure.NewCredentials()))
	if err != nil {
		logrus.Fatalf("can establish grpc client conn %s", err.Error())
	}

	c := pb.NewAuditServiceClient(conn)

	return &warehouse{
		producer: producer,
		topic:    config.KafkaConfig.Topic,
		client:   c,
	}, err
}

func responseWithJson(writer http.ResponseWriter, status int, object interface{}) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	err := json.NewEncoder(writer).Encode(object)
	if err != nil {
		return
	}
}

func main() {
	cfg := config.Config{}
	kong.Parse(&cfg)

	service, err := newWarehouseService(cfg)
	if err != nil {
		logrus.Fatalf("error create sync producer %s", err.Error())
	}

	// craft gorilla server
	r := mux.NewRouter()
	r.HandleFunc("/insert-warehouse", service.InsertWarehouseHandler).Methods(http.MethodPost)

	logrus.Infof("Run warehouse server at: 0.0.0.0:%d", cfg.HttpPort)
	log.Fatal(http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", cfg.HttpPort), r))
}
