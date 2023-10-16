package main

import (
	"fmt"
	"github.com/alecthomas/kong"
	"github.com/sirupsen/logrus"
	config2 "microservice/config"
	"net/http"
)

func main() {
	config := config2.Config{}
	kong.Parse(config)

	http.HandlerFunc("/get-customer", getCustomer)

	if err := http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", config.HttpPort), nil); err != nil {
		logrus.Fatalf("can listen to port %d\n", config.HttpPort)
	}
}
