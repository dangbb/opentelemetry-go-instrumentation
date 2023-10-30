package main

import (
	"fmt"
	"html"
	"net/http"
	"strconv"
	"sync"

	"github.com/alecthomas/kong"
	"github.com/sirupsen/logrus"
)

type Config struct {
	Port uint64 `name:"port" help:"HTTP server port" env:"HTTP_PORT" default:"8094"`
}

func doLog(n int) {
	wg := sync.WaitGroup{}
	wg.Add(n)

	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			logrus.Infof("Info at goroutine %d", i)
			logrus.Debugf("Debug at goroutine %d", i)
			logrus.Warnf("Warn at goroutine %d", i)
			logrus.Errorf("Error at goroutine %d", i)
		}()
	}

	wg.Wait()
}

func main() {
	config := Config{}
	kong.Parse(&config)

	logrus.SetLevel(logrus.DebugLevel)

	http.HandleFunc("/entry", func(w http.ResponseWriter, r *http.Request) {
		n := r.URL.Query().Get("n")
		if n == "" {
			http.Error(w, "n loops not found", http.StatusBadRequest)
		}

		nLoops, err := strconv.ParseInt(n, 10, 64)
		if err != nil {
			http.Error(w, fmt.Sprintf("cannot parse %d", nLoops), http.StatusBadRequest)
		}

		doLog(int(nLoops))

		fmt.Fprintf(w, "Hello, %q\n", html.EscapeString(r.URL.Path))
	})

	logrus.Infof("Start service test for logrus at: 0.0.0.0:%d", config.Port)
	if err := http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", config.Port), nil); err != nil {
		logrus.Fatalf("cant listen to port %d\n", config.Port)
	}
}
