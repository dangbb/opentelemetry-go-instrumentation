Run command: 

```shell
go run -exec sudo ./playground/sarama --binary playground/sarama/publisher/main --method "github.com/IBM/sarama.(*syncProducer).SendMessage" --pid 14249
```

Show eBPF log:

```shell
sudo cat /sys/kernel/debug/tracing/trace_pipe
```

Add GDB checkpoint 
```shell
b github.com/IBM/sarama.(*syncProducer).SendMessage
b github.com/sirupsen/logrus.(*Entry).write

```

How Golang binary build and store array of type byte ?

- msg: `0xc0002a6000` 
-             from msg + 3 * 8
- Key: data - 0xc000012018, address point to `0xc000028180`, point to 0x65756c617679656b
-             extract "\200\201\002\300"      extract "keyvalue 2"
-                                             find length of key: 0xc000012018+8
-               from msg + 5 * 8
- Value: data - 0xc000012030, address point to `0xc000028183`, point to ...
-                                               extract value "value 2"
-                                               find length of key: 0xc000012030+8
-                from msg + 7 * 8
- Header: data - 0xc0001766f0, address point to `0xc0000c26f0`
-                                               contain value "header-key"
-                                               find length of header: 0xc0001766f0+8
- 
- 
- Header with 2 elements:
- Array at: `0xc000216fc0`, address point to `0xc000202490`
-                                            contain value of "header-key"
-                                            find length of header: 0xc000216fc0+8 (10)
- Array at: `0xc000216fc0+24`, address point to `0xc0002024a0`
-                                                contain value of "header-value"
-                                                find length of header:  0xc000216fc0+32 (12)
- Array at: `0xc000216fc0+48`, address point to `0xc0002024b0`
-                                                contain value of "header-key-2"
-                                                find length of header: 0xc000216fc0+56 (12)
- Array at: `0xc000216fc0+72`, address point to `0xc0002024c0`
-                                                contain value of "header-value-2"
-                                                find length of header: 0xc000216fc0+80 (14)

Open bpf trace pipe 

```shell
sudo cat /sys/kernel/debug/tracing/trace_pipe
```

Inspect function call goroutine (r14):
1. 0xc0000076c0        824633751232
2. 0xc0000076c0        824633751232 -> is identical

Inspect function call goroutine (r14), in case function is really in goroutin:
1. r14            0xc0001781a0        824635261344
2. r14            0xc0001781a0        824635261344

1. r14            0xc0001791e0        824635265504
2. r14            0xc0001791e0        824635265504

check what inside this address
1. 0xc000179520 -> 0xc0001b1000 

(gdb) x/a 0xc000179520
0xc000179520:   0xc0001b1000
(gdb) x/a 0xc0001b1000
0xc0001b1000:   0xc000072000
(gdb) x/a 0xc000072000
0xc000072000:   0xc000073000
(gdb) x/a 0xc000073000
0xc000073000:   0xc00006e000
(gdb) x/a 0xc00006e000
0xc00006e000:   0x0

-> Still identical -> maybe this only work with goroutine ?


For sarama, the sync process detach from main goroutine (not from the goroutine that handle the request). 
So the process of integration from this package is hard.