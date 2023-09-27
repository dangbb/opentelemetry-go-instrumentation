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
volatile const u64 key_ptr_pos;
volatile const u64 value_ptr_pos;
volatile const u64 offset_ptr_pos;
volatile const u64 partition_ptr_pos;

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

    bpf_trace_printk(req.topic, sizeof(req.topic));

    // extract key
    u64 key_len = 0;
    bpf_probe_read(&key_len, sizeof(key_len), (void *)(msg_ptr + (key_ptr_pos + 8)));
    key_len = key_len > KEY_MAX_LEN ? KEY_MAX_LEN : key_len;

    void *key_ptr = 0;
    bpf_probe_read(&key_ptr, sizeof(key_ptr), (void *)(msg_ptr + key_ptr_pos));
    bpf_probe_read(&req.key, key_len, key_ptr);

    bpf_trace_printk(req.key, sizeof(req.key));

    // extract value
    u64 value_len = 0;
    bpf_probe_read(&value_len, sizeof(value_len), (void *)(msg_ptr + (value_ptr_pos + 8)));
    value_len = value_len > VALUE_MAX_LEN ? VALUE_MAX_LEN : value_len;

    void *value_ptr = 0;
    bpf_probe_read(&value_ptr, sizeof(value_ptr), (void *)(msg_ptr + value_ptr_pos));
    bpf_probe_read(&req.value, value_len, value_ptr);

    bpf_trace_printk(req.value, sizeof(req.value));

    // extract offset
    bpf_printk("Offset at : %d", offset_ptr_pos);
    bpf_probe_read(&req.offset, sizeof(req.offset), (void *)(msg_ptr + offset_ptr_pos));
    bpf_printk("Offset: %d", req.offset);

    // extract partition
    bpf_printk("Partition at : %d", partition_ptr_pos);
    bpf_probe_read(&req.partition, sizeof(req.partition), (void *)(msg_ptr + partition_ptr_pos));
    bpf_printk("Partition: %d", req.partition);

    return 0;
}