# 1. Cilium/bpd and libbpf: basic example 

Target: develop eBPF application based ib cilium/ebpf.

File system:
- *.c: kernel state part of eBPF program.
- *.go: user state part of eBPF program.
- *.o: immediate target file. 

Using command `readelf -a bpf_bpfeb.o`, which used to read content of elf format file.
`elf` means Executable and Linkable Format.
Use to store binary, lib, and core dump (??) on disk in Linux and unix-based.
`elf` is flexible, allow to execute in various processor type.
written bby high level lang (C, C++).
Cannot be executed directly by CPU.

The working flow is as following:
1. In *.c file, user define the program to interact with register state.
2. From *.c file, generate *.o and *.go file, which provide interface, mapping, and program bytecode (via embed),
3. Using define structure from *.go, dev can do their own task on user programming space.

The c source code file is then compiled by clang, into `bpf_bpfeb.o` and `bpf_bpfel.o` for big endian and little endian, respectively.
The eBPF bytecode is then embedded into go code.

Problem: In generate command, come with series of header file to the clang compiler.
Must depend on `libbpf` library if using following header.

