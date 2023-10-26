#include "bpf_helpers.h"

#define GOPC_PGOID 1
#define GOID_GOPC 2
#define GOID_SC 3

struct gmap_t {
    u64 key;
    u64 value;
    struct span_context sc;
    u64 type;
    u64 start_time;
};

// Map sending back to Golang backend
struct {
    __uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
} gmap_events SEC(".maps");