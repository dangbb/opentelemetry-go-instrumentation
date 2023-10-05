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

char __license[] SEC("license") = "Dual MIT/GPL";

#define TOPIC_MAX_LEN 30
#define KEY_MAX_LEN 20
#define VALUE_MAX_LEN 100
#define MAX_CONCURRENT 5
#define MAGIC_NUMBER 24
#define MAX_HEADER_LEN 25

struct publisher_message_t
{
    BASE_SPAN_PROPERTIES
    char topic[TOPIC_MAX_LEN];
    char key[KEY_MAX_LEN];
    char value[VALUE_MAX_LEN];
    u64 goid;

//    char header_1[MAX_HEADER_LEN];
//    char value_1[MAX_HEADER_LEN];

//    char header_2[MAX_HEADER_LEN];
//    char value_2[MAX_HEADER_LEN];

//    char header_3[MAX_HEADER_LEN]; -- reduce size to use span context and tracked span;
//    char value_3[MAX_HEADER_LEN];
};

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, void *);
    __type(value, struct publisher_message_t);
    __uint(max_entries, MAX_CONCURRENT);
} publisher_message_events SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
} events SEC(".maps");

const struct publisher_message_t *unused __attribute__((unused));

// Injected in init
volatile const u64 topic_ptr_pos;
volatile const u64 key_ptr_pos;
volatile const u64 value_ptr_pos;
volatile const u64 headers_arr_ptr_pos;

// This instrumentation attachs uprobe to the following function:
// func (sp *syncProducer) SendMessage(msg *ProducerMessage) (partition int32, offset int64, err error)
SEC("uprobe/syncProducer_SendMessage")
int uprobe_syncProducer_SendMessage(struct pt_regs *ctx)
{
    u64 msg_ptr_pos = 2;

    struct publisher_message_t req = {};
    req.start_time = bpf_ktime_get_ns();

    // get message struct pointer
    void *msg_ptr = get_argument(ctx, msg_ptr_pos);

    // extract topic
    u64 topic_len = 0;
    bpf_probe_read(&topic_len, sizeof(topic_len), (void *)(msg_ptr + (topic_ptr_pos + 8)));
    topic_len = topic_len > TOPIC_MAX_LEN ? TOPIC_MAX_LEN : topic_len;

    void *topic_ptr = 0;
    bpf_probe_read(&topic_ptr, sizeof(topic_ptr), (void *)(msg_ptr + topic_ptr_pos));
    bpf_probe_read(&req.topic, topic_len, topic_ptr);

    // extract key
    void *key_ptr_ptr = 0;
    bpf_probe_read(&key_ptr_ptr, sizeof(key_ptr_ptr), (void *)(msg_ptr + key_ptr_pos));

    void *key_ptr = 0;
    bpf_probe_read(&key_ptr, sizeof(key_ptr), key_ptr_ptr);

    u64 key_len = 0;
    bpf_probe_read(&key_len, sizeof(key_len), (void *)(key_ptr_ptr + 8));
    key_len = key_len > KEY_MAX_LEN ? KEY_MAX_LEN : key_len;
    bpf_probe_read(&req.key, key_len, key_ptr);

    // extract value
    void *value_ptr_ptr = 0;
    bpf_probe_read(&value_ptr_ptr, sizeof(value_ptr_ptr), (void *)(msg_ptr + value_ptr_pos));

    void *value_ptr = 0;
    bpf_probe_read(&value_ptr, sizeof(value_ptr), value_ptr_ptr);

    u64 value_len = 0;
    bpf_probe_read(&value_len, sizeof(value_len), (void *)(value_ptr_ptr + 8));
    value_len = value_len > VALUE_MAX_LEN ? VALUE_MAX_LEN : value_len;
    bpf_probe_read(&req.value, value_len, value_ptr);

//    // extract header length
//    u64 headers_len = 0;
//
//    bpf_probe_read(&headers_len, sizeof(headers_len), (void *)(msg_ptr + (headers_arr_ptr_pos + 8)));
//    bpf_printk("Header count: %d", headers_len);
//
//    if (headers_len > 0) {
//        void *header_arr_ptr = 0;
//        bpf_probe_read(&header_arr_ptr, sizeof(header_arr_ptr), (void *)(msg_ptr + headers_arr_ptr_pos));
//
//        void *header_key_ptr = 0;
//        u64 header_key_len = 0;

//        // 1st header key
//        bpf_probe_read(&header_key_ptr, sizeof(header_key_ptr), (void *)(header_arr_ptr + (0 * 24)));
//        bpf_probe_read(&header_key_len, sizeof(header_key_len), (void *)(header_arr_ptr + (0 * 24 + 8)));
//        bpf_printk("Header key len: %d", header_key_len);
//        header_key_len = header_key_len > MAX_HEADER_LEN ? MAX_HEADER_LEN : header_key_len;
//
//        bpf_probe_read(&req.header_1, header_key_len, header_key_ptr);
//        bpf_trace_printk(req.header_1, sizeof(req.header_1));
//
//        // 1st header value
//        bpf_probe_read(&header_key_ptr, sizeof(header_key_ptr), (void *)(header_arr_ptr + (1 * 24)));
//        bpf_probe_read(&header_key_len, sizeof(header_key_len), (void *)(header_arr_ptr + (1 * 24 + 8)));
//        bpf_printk("Header key len: %d", header_key_len);
//        header_key_len = header_key_len > MAX_HEADER_LEN ? MAX_HEADER_LEN : header_key_len;
//
//        bpf_probe_read(&req.value_1, header_key_len, header_key_ptr);
//        bpf_trace_printk(req.value_1, sizeof(req.value_1));
//
//        if (headers_len > 1) {
//            // 2nd header key
//            bpf_probe_read(&header_key_ptr, sizeof(header_key_ptr), (void *)(header_arr_ptr + (2 * 24)));
//            bpf_probe_read(&header_key_len, sizeof(header_key_len), (void *)(header_arr_ptr + (2 * 24 + 8)));
//            bpf_printk("Header key len: %d", header_key_len);
//            header_key_len = header_key_len > MAX_HEADER_LEN ? MAX_HEADER_LEN : header_key_len;
//
//            bpf_probe_read(&req.header_2, header_key_len, header_key_ptr);
//            bpf_trace_printk(req.header_2, sizeof(req.header_2));
//
//            // 2nd header value
//            bpf_probe_read(&header_key_ptr, sizeof(header_key_ptr), (void *)(header_arr_ptr + (3 * 24)));
//            bpf_probe_read(&header_key_len, sizeof(header_key_len), (void *)(header_arr_ptr + (3 * 24 + 8)));
//            bpf_printk("Header key len: %d", header_key_len);
//            header_key_len = header_key_len > MAX_HEADER_LEN ? MAX_HEADER_LEN : header_key_len;
//
//            bpf_probe_read(&req.value_2, header_key_len, header_key_ptr);
//            bpf_trace_printk(req.value_2, sizeof(req.value_2));
//        }
//
//        if (headers_len > 2) {
//            // 3rd header key
//            bpf_probe_read(&header_key_ptr, sizeof(header_key_ptr), (void *)(header_arr_ptr + (2 * 24)));
//            bpf_probe_read(&header_key_len, sizeof(header_key_len), (void *)(header_arr_ptr + (2 * 24 + 8)));
//            bpf_printk("Header key len: %d", header_key_len);
//            header_key_len = header_key_len > MAX_HEADER_LEN ? MAX_HEADER_LEN : header_key_len;
//
//            bpf_probe_read(&req.header_3, header_key_len, header_key_ptr);
//            bpf_trace_printk(req.header_3, sizeof(req.header_3));
//
//            // 3rd header value
//            bpf_probe_read(&header_key_ptr, sizeof(header_key_ptr), (void *)(header_arr_ptr + (3 * 24)));
//            bpf_probe_read(&header_key_len, sizeof(header_key_len), (void *)(header_arr_ptr + (3 * 24 + 8)));
//            bpf_printk("Header key len: %d", header_key_len);
//            header_key_len = header_key_len > MAX_HEADER_LEN ? MAX_HEADER_LEN : header_key_len;
//
//            bpf_probe_read(&req.value_3, header_key_len, header_key_ptr);
//            bpf_trace_printk(req.value_3, sizeof(req.value_3));
//        }
//    }

    // extract key (address of msg)
    void *key = get_consistent_key(ctx, msg_ptr);
    u64 key64 = (u64)key;

    void *sc_ptr = get_sc();

    if (sc_ptr != NULL) {
        // generate spanID, copy traceID
        void *psc_ptr = get_sc();
        bpf_probe_read(&req.psc, sizeof(req.psc), psc_ptr);

        copy_byte_arrays(req.psc.TraceID, req.sc.TraceID, TRACE_ID_SIZE);
        generate_random_bytes(req.sc.SpanID, SPAN_ID_SIZE);

        bpf_printk("Sarama create new sc from exist psc");
    } else {
        // generate new sc
        req.sc = generate_span_context();

        req.trace_root = 1;
        // Set kv for sc span
        u64 go_id = get_current_goroutine();

        // Only create new
        u32 status = bpf_map_update_elem(&sc_map, &go_id, &req.sc, 0);

        if (status == 0) {
            bpf_printk("sarama - create correlation success go_id %d", go_id);

            void *new_sc_ptr = get_sc();
            bpf_printk("sarama - After create, test exist goid %d - result %d", go_id, (new_sc_ptr == NULL) ? 0 : 1);
        } else {
            bpf_printk("sarama - create correlation fail go_id %d", go_id);
        }
    }

    bpf_map_update_elem(&publisher_message_events, &key, &req, 0);
    start_tracking_span(msg_ptr, &req.sc);

//    // send back to instrumentor -- disable sending event back to perf channel
//    bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, &req, sizeof(req));
    return 0;
}

SEC("uprobe/syncProducer_SendMessage")
int uprobe_syncProducer_SendMessage_Returns(struct pt_regs *ctx) {
    void *req_ptr = get_argument(ctx, 2);         // extract message
    void *key = get_consistent_key(ctx, req_ptr); // get consistent key as value of req itself
    void *req_ptr_map = bpf_map_lookup_elem(&publisher_message_events, &key);

    u64 key64 = (u64)key;

    struct publisher_message_t tmpReq = {};
    bpf_probe_read(&tmpReq, sizeof(tmpReq), req_ptr_map);
    tmpReq.end_time = bpf_ktime_get_ns();

    tmpReq.goid = get_current_goroutine();

    bpf_printk("Sarama current goroutine: %d", get_current_goroutine());

    bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, &tmpReq, sizeof(tmpReq));
    bpf_map_delete_elem(&publisher_message_events, &key);
    stop_tracking_span(&tmpReq.sc);

    if (tmpReq.trace_root > 0) {
        delete_sc();
    }

    return 0;
}