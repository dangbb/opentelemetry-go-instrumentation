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
	"go.opentelemetry.io/auto/pkg/runner"
	"os"
	"os/signal"
	"syscall"

	"go.opentelemetry.io/auto/pkg/log"
	"go.opentelemetry.io/auto/pkg/process"
)

const configFileEnvVar = "CONFIG_FILE_PATH"

func main() {
	err := log.Init()
	if err != nil {
		fmt.Printf("could not init logger: %s\n", err)
		os.Exit(1)
	}

	filepath, exists := os.LookupEnv(configFileEnvVar)
	if !exists {
		panic("File path not exist")
	}

	config := process.ParseJobConfig(filepath)
	runners := []*runner.Runner{}

	for _, job := range config.Jobs {
		r := runner.NewRunner(job)

		go func() {
			err = r.Run()
			if err != nil {
				panic(err)
			}
		}()

		runners = append(runners, &r)
	}

	stopper := make(chan os.Signal, 1)
	signal.Notify(stopper, os.Interrupt, syscall.SIGTERM)

	<-stopper
	for _, r := range runners {
		r.Close()
	}

	fmt.Println("Graceful shutdown")
}
