#include "bpf_helpers.h"
#include "span_context.h"

#define MAX_SYSTEM_THREADS 20

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__type(key, u64);
	__type(value, u64);
	__uint(max_entries, MAX_SYSTEM_THREADS);
	__uint(pinning, LIBBPF_PIN_BY_NAME);
} goroutines_map SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
	__type(key, u64);
	__type(value, struct span_context);
	__uint(max_entries, MAX_SYSTEM_THREADS);
	__uint(pinning, LIBBPF_PIN_BY_NAME);
} sc_map SEC(".maps");


u64 get_current_goroutine() {
    u64 current_thread = bpf_get_current_pid_tgid();
    void* goid_ptr = bpf_map_lookup_elem(&goroutines_map, &current_thread);
    u64 goid;
    bpf_probe_read(&goid, sizeof(goid), goid_ptr);
    return goid;
}

// check if sc exist
int is_sc_exist() {
    u64 go_id = get_current_goroutine();

    void *sc_ptr = bpf_map_lookup_elem(&sc_map, &go_id);

    bpf_printk("Check sc exist for goid %d - result %d", go_id, (sc_ptr == NULL) ? 0 : 1);

    return (sc_ptr == NULL) ? 0 : 1;
}

// get sc record
static __always_inline struct span_context *get_sc() {
    u64 go_id = get_current_goroutine();

    bpf_printk("Get sc for %d", go_id);

    return bpf_map_lookup_elem(&sc_map, &go_id);
}

// delete sc record
void delete_sc() {
    u64 go_id = get_current_goroutine();

    bpf_map_delete_elem(&sc_map, &go_id);

    bpf_printk("Remove correlation on go_id: %d", go_id);
}