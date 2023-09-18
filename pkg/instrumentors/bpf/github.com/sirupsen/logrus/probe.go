//Ngo Hai Dang (Dangbb)'s thesis contribution:
//- Implement eBPF instrumentation for logrus library.

package logrus


//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64,arm64 -cc clang -cflags $CFLAGS bpf ./bpf/probe.bpf.c

const instrumentedPkg = "sirupsen/logrus"