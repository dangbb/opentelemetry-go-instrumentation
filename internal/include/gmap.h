#include "bpf_helpers.h"

#define MAX_SIZE 1024
#define GOPC_PGOID 1
#define GOID_GOPC 2
#define GOID_SC 3

struct gmap_t {
    u64 key;
    u64 value;
    struct span_context sc;
    u64 type;
};

// holder for gmap_t
struct
{
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, void *);
    __type(value, struct gmap_t);
    __uint(max_entries, 1);
} placeholder_map SEC(".maps");

// Map sending back to Golang backend
struct {
    __uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
} gmap_events SEC(".maps");