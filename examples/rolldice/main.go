// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"go.opentelemetry.io/otel/trace"
	"math/rand"
	"net/http"
	"time"

	"go.uber.org/zap"
)

var logger *zap.Logger

// Server is Http server that exposes multiple endpoints.
type Server struct {
	rand *rand.Rand
}

// NewServer creates a server struct after initialing rand.
func NewServer() *Server {
	rd := rand.New(rand.NewSource(time.Now().Unix()))
	return &Server{
		rand: rd,
	}
}

func (s *Server) rolldice(w http.ResponseWriter, r *http.Request) {
	n := s.rand.Intn(6) + 1
	logger.Info("rolldice called 1 2 3", zap.Int("dice", n))

	logger.Info("rolldice called 1 2 3", zap.Int("dice", n))

	ctx := trace.SpanContextFromContext(r.Context())
	logger.Info("rolldice called 1 2 3", zap.Int("dice", n))

	logger.Info("get request context",
		zap.String("TraceID", ctx.TraceID().String()),
		zap.String("SpanID", ctx.SpanID().String()))

	fmt.Fprintf(w, "%v", n)
}

func setupHandler(s *Server) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/rolldice", s.rolldice)
	return mux
}

func main() {
	var err error
	logger, err = zap.NewDevelopment()
	if err != nil {
		fmt.Printf("error creating zap logger, error:%v", err)
		return
	}
	port := fmt.Sprintf(":%d", 8080)
	logger.Info("starting http server 1 2 3", zap.String("port", port))
	logger.Info("starting http server", zap.String("port", port))
	logger.Info("starting http server", zap.String("port", port))
	logger.Info("init logger")

	s := NewServer()
	mux := setupHandler(s)
	if err := http.ListenAndServe(port, mux); err != nil {
		logger.Error("error running server", zap.Error(err))
	}
}
