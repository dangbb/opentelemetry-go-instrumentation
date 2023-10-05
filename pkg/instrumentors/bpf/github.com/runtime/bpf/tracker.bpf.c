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

#include "arguments.h"
#include "goroutines.h"

char __license[] SEC("license") = "Dual MIT/GPL";

// Injected in init
volatile const u64 goid_pos;

SEC("uprobe/runtime_casgstatus")
int uprobe_runtime_casgstatus_ByRegisters(struct pt_regs *ctx) {
    u64 newval = (u64)get_argument(ctx, 3); // 3rd argument
    if (newval != 2) { // not _Grunning flag
        return 0;
    }

    void* g_ptr = get_argument(ctx, 1); // get 1st argument - struct of goroutine
    u64 goid = 0;

    bpf_probe_read(&goid, sizeof(goid), (void *)(g_ptr+goid_pos));
    u64 current_thread = bpf_get_current_pid_tgid();
    bpf_map_update_elem(&goroutines_map, &current_thread, &goid, 0);

//    u64 curg = get_current_goroutine();
//    bpf_printk("Extracted value of goid: %d - on thread %d - curg %d", goid, current_thread, curg);

    return 0;
}