package main

import (
	"encoding/json"
	"fmt"
	"github.com/IBM/sarama"
	"log"
	"net/http"

	"github.com/alecthomas/kong"
	"github.com/gorilla/mux"

	"microservice/config"
)

var producer sarama.SyncProducer

func newSyncPublisher(kafkaCfg config.) (sarama.SyncProducer, error) {
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

type Warehouse struct {
	Location string
	Name     string
}

func responseWithJson(writer http.ResponseWriter, status int, object interface{}) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	err := json.NewEncoder(writer).Encode(object)
	if err != nil {
		return
	}
}

func InsertWarehouseHandler(w http.ResponseWriter, r *http.Request) {
	// extract request content and send to kafka
	var object Warehouse
	if err := json.NewDecoder(r.Body).Decode(&object); err != nil {
		responseWithJson(w, http.StatusBadRequest, map[string]string{"message": "Invalid body"})
		return
	}

	// send to kafka

	// response
	responseWithJson(w, http.StatusOK, map[string]string{"message": "OK"})
}

func main() {
	cfg := config.Config{}
	kong.Parse(cfg)

	// craft gorilla server
	r := mux.NewRouter()
	r.HandleFunc("/insert-warehouse", InsertWarehouseHandler).Methods(http.MethodPost)

	log.Fatal(http.ListenAndServe(fmt.Sprintf("0.0.0.0:%s", cfg.HttpPort), r))

	// craft grpc client
	// craft sarama conn
}
