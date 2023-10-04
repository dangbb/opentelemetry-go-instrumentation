## Try to access runtime.proc function to extract goroutine id.

Add checkpoint:
```shell
b runtime.execute
b runtime.goexit0
b runtime.runqput
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
- m, once attach to a p, surely used for executed go code
- g -> m -> p.
- m -> curg (current running goroutine) -> p (for the executed code, nil if not executing any)
- Where machine perform pop goroutine out of queue and perform code ? In short, the performing cycle of golang can be express as following step:
  1. Using keywork `go` to create a new goroutine (should track what and how goroutine **of this type** is created and handle before pushing to queue)
  2. Newly created goroutine is being pushed to local or global queue
  3. A M is being **waken**/**created** to execute golang code, can **steal**/**findrunnable** from global queue or others Ms.
  4. Schedule loop (?)
  5. Try to get goroutine execute (**G.fn()**)
  6. Clear, and reenter schedule loop (**goexit**)

Deeper thought 
- P is logical processor, which contains context of current running goroutine. So M can only run when accquired a P as context
- P contains list of G. In order to execute goroutine, M need to be holding context of P.
- **malg** create new goroutine.
- **newproc(fn \*funcval)** create new g to running fn. Put in the queue of g. 
  - (can be this)
- **newproc1**: accquire one m. Get P. Get goroutine from queue.
- newg.startpc = fn.fn
- Should take from field of M.curg, since it means current goroutine.
- How to check if goroutine belong to user or system. 
- runqput(pp, newg, true): This function often associate with newproc, which is used to create new goroutine. So, this function can be used to register goroutine id.
- To prevent size of the map not top big, should perform retention scan for every 5 minutes.
  1. First, extract that startpc function name from goroutine. If function ID = 17 || 10 || 16 (16 should be include with condition is fixed), or the function name has prefix `runtime.` (runtime/symtab.go:861)
  2. 