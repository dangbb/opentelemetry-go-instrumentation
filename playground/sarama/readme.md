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
```