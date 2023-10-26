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
#include "span_context.h"
#include "go_context.h"
#include "uprobe.h"
#include "gmap.h"

char __license[] SEC("license") = "Dual MIT/GPL";

#define MAX_LOG_SIZE 100
#define MAX_CONCURRENT 50

// Define event struct
struct log_event_t {
    BASE_SPAN_PROPERTIES
    u64 level;
    char log[MAX_LOG_SIZE];
    u64 goid;
    u64 is_goroutine;
    u64 cur_thread;
};

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, void *);
    __type(value, struct log_event_t);
    __uint(max_entries, MAX_CONCURRENT);
} log_events SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
} events SEC(".maps"); // handle checkpoint at the return of function

const struct log_event_t *unused __attribute__((unused));

// Define main function to extract information
// Attach probe at function:
// `func (entry *Entry) log(level Level, msg string) {...}`
// We would want to extract `level` and `msg`, so the offset should be
// 2, 3 and 4. (2 for level, 3 for string content and 4 for string length).
// 1 is pointer to entry.

//SEC("uprobe/Logrus_EntryLog")
//int uprobe_Logrus_EntryLog(struct pt_regs *ctx) { // take list of register and stack as input
//    u64 level_pos = 2;
//    u64 str_ptr_pos = 3;
//    u64 str_len_pos = 4;
//
//    struct log_event_t logEvent = {};
//    logEvent.start_time = bpf_ktime_get_ns();
//
//    // get level position
//    logEvent.level = (u64)get_argument(ctx, level_pos);
//
//    // get string length and string content
//    void *str_ptr = get_argument(ctx, str_ptr_pos);
//    u64 str_len = (u64)get_argument(ctx, str_len_pos);
//    u64 str_size = MAX_LOG_SIZE < str_len ? MAX_LOG_SIZE : str_len;
//    bpf_probe_read(logEvent.log, str_size, str_ptr);
//
//    // set span context
//    logEvent.sc = generate_span_context();
//
//    // add to perf map
//    // BPF_F_CURRENT_CPU flaf option ?
//    bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, &logEvent, sizeof(logEvent));
//    return 0;
//};

// Injected in init
volatile const u64 level_ptr_pos;
volatile const u64 message_ptr_pos;

// Define main function to extract information
// Attach probe at function:
// `func (entry *Entry) write() {...}`
// Extract entry pointer at 1. Then extract Level at 6, and Message & Length at 8 and 9

SEC("uprobe/Logrus_EntryWrite")
int uprobe_Logrus_EntryWrite(struct pt_regs *ctx) { // take list of register and stack as input
    u64 entry_ptr_pos = 1;

    struct log_event_t logEvent = {};
    logEvent.start_time = bpf_ktime_get_ns();

    // get level position
    void *entry_ptr = get_argument(ctx, entry_ptr_pos);

    bpf_probe_read(&logEvent.level, sizeof(logEvent.level), (void *)(entry_ptr + level_ptr_pos));

    u64 msg_len = 0;
    bpf_probe_read(&msg_len, sizeof(msg_len), (void *)(entry_ptr + message_ptr_pos + 8));
    msg_len = msg_len > MAX_LOG_SIZE ? MAX_LOG_SIZE : msg_len;
    void *path_ptr = 0;
    bpf_probe_read(&path_ptr, sizeof(path_ptr), (void *)(entry_ptr + message_ptr_pos));
    bpf_probe_read(&logEvent.log, msg_len, path_ptr);

    u64 goid = logEvent.goid;
    void* same_goroutine_sc_ptr = bpf_map_lookup_elem(&goroutine_sc_map, &goid);

    if (same_goroutine_sc_ptr != NULL) {
        struct span_context sc = {};
        bpf_probe_read(&sc, sizeof(sc), same_goroutine_sc_ptr);

        logEvent.psc = sc;
        copy_byte_arrays(logEvent.psc.TraceID, logEvent.sc.TraceID, TRACE_ID_SIZE);
        generate_random_bytes(logEvent.sc.SpanID, SPAN_ID_SIZE);
    } else {
        logEvent.sc = generate_span_context();
    }

    // add to perf map
    // BPF_F_CURRENT_CPU flaf option ?
    u64 cur_thread = bpf_get_current_pid_tgid();

    logEvent.goid = get_current_goroutine();
    logEvent.cur_thread = cur_thread;

    // send type 3 event
    struct gmap_t event3 = {};

    event3.key = logEvent.goid;
    event3.sc = logEvent.sc;
    event3.type = GOID_SC;
    event3.start_time = logEvent.start_time;

    bpf_perf_event_output(ctx, &gmap_events, BPF_F_CURRENT_CPU, &event3, sizeof(event3));

    void *key = get_consistent_key(ctx, entry_ptr);
    bpf_map_update_elem(&log_events, &key, &logEvent, 0);
    start_tracking_span(entry_ptr, &logEvent.sc);

    // bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, &logEvent, sizeof(logEvent));
    return 0;
};

SEC("uprobe/Logrus_EntryWrite")
int uprobe_Logrus_EntryWrite_Returns(struct pt_regs *ctx) {
    void *req_ptr = get_argument(ctx, 1);         // extract entry
    void *key = get_consistent_key(ctx, req_ptr); // get consistent key as value of req itself
    void *req_ptr_map = bpf_map_lookup_elem(&log_events, &key);

    struct log_event_t tmpReq = {};
    bpf_probe_read(&tmpReq, sizeof(tmpReq), req_ptr_map);
    tmpReq.end_time = bpf_ktime_get_ns();

    tmpReq.goid = get_current_goroutine();

    bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, &tmpReq, sizeof(tmpReq));
    bpf_map_delete_elem(&log_events, &key);
    stop_tracking_span(&tmpReq.sc);
    return 0;
}