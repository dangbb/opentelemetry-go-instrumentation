## Try to access runtime.proc function to extract goroutine id.

Add checkpoint:
```shell
b runtime.execute
b runtime.goexit0
```

`goid` position is at

Examinate:
1. x/a 0xc0000061a0 (address of gp)
2. x/u 0xc0000061a0+152 -> address of goid, next is how to check if process belongs to what goid, in case of assign to GC, network, ...

Trying to check other function which get from user goroutine, and can be used to match.

## Test

Try debugging to get stack trace of goroutine.

Lot of goroutine goes for runtime.gopark
A few go for actual function

Stack low & high ?
After we extract the goroutine, how can we know what trace ID belong to what gid
Can we extract it like we do with goroutine function ?

Waiting function is stored at 
```go
func chanrecv1(c *hchan, elem unsafe.Pointer) {
	chanrecv(c, elem, true)
}
```

Then gopark is called
```go
func gopark(unlockf func(*g, unsafe.Pointer) bool, lock unsafe.Pointer, reason waitReason, traceEv byte, traceskip int) {
```
Go park receive a wait reason value.

Note:
- Execute is like select and reschedule goroutine.
- Goroutine pointer inside `execute` function look for


Understand how goroutine and scheduling in Golang work.
- 4 ways for scheduler to create goroutine: keywork `go`, gc, syscall, sync.
- Scheduling decision: Choose what goroutine which using specific components at a specific slice of time.
- Context switching


Async syscall:
- network poller used to process the syscall 
- using queue of goroutine, which will be used by processor and machine respectively
- 