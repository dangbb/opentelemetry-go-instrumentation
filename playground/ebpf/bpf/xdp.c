#include <linux/bpf.h>
#include <bpf_helpers.h>

// define data structure and map for perf event
struct perf_trace_event {
    __u64 timestamps;
    __u32 processing_time_ns;
    __u8 type;
};

#define TYPE_ENTER 1
#define TYPE_DROP 2
#define TYPE_PASS 3

// define metadata for map to store perf event.
struct {
    __uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY); // bpf_map_type: perf buffer for event array
    __uint(key_size, sizeof(int)); // key type int
    __uint(value_size, sizeof(struct perf_trace_event)); // value same size as define structure
    __uint(max_entries, 1024); // size of array
} output_map SEC(".maps");

/**
SEC: declare structure for BTF. Put the given object to ELF section.
allow libbpf parse metadata and find/create map before program can use them.
This example above show how to declare and array in BPF program.

Tell kernel that we're going to declare an map, with key type int
value type struct, and number of elements is 1024.

1.
struct bpf_map_def SEC(".maps") map_parsing_context = {
...
};

Parse relevant metadata and create map before program can used.
Allow parse data from ELF option. Extract metadata for the map and metadata.

2.
struct {
...
} map_keys SEC(".maps");

Put additional metadata into .map ELF section. Declare the structure of the map.
`bpftool map dump` would show structure of the map.

Allow parse key and value fields.

"maps" is still allow, but ".maps" is right way to do it. Everything (metadata and structure layout) will go to ".maps"
section.
**/
SEC("xdp") // declare this object in new section `xdp`
int xdp_test(struct xdp_md *ctx) {
    struct perf_trace_event e = {};

    // perf event for entering xdp program
    // function from <linux/bpf.h>
    e.timestamps = bpf_ktime_get_ns();
    e.type = TYPE_ENTER;
    e.processing_time_ns = 0;
    // from <linux/bpf.h> definition
    bpf_perf_event_output(ctx, &output_map, BPF_F_CURRENT_CPU, &e, sizeof(e));

    if (bpf_get_prandom_u32() % 2 == 0) {
        e.type = TYPE_DROP;
        __u64 ts = bpf_ktime_get_ns();
        e.processing_time_ns = ts - e.timestamps;
        e.timestamps = ts;
        bpf_perf_event_output(ctx, &output_map, BPF_F_CURRENT_CPU, &e, sizeof(e));
        // xdp action, import from <linux/bpf.h>
        return XDP_DROP;
    }

    e.type =  TYPE_PASS;
    __u64 ts = bpf_ktime_get_ns();
    e.processing_time_ns = ts - e.timestamps;
    e.timestamps = ts;
    bpf_perf_event_output(ctx, &output_map, BPF_F_CURRENT_CPU, &e, sizeof(e));
    // Only when type was 'BPF_MAP_TYPE_PERF_EVENT_ARRAY'
    // write raw perf event action, import from <linux/bpf.h>
    return XDP_PASS;
}




