#include "bpf_helpers.h"
#include "span_context.h"

#define MAX_SYSTEM_THREADS 30
#define MAX_DEPTH 16

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__type(key, u64);
	__type(value, u64);
	__uint(max_entries, MAX_SYSTEM_THREADS);
	__uint(pinning, LIBBPF_PIN_BY_NAME);
} goroutines_map SEC(".maps");


u64 get_current_goroutine() {
    u64 current_thread = bpf_get_current_pid_tgid();
    void* goid_ptr = bpf_map_lookup_elem(&goroutines_map, &current_thread);
    u64 goid;
    bpf_probe_read(&goid, sizeof(goid), goid_ptr);
    return goid;
}