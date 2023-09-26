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

/*
MVP Program to test eBPF for logrus, by dang.nh1

Reproduce step:
1. Build binary file of this. Since only running binary give correct behavior.
2. Using `ps aux` to get list of running process, and `grep` out process of `main`
3. Extract the process ID, then provide to param `binary`. Fill `log` to param method.
4. Test.
*/
package main

import (
	"flag"

	"go.opentelemetry.io/auto/pkg/instrumentors/bpf/github.com/IBM/sarama"
)

var (
	binaryProg string
	methodName string
	pid        uint64
)

func init() {
	flag.StringVar(&binaryProg, "binary", "", "The binary to probe")
	flag.StringVar(&methodName, "method", "", "The function name to probe")
	flag.Uint64Var(&pid, "pid", 0, "The function name to probe")
}

func main() {
	flag.Parse()

	if len(binaryProg) == 0 {
		panic("Argument --binary needs to be specified")
	}

	sarama.RunEBPF(binaryProg, methodName, pid)
}
