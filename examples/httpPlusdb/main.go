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
	"context"
	"database/sql"
	"fmt"
	"go.opentelemetry.io/otel/trace"
	"net/http"
	"os"

	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
)

const sqlQuery = "SELECT * FROM contacts"
const dbName = "test.db"
const tableDefinition = `CREATE TABLE contacts (
							contact_id INTEGER PRIMARY KEY,
							first_name TEXT NOT NULL,
							last_name TEXT NOT NULL,
							email TEXT NOT NULL UNIQUE,
							phone TEXT NOT NULL UNIQUE);`
const tableInsertion = `INSERT INTO 'contacts'
						('first_name', 'last_name', 'email', 'phone') VALUES
						('Moshe', 'Levi', 'moshe@gmail.com', '052-1234567');`

// Server is Http server that exposes multiple endpoints.
type Server struct {
	db *sql.DB
}

// Create the db file.
func CreateDb() {
	file, err := os.Create(dbName)
	if err != nil {
		panic(err)
	}
	err = file.Close()
	if err != nil {
		panic(err)
	}
}

// NewServer creates a server struct after creating the DB and initializing it
// and creating a table named 'contacts' and adding a single row to it.
func NewServer() *Server {
	CreateDb()

	database, err := sql.Open("sqlite3", dbName)

	if err != nil {
		panic(err)
	}

	_, err = database.Exec(tableDefinition)

	if err != nil {
		panic(err)
	}

	_, err = database.Exec(tableInsertion)

	if err != nil {
		panic(err)
	}

	return &Server{
		db: database,
	}
}

func (s *Server) querying(ctx context.Context, w http.ResponseWriter) {
	conn, err := s.db.Conn(ctx)

	if err != nil {
		panic(err)
	}

	rows, err := conn.QueryContext(ctx, sqlQuery)
	if err != nil {
		panic(err)
	}

	logger.Info("queryDb called")
	for rows.Next() {
		var id int
		var firstName string
		var lastName string
		var email string
		var phone string
		err := rows.Scan(&id, &firstName, &lastName, &email, &phone)
		if err != nil {
			panic(err)
		}
		fmt.Fprintf(w, "ID: %d, firstName: %s, lastName: %s, email: %s, phone: %s\n", id, firstName, lastName, email, phone)
	}
}

func (s *Server) queryingWithoutWrite(ctx context.Context) {
	conn, err := s.db.Conn(ctx)

	if err != nil {
		panic(err)
	}

	rows, err := conn.QueryContext(ctx, sqlQuery)
	if err != nil {
		panic(err)
	}

	logger.Info("queryDb called")
	for rows.Next() {
		var id int
		var firstName string
		var lastName string
		var email string
		var phone string
		err := rows.Scan(&id, &firstName, &lastName, &email, &phone)
		if err != nil {
			panic(err)
		}
	}
}

func (s *Server) queryDb(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	s.querying(ctx, w)
	s.queryingWithoutWrite(ctx)

	traceCtx := trace.SpanContextFromContext(ctx)
	logger.Info("check var queryingWithoutWrite: ",
		zap.String("trace id", traceCtx.TraceID().String()),
		zap.String("span id", traceCtx.SpanID().String()),
	)
}

var logger *zap.Logger

func setupHandler(s *Server) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/query_db", s.queryDb)
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
	logger.Info("starting http server", zap.String("port", port))
	logger.Info("check if reload")

	s := NewServer()
	mux := setupHandler(s)
	if err := http.ListenAndServe(port, mux); err != nil {
		logger.Error("error running server", zap.Error(err))
	}
}
