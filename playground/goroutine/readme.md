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

