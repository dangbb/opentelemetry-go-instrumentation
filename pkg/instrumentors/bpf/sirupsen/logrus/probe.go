/**
=====================================================
Ngo Hai Dang (Dangbb)'s thesis contribution:
- Implement eBPF instrumentation for logrus library.
=====================================================
*/

package logrus

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64,arm64 -cc clang -cflags $CFLAGS bpf ./bpf/probe.bpf.c

const instrumentedPkg = "github.com/sirupsen/logrus"

/**
=====================================================
Goal:
1. Phân tích `github.com/sirupsen/logrus` và chọn hàm phù hợp để đặt các điểm theo dõi (probe).
Với mỗi hàm phù hợp, chọn các thông tin thích hợp và tạo object lấy và gửi thông tin đó.
2. Tạo chương trình đơn giản cho phép lấy các thông tin từ logrus trong file binary.
3. Tích hợp với luồng hiện tại của repo.
4. Kiểm thử.
=====================================================
*/
