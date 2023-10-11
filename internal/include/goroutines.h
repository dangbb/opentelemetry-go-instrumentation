#include "bpf_helpers.h"
#include "span_context.h"

#define MAX_SYSTEM_THREADS 30
#define MAX_DEPTH 16

#define GOID_PGOID 1
#define GOID_SC 2
#define THREAD_GOID 3

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__type(key, u64);
	__type(value, u64);
	__uint(max_entries, MAX_SYSTEM_THREADS);
	__uint(pinning, LIBBPF_PIN_BY_NAME);
} goroutines_map SEC(".maps");

// mapping between gopc and parent goroutine id
struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__type(key, u64);
	__type(value, u64);
	__uint(max_entries, MAX_SYSTEM_THREADS);
	__uint(pinning, LIBBPF_PIN_BY_NAME);
} gopc_to_pgoid SEC(".maps");

// mapping between current goid to pgoid
struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__type(key, u64);
	__type(value, u64);
	__uint(max_entries, MAX_SYSTEM_THREADS);
	__uint(pinning, LIBBPF_PIN_BY_NAME);
} p_goroutines_map SEC(".maps");


struct {
    __uint(type, BPF_MAP_TYPE_HASH);
	__type(key, u64);
	__type(value, struct span_context);
	__uint(max_entries, MAX_SYSTEM_THREADS);
	__uint(pinning, LIBBPF_PIN_BY_NAME);
} sc_map SEC(".maps");

// common map struct
struct thread_goid_mapping_t
{
    u64 key;
    u64 value;
    span_context sc_value;
    u64 type;
}

// event channel for mapping events
struct {
    __uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
} mapping_events SEC(".maps");


const struct thread_goid_mapping_t *unused __attribute__((unused));


u64 get_current_goroutine() {
    u64 current_thread = bpf_get_current_pid_tgid();
    void* goid_ptr = bpf_map_lookup_elem(&goroutines_map, &current_thread);
    u64 goid;
    bpf_probe_read(&goid, sizeof(goid), goid_ptr);
    return goid;
}

// get parent goroutine id.
static __always_inline u64 get_pgoid_from_gopc(u64 key) {
    void* goid_ptr = bpf_map_lookup_elem(&gopc_to_pgoid, &key);
    u64 goid = 0;
    bpf_probe_read(&goid, sizeof(goid), goid_ptr);
    return goid;
}

// check if sc exist
int is_sc_exist() {
    u64 go_id = get_current_goroutine();

    void *sc_ptr = bpf_map_lookup_elem(&sc_map, &go_id);

    return (sc_ptr == NULL) ? 0 : 1;
}

// get sc record
static __always_inline struct span_context *get_sc() {
    u64 go_id = get_current_goroutine();

    return bpf_map_lookup_elem(&sc_map, &go_id);
}

// delete sc record
void delete_sc() {
    u64 go_id = get_current_goroutine();

    bpf_map_delete_elem(&sc_map, &go_id);
}

// ancestor goroutine manipulation
// get sc of corresponding parent goroutine id for current goroutine id.
static __always_inline void* get_nearest_ancestor_sc() {
    u64 goid = get_current_goroutine();
    bpf_printk("Check ancestor of %d", goid);
    u64 cur = goid;
    u64 pgoid = 0;

    // travel through parent
    for (u64 i = 0; i < MAX_DEPTH; i++) {
        // extract parent goid
        void *pgoid_ptr = bpf_map_lookup_elem(&p_goroutines_map, &goid);
        if (pgoid_ptr == NULL) {
            bpf_printk("Not found ancestor for %d at %d", cur, goid);
            return NULL;
        }

        bpf_probe_read(&pgoid, sizeof(pgoid), pgoid_ptr);

        // try to extract sc for pgoid
        void *asc_ptr = bpf_map_lookup_elem(&sc_map, &pgoid);
        if (asc_ptr == NULL) {
            // not exist, continue to find
            goid = pgoid;
            continue;
        }

        bpf_printk("Found ancestor for %d: %d", cur, pgoid);

        // exist, return
        return asc_ptr;
    }
    // reach max depth, but can find. Return NULL.
    bpf_printk("Reach max depth for %d", cur);
    return NULL;
}