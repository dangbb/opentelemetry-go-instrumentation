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
#include "gmap.h"

char __license[] SEC("license") = "Dual MIT/GPL";

// Injected in init
volatile const u64 goid_pos;

SEC("uprobe/runtime_newproc1")
int uprobe_runtime_newproc1(struct pt_regs *ctx) {
    void *callergp_ptr = get_argument(ctx, 2);

    u64 goid = 0;
    bpf_probe_read(&goid, sizeof(goid), (void *)(callergp_ptr + 152));

    return 0;
}

SEC("uprobe/runtime_casgstatus")
int uprobe_runtime_casgstatus_ByRegisters(struct pt_regs *ctx) {
    void *newg = get_argument(ctx, 1);
    u64 oldval = (u64)get_argument(ctx, 2);
    u64 newval = (u64)get_argument(ctx, 3); // newval value

    u64 goid = 0;

    bpf_probe_read(&goid, sizeof(goid), (void *)(newg+goid_pos));

    // extract value of gopc (caller)
    u64 gopc = 0;
    bpf_probe_read(&gopc, sizeof(gopc), (void *)(newg + 296));

    // extract value of startpc (executor)
    u64 startpc = 0;
    bpf_probe_read(&startpc, sizeof(startpc), (void *)(newg + 312));

    // extract current goroutine
    u64 cur_goid = get_current_goroutine();
    u64 current_thread = bpf_get_current_pid_tgid();

    // creating
    if (newval == 1 && oldval == 6) {
        // send type 1 event
        struct gmap_t event1 = {};

        event1.key = gopc;
        event1.value = cur_goid;
        event1.type = GOPC_PGOID;

        bpf_perf_event_output(ctx, &gmap_events, BPF_F_CURRENT_CPU, &event1, sizeof(event1));
        bpf_printk("Type 1, gopc %d - pgoid %d", gopc, cur_goid);
        return 0;
    }

    // running
    if (newval == 2) {
        bpf_map_update_elem(&goroutines_map, &current_thread, &goid, 0);

        // send type 2 event
        struct gmap_t event2 = {};

        event2.key = current_thread;
        event2.value = goid;
        event2.type = CURTHREAD_GOID;

        bpf_perf_event_output(ctx, &gmap_events, BPF_F_CURRENT_CPU, &event2, sizeof(event2));
        bpf_printk("Type 2, cur thread %d - goid %d", current_thread, goid);

        // send type 3 event
        struct gmap_t event3 = {};

        event3.key = current_thread;
        event3.value = gopc;
        event3.type = CURTHREAD_GOPC;

        bpf_perf_event_output(ctx, &gmap_events, BPF_F_CURRENT_CPU, &event3, sizeof(event3));
        bpf_printk("Type 3, cur thread %d - gopc %d", current_thread, gopc);

        return 0;
    }

    return 0;
}