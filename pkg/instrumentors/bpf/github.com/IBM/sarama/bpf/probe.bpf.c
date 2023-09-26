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

#define TOPIC_MAX_LEN 100
#define KEY_MAX_LEN 20
#define VALUE_MAX_LEN 150
#define MAX_CONCURRENT 50

struct publisher_message_t
{
    BASE_SPAN_PROPERTIES
    char topic[TOPIC_MAX_LEN];
    char key[KEY_MAX_LEN];
    char value[VALUE_MAX_LEN];
    u64 offset;
    u64 partition;
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

// Injected in init
volatile const u64 topic_ptr_pos;
//volatile const u64 key_ptr_pos;
//volatile const u64 value_ptr_pos;
//volatile const u64 offset_ptr_pos;
//volatile const u64 partition_ptr_pos;

// This instrumentation attachs uprobe to the following function:
// func (sp *syncProducer) SendMessage(msg *ProducerMessage) (partition int32, offset int64, err error)
SEC("uprobe/syncProducer_SendMessage")
int uprobe_syncProducer_SendMessage(struct pt_regs *ctx)
{
    u64 msg_ptr_pos = 6;
    // 6 -> value
    // 6 new -> header ?

    struct publisher_message_t req = {};
    req.start_time = bpf_ktime_get_ns();

    // get message struct pointer
    void *msg_ptr = get_argument(ctx, msg_ptr_pos);

    // extract information from struct
//    u64 topic_ptr_pos = 56;
//    u64 topic_len = 0;
//    bpf_probe_read(&topic_len, sizeof(topic_len), (void *)(msg_ptr + (topic_ptr_pos + 8)));

    u64 topic_len = 0;

    bpf_probe_read(&topic_len, sizeof(topic_len), (void *)(msg_ptr + 0));
    bpf_printk("Show topic length 0 %d\n", topic_len);

    bpf_probe_read(&topic_len, sizeof(topic_len), (void *)(msg_ptr + 8));
    bpf_printk("Show topic length 1 %d\n", topic_len);

    bpf_probe_read(&topic_len, sizeof(topic_len), (void *)(msg_ptr + 16));
    bpf_printk("Show topic length 2 %d\n", topic_len);

    bpf_probe_read(&topic_len, sizeof(topic_len), (void *)(msg_ptr + 24));
    bpf_printk("Show topic length 3 %d\n", topic_len);

    bpf_probe_read(&topic_len, sizeof(topic_len), (void *)(msg_ptr + 32));
    bpf_printk("Show topic length 4 %d\n", topic_len);

    bpf_probe_read(&topic_len, sizeof(topic_len), (void *)(msg_ptr + 40));
    bpf_printk("Show topic length 5 %d\n", topic_len);

    bpf_probe_read(&topic_len, sizeof(topic_len), (void *)(msg_ptr + 48));
    bpf_printk("Show topic length 6 %d\n", topic_len);

    bpf_probe_read(&topic_len, sizeof(topic_len), (void *)(msg_ptr + 56));
    bpf_printk("Show topic length 7 %d\n", topic_len);

    bpf_probe_read(&topic_len, sizeof(topic_len), (void *)(msg_ptr + 64));
    bpf_printk("Show topic length 8 %d\n", topic_len);

    bpf_probe_read(&topic_len, sizeof(topic_len), (void *)(msg_ptr + 72));
    bpf_printk("Show topic length 9 %d\n", topic_len);

    bpf_probe_read(&topic_len, sizeof(topic_len), (void *)(msg_ptr + 80));
    bpf_printk("Show topic length 10 %d\n", topic_len);

    bpf_probe_read(&topic_len, sizeof(topic_len), (void *)(msg_ptr + 88));
    bpf_printk("Show topic length 11 %d\n", topic_len);

    bpf_probe_read(&topic_len, sizeof(topic_len), (void *)(msg_ptr + 96));
    bpf_printk("Show topic length 12 %d\n", topic_len);

     bpf_probe_read(&topic_len, sizeof(topic_len), (void *)(msg_ptr + 13 * 8));
        bpf_printk("Show topic length 0 %d\n", topic_len);

        bpf_probe_read(&topic_len, sizeof(topic_len), (void *)(msg_ptr + 14 * 8));
        bpf_printk("Show topic length 1 %d\n", topic_len);

        bpf_probe_read(&topic_len, sizeof(topic_len), (void *)(msg_ptr + 15 * 8));
        bpf_printk("Show topic length 2 %d\n", topic_len);

        bpf_probe_read(&topic_len, sizeof(topic_len), (void *)(msg_ptr + 16 * 8));
        bpf_printk("Show topic length 3 %d\n", topic_len);

        bpf_probe_read(&topic_len, sizeof(topic_len), (void *)(msg_ptr + 17 * 8));
        bpf_printk("Show topic length 4 %d\n", topic_len);

        bpf_probe_read(&topic_len, sizeof(topic_len), (void *)(msg_ptr + 18 * 8));
        bpf_printk("Show topic length 5 %d\n", topic_len);

        bpf_probe_read(&topic_len, sizeof(topic_len), (void *)(msg_ptr + 19 * 8));
        bpf_printk("Show topic length 6 %d\n", topic_len);

        bpf_probe_read(&topic_len, sizeof(topic_len), (void *)(msg_ptr + 20 * 8));
        bpf_printk("Show topic length 7 %d\n", topic_len);

        bpf_probe_read(&topic_len, sizeof(topic_len), (void *)(msg_ptr + 21 * 8));
        bpf_printk("Show topic length 8 %d\n", topic_len);

        bpf_probe_read(&topic_len, sizeof(topic_len), (void *)(msg_ptr + 22 * 8));
        bpf_printk("Show topic length 9 %d\n", topic_len);

//    void *topic_ptr = 0;
//    long res;
//
//    bpf_probe_read(&topic_ptr, sizeof(topic_ptr), (void *)(msg_ptr + topic_ptr_pos));
//
//    bpf_printk("Show topic length %d\n", topic_len);
//
//    topic_len = topic_len > TOPIC_MAX_LEN ? TOPIC_MAX_LEN : topic_len;
//    res = bpf_probe_read(&req.topic, topic_len, topic_ptr);
//
//    bpf_trace_printk(req.topic, sizeof(req.topic));
    return 0;
}