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

#define PATH_MAX_LEN 100
#define METHOD_MAX_LEN 7
#define MAX_CONCURRENT 50

struct http_request_t {
    BASE_SPAN_PROPERTIES
    char method[METHOD_MAX_LEN];
    char path[PATH_MAX_LEN];
    u64 goid;
    u64 cur_thread;
};

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, void *);
    __type(value, struct http_request_t);
    __uint(max_entries, MAX_CONCURRENT);
} http_events SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
} events SEC(".maps");

// Injected in init
volatile const u64 method_ptr_pos;
volatile const u64 url_ptr_pos;
volatile const u64 path_ptr_pos;
volatile const u64 ctx_ptr_pos;

// This instrumentation attaches uprobe to the following function:
// func (mux *ServeMux) ServeHTTP(w ResponseWriter, r *Request)
SEC("uprobe/GorillaMux_ServeHTTP")
int uprobe_GorillaMux_ServeHTTP(struct pt_regs *ctx) {
    u64 request_pos = 4;
    struct http_request_t httpReq = {};
    httpReq.start_time = bpf_ktime_get_ns();

    // Get request struct
    void *req_ptr = get_argument(ctx, request_pos);

    // Get method from request
    void *method_ptr = 0;
    bpf_probe_read(&method_ptr, sizeof(method_ptr), (void *)(req_ptr + method_ptr_pos));
    u64 method_len = 0;
    bpf_probe_read(&method_len, sizeof(method_len), (void *)(req_ptr + (method_ptr_pos + 8)));
    u64 method_size = sizeof(httpReq.method);
    method_size = method_size < method_len ? method_size : method_len;
    bpf_probe_read(&httpReq.method, method_size, method_ptr);

    // get path from Request.URL
    void *url_ptr = 0;
    bpf_probe_read(&url_ptr, sizeof(url_ptr), (void *)(req_ptr + url_ptr_pos));
    void *path_ptr = 0;
    bpf_probe_read(&path_ptr, sizeof(path_ptr), (void *)(url_ptr + path_ptr_pos));
    u64 path_len = 0;
    bpf_probe_read(&path_len, sizeof(path_len), (void *)(url_ptr + (path_ptr_pos + 8)));
    u64 path_size = sizeof(httpReq.path);
    path_size = path_size < path_len ? path_size : path_len;
    bpf_probe_read(&httpReq.path, path_size, path_ptr);

    // Get key
    void *req_ctx_ptr = 0;
    bpf_probe_read(&req_ctx_ptr, sizeof(req_ctx_ptr), (void *)(req_ptr + ctx_ptr_pos));
    void *key = get_consistent_key(ctx, (void *)(req_ptr + ctx_ptr_pos));

    httpReq.sc = generate_span_context();

    // Write event
    u64 cur_thread = bpf_get_current_pid_tgid();

    httpReq.goid = get_current_goroutine();
    httpReq.cur_thread = cur_thread;

    // send type 3 event
    struct gmap_t event3 = {};

    event3.key = httpReq.goid;
    event3.sc = httpReq.sc;
    event3.type = GOID_SC;
    event3.start_time = httpReq.start_time;

    bpf_perf_event_output(ctx, &gmap_events, BPF_F_CURRENT_CPU, &event3, sizeof(event3));

    bpf_map_update_elem(&http_events, &key, &httpReq, 0);
    start_tracking_span(req_ctx_ptr, &httpReq.sc);

//    static __always_inline void start_tracking_span(void *ctx, struct span_context *sc) {
//        bpf_map_update_elem(&tracked_spans, &ctx, sc, BPF_ANY);
//        bpf_map_update_elem(&tracked_spans_by_sc, sc, &ctx, BPF_ANY);
//    }

    return 0;
}

UPROBE_RETURN(GorillaMux_ServeHTTP, struct http_request_t, 4, ctx_ptr_pos, http_events, events)

//#define UPROBE_RETURN(name, event_type, ctx_struct_pos, ctx_struct_offset, uprobe_context_map, events_map) \
//SEC("uprobe/##name##")                                                                                     \
//int uprobe_##name##_Returns(struct pt_regs *ctx) {                                                         \
//    void *req_ptr = get_argument(ctx, ctx_struct_pos);                                                     \
//    void *key = get_consistent_key(ctx, (void *)(req_ptr + ctx_struct_offset));                            \
//    void *req_ptr_map = bpf_map_lookup_elem(&uprobe_context_map, &key);                                    \
//    event_type tmpReq = {};                                                                                \
//    bpf_probe_read(&tmpReq, sizeof(tmpReq), req_ptr_map);                                                  \
//    tmpReq.end_time = bpf_ktime_get_ns();                                                                  \
//    bpf_perf_event_output(ctx, &events_map, BPF_F_CURRENT_CPU, &tmpReq, sizeof(tmpReq));                   \
//    bpf_map_delete_elem(&uprobe_context_map, &key);                                                        \
//    stop_tracking_span(&tmpReq.sc);                                                                        \
//    return 0;                                                                                              \
//}