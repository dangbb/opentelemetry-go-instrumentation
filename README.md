# OpenTelemetry Go Automatic Instrumentation

This repository provides [OpenTelemetry] instrumentation for [Go] libraries using [eBPF].

## Project Status

:construction: This project is currently work in progress.

### Compatibility

OpenTelemetry Go Automatic Instrumentation is compatible with all current supported versions of the [Go language](https://golang.org/doc/devel/release#policy).

> Each major Go release is supported until there are two newer major releases.
> For example, Go 1.5 was supported until the Go 1.7 release, and Go 1.6 was supported until the Go 1.8 release.

For versions of Go that are no longer supported upstream, this repository will stop ensuring compatibility with these versions in the following manner:

- A minor release will be made to add support for the new supported release of Go.
- The following minor release will remove compatibility testing for the oldest (now archived upstream) version of Go.
   This, and future, releases may include features only supported by the currently supported versions of Go.

Currently, OpenTelemetry Go Automatic Instrumentation is tested for the following environments.

| OS      | Go Version | Architecture |
| ------- | ---------- | ------------ |
| Ubuntu  | 1.21       | amd64        |
| Ubuntu  | 1.20       | amd64        |

Automatic instrumentation should work on any Linux kernel above 4.4.

OpenTelemetry Go Automatic Instrumentation supports the arm64 architecture.
However, there is no automated testing for this platform.
Be sure to validate support on your own ARM based system.

Users of non-Linux operating systems can use
[the Docker images](https://github.com/open-telemetry/opentelemetry-go-instrumentation/pkgs/container/opentelemetry-go-instrumentation%2Fautoinstrumentation-go)
or create a virtual machine to compile and run OpenTelemetry Go Automatic Instrumentation.

## Contributing

See the [contributing documentation](./CONTRIBUTING.md).

## Dangnh's Thesis contributing 

### Research about current topic 

Environment variables:

- OTEL_EXPORTER_OTLP_ENDPOINT=http://jaeger:4317
- OTEL_GO_AUTO_TARGET_EXE=/app/main
- OTEL_GO_AUTO_INCLUDE_DB_STATEMENT=true
- OTEL_SERVICE_NAME=httpPlusdb
- OTEL_PROPAGATORS=tracecontext,baggage
- CGO_ENABLED=1

Flow performing auto instrumentation:

1. Read path to executable. Define `target`
2. Create an instance of process analyzer, which support
   - Discover process ID: read list of process via `/proc` folder
   - For each process, read it content and find `cmdLine`
3. Create instance of OTel controller
  - Initialize Open Telemetry connection with env variable configuration.
    - Define service name 
    - Define exporter and provider
    - Define method to send eBPF trace to OTLP endpoint, which receive an event object and send to OTLP. Send the whole value of event into context (`Trace` method).
    - Define method to get timestamp information root. 
4. Create instance of instrumentor manager
  - Contains list of predefined method (with corresponding lib), which can be used to place probe.
  - Register all supported method and lib into
  - Create new allocator: Handle allocation of BPF file-system. `bpffs` list will mount this allocator.
  - In `load` method, create new injector, which take target allocation detail and offset read from `offset-tracker` and parse into a json file.
5. Discover process id of target 
6. Process analyzer analyze the pid with list of supported method.
  - Analyze golang version, dependency library, and total function found
  - Return an instrumentation target details. Contains:
    - List of functions that auto instrumentation support. The result after register process is being store inside a map.
      - `Instrumentors`: List of instrumentor supported for specific library, which is predefined.
    - `Run` method: Perform injection process
      1. Load target: Process of inject eBPF into process.
         - Create a new injector instance based on target details.
         - Open executable to instrumented process.
         - Load the eBPF file system. Define and mount folder for list of eBPF file.
         - Load instrumentors.
           - Inject eBPF into source code
           - Load eBPF object into kernel. (eBPF library support an interface to modify value of program loaded to kernel)
         - Instrumentor register probe into specific function name. 
           - `Load` method: 
             - Inject perform inject, using loadBpf method and list of struct field for each field which should be monitored. This method return a `spec` object.
             - `Spec` object creation flow:
               - Perform read from file `bpf_bpfel_x86.o`
               - Using method from `ebpf` to extract spec from reader.
               - Iterate over list of field (each library has a predefined list of field). For each, find offset of field. Offset is store as value, with key is var name, in map called `injectedVars`.
               - First add common Injection, with common allocation.
                 - Add some mapping with some common field.
               - Second, add config injection, with those fields from above.
                 - Add map view from value of those extract above sub-step.
               - After that, performs rewrite constants with parameter is the value of map.
                 - This step is being handle by external library. The operation is performed with `spec` as receiver.
               - `LoadAndAssign` method which load value of spec and parse to interface. Load map and programs into kernel and assign them to a struct.
               - Mapping using tag `ebpf` and type ebpf.Program/Map (type inside struct field).
                 - In this struct, currently store list of map for different purpose.
                 - Also, This contains program, which store here after being loaded into kernel.
             - For each instrumentor, a new BPF Object is created and used to store eBPF object load from spec. With some collection options.
               - Loading map into kernel, using pin path, as base path to pin Map.
               - After that, object will have properties Events which is an eBPF map and sent event back to base. Value is extracted from `spec` object.
               - Probe is then registed.
      2. Run instrumentor for incoming event. Input is an event channel, send event to OTel controller.
         - Instrumentor performs read from event reader and extract eBPF event for specific struct.
    - Allocation detail: ???
    - 
7. Instrumentation manager will filter out unused instrumentors inside list of target from step 6 (e.g. if lib net/http not foudn, remove it from instrumentor manager)
8. Instrumentor manager perform running based on target details.
9. Process of event after being extracted by bBPF program. Event contains following properties:
    - Method
    - Path
    - StartTime
    - EndTime
    - SpanContext
    - ParentSpanContext
      - For span context, each contains 2 field traceId and spanId.
    - The process of extracting event includes following:
   1. `eventReader` get event from eBPF program (???). 
      - eventReader is a pref.Reader instance.
   2. Record then being parsed into event object.
   3. Send back to channel.
    
    How to extract event from object from eBPF program.
    - Do eBPF lib able to read everything (even context, or arbitrary object) ?
    - Ability to modify context (?) - no, not realy. It can only modify by substitution lost of byte with another list of bytes.
    - Read object defined by library - Has to defined it in *.c file.
    Understand the flow:
    - Include list of C header file.
    - Define list of constants to replace in runtime.
    Understand the C instrumentation code.
    - 

### Disadvantages

The main question is, can we generalize it ?

- For each request and process, the span will be appended to the end of the list, instead of the same grade. This is because we cant modify object. 
- Can we integrate with exist trace propagation flow? 
  - Injection: Change value of a specific field for storing trace context. That library should support open telemetry to accomplish with this task.
  - Extraction: 

### eBPF which is related to topic

Link: [uprobe](https://github.com/iovisor/bpftrace/blob/master/docs/reference_guide.md#3-uprobeuretprobe-dynamic-tracing-user-level)

Allow attach probes to user space program's function.
- Allow attack via function name.
- Allow attach via address, in case of stripped binary.
- Allow add offset to function name to create checkpoint. The only thing to keep in mind is should align with instruction boundaries. E.g if instruction is 4 bytes, then an address like 'main+1' won't work.

### Writing eBPF C code 

Link: [build eBPF program](https://dev.to/pemcconnell/building-an-xdp-ebpf-program-with-c-and-golang-a-step-by-step-guide-4hoa)
-

### Research about cilium/ebpf - How it works ? 

- Using bootstrap-life to generate bpf program inline. E.g:
  - `tracepoint.c` corresponding for kernel state of ebpf program.
  - `bpf_bpfel.go` for user state after generated. 
  - `bpf_bpfel.o` for user state after generated.
- `bpf_bpfel.o` is eBPF Linux byte code.
- Source code a.k.a *.c file, is compiled by bpf2go into *.o file, then it generated *.go file based on  
- Byte code of ebpf program is embedded into *.go file.

- How it perform substitition ? Can we generalized it into some common usecase ? - Should be able to understand how substitition work.

### Working on improvement

List of tasks: https://app.diagrams.net/#G1SlL7WR4KKabT_eNkqCcU4cSg3jpgSI-5 



## License

OpenTelemetry Go Automatic Instrumentation is licensed under the terms of the [Apache Software License version 2.0].
See the [license file](./LICENSE) for more details.

Third-party licenses and copyright notices can be found in the [LICENSES directory](./LICENSES).

[OpenTelemetry]: https://opentelemetry.io/
[Go]: https://go.dev/
[eBPF]: https://ebpf.io/
[Apache Software License version 2.0]: https://www.apache.org/licenses/LICENSE-2.0
