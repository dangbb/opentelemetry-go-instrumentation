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

// Package context provides context utilities.
//
// Deprecated: This package is no longer supported.
package context

import (
	"github.com/cilium/ebpf/link"

	"go.opentelemetry.io/auto/pkg/inject"  // nolint:staticcheck  // Atomic deprecation.
	"go.opentelemetry.io/auto/pkg/process" // nolint:staticcheck  // Atomic deprecation.
)

// InstrumentorContext holds the state of the auto-instrumentation system.
type InstrumentorContext struct {
	TargetDetails *process.TargetDetails
	Executable    *link.Executable
	Injector      *inject.Injector
}
