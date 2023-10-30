package main

import (
	"fmt"
	http "github.com/helios/go-sdk/proxy-libs/helioshttp"
	"html"
	"microservice/pkg/trace"

	"github.com/alecthomas/kong"
	logrus "github.com/helios/go-sdk/proxy-libs/helioslogrus"

	config2 "microservice/config"
)

func main() {
	trace.InitTrace("customer")
	config := config2.Config{}
	kong.Parse(&config)

	http.HandleFunc("/customer", func(w http.ResponseWriter, r *http.Request) {
		logrus.Infof("Get request to %s", "/customer")
		fmt.Fprintf(w, "Hello, %q\n", html.EscapeString(r.URL.Path))
	})

	logrus.Infof("Start customer service at: 0.0.0.0:%d", 8093)
	if err := http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", 8093), nil); err != nil {
		logrus.Fatalf("can listen to port %d\n", 8093)
	}
}
