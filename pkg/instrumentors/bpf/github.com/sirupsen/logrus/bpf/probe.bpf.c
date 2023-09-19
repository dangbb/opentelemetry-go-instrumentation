// Import necessary libraries
#include "arguments.h"
#include "span_context.h"
#include "go_context.h"
#include "uprobe.h"

#define MAX_LOG_SIZE 100
#define MAX_CONCURRENT 50

// Define event struct
struct log_event_t {
    BASE_SPAN_PROPERTIES
    u64 level;
    char log[MAX_LOG_SIZE];
};

struct {
    __uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
} events SEC(".maps"); // handle checkpoint at the return of function

// Define main function to extract information
// Attach probe at function:
// `func (entry *Entry) log(level Level, msg string) {...}`
// We would want to extract `level` and `msg`, so the offset should be
// 2, 3 and 4. (2 for level, 3 for string content and 4 for string length).
// 1 is pointer to entry.
SEC("uprobe/Logrus_EntryLog")
int uprobe_Logrus_EntryLog(struct pt_regs *ctx) { // take list of register and stack as input
    u64 level_pos = 2;
    u64 str_ptr_pos = 3;
    u64 str_len_pos = 4;

    struct log_event_t logEvent = {};
    logEvent.start_time = bpf_ktime_get_ns();

    // get level position
    logEvent.level = (u64)get_argument(ctx, level_pos);

    // get string length and string content
    void *str_ptr = get_argument(ctx, str_ptr_pos);
    u64 str_len = (u64)get_argument(ctx, str_len_pos);
    u64 str_size = MAX_LOG_SIZE < str_len ? MAX_LOG_SIZE : str_len;
    bpf_probe_read(logEvent.log, str_size, str_ptr);

    // set span context
    logEvent.sc = generate_span_context();

    // add to perf map
    // BPF_F_CURRENT_CPU flaf option ?
    bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, &logEvent, sizeof(logEvent));
    return 0;
};