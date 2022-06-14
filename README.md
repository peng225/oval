# What is this

Oval is a data validation tool for S3 compatible object storages.

# Motivation

Storage systems can hardly achieve 100% of data integrity. 
Though there is a lot of possible cause of data corruption, the data corruption due to software bugs is catastrophic. So continuous efforts to reduce the possibility of data corruption are required.   

Some storage systems have an internal defense mechanism against data corruption. However, it is sometimes useless when the data get corrupted in undetectable ways, or if there are bugs in the defense mechanism itself. In that case, data will get corrupted silently.

So, it is important for storage system's developers and users
to check if their storage systems have enough data integrity from the storage user's point of view.
People sometimes check data integrity by the following steps.

1. Generate the data to be written.
2. Write the data which is generated in step1.
3. Read the just-written data, and compare it with the original data which is generated in the step1.

However, this procedure is not enough;
There may be data destroying bugs in the background data rebalancing process, or data may get corrupted by the concurrent data write, etc.
Generally speaking, those kinds of bugs are hard to detect.

To reveal the root cause of those difficult data corruption bugs,
it is important to know when the data was destroyed and which process caused the problem.
For block storage, there are some brilliant tools (cf. [vdbench](https://www.oracle.com/technetwork/server-storage/vdbench-1901683.pdf)),
but object storages seem to lack those data validation tools. That is why Oval was developed.

## How Oval works

Oval checks the data integrity during the object I/O benchmarks.
Oval stores the expected data contents for each object in memory,
and compares the actual data with it every time an object is read.

To detect the data corruption as soon as possible, Oval issues get operations not only at random
but also right before and after the put and delete operations.

Oval splits the data into several data units each of which has the size of 256 bytes,
and embeds the following information in the contents of the data unit itself:

- Bucket name
- Object's key name
- Write count (the generation of data)
- Offset of the data unit
- Unix time
  - This is not used for data validation, but is helpful to investigate when the corrupted data was written.
- Worker ID

If the read data include some unexpected content,
Oval dumps the actual binary data,
and you can investigate the root cause of the data corruption using the dump data.

## How to use

```
$ go build -o oval
$ ./oval -h                                                                  
Usage of ./oval:
  -bucket string
        The name of the bucket.
  -endpoint string
        The endpoint URL and TCP port number. Eg. "http://127.0.0.1:9000"
  -num_obj int
        The maximum number of objects. (default 10)
  -num_worker int
        The number of workers. (default 1)
  -ope_ratio string
        The ration of put, get and delete operations. Eg. "2,3,1" (default "1,1,1")
  -size string
        The size of object. Should be in the form like "8k" or "4k-2m". The unit "g" or "G" is not allowed. (default "4k")
  -time int
        Time duration in seconds to run the workload. (default 3)
```

### Example

#### Success case

```
$ ./oval -size 4k-16k --time 5 -num_obj 1000 -num_worker 4 -bucket test-bucket -endpoint http://localhost:9000
Worker ID = 0xb67f, Key = [ov0000000000, ov0000000249]
Worker ID = 0xb680, Key = [ov0000000250, ov0000000499]
Worker ID = 0xb681, Key = [ov0000000500, ov0000000749]
Worker ID = 0xb682, Key = [ov0000000750, ov0000000999]
Validation start.
Validation finished.
Statistics report.
put count: 515
get count: 499
get (for validation) count: 992
delete count: 465
```

#### Data corruption case

```
$ ./oval -size 4k-16k --time 5 -num_obj 1000 -num_worker 4 -bucket test-bucket -endpoint http://localhost:9000
Worker ID = 0xf10c, Key = [ov0000000000, ov0000000249]
Worker ID = 0xf10d, Key = [ov0000000250, ov0000000499]
Worker ID = 0xf10e, Key = [ov0000000500, ov0000000749]
Worker ID = 0xf10f, Key = [ov0000000750, ov0000000999]
Validation start.
validator.go:78: Data validation error occurred after put.
WriteCount is wrong. (expected = "2", actual = "1")
00000000  74 65 73 74 2d 62 75 63  6b 65 74 20 6f 76 30 30  |test-bucket ov00|
          ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^ bucket name
                                               ^^^^^^^^^^^
00000010  30 30 30 30 30 33 33 30  01 00 00 00 00 04 00 00  |00000330........|
          ^^^^^^^^^^^^^^^^^^^^^^^ key name
                                   ^^^^^^^^^^^ write count
                                               ^^^^^^^^^^^ byte offset in this object
00000020  6d 9d 62 9c 55 e1 05 00  0d f1 00 00 2c 2d 2e 2f  |m.b.U.......,-./|
          ^^^^^^^^^^^^^^^^^^^^^^^ unix time (micro sec)
                                   ^^^^^^^^^^^ Worker ID
00000030  30 31 32 33 34 35 36 37  38 39 3a 3b 3c 3d 3e 3f  |0123456789:;<=>?|
00000040  40 41 42 43 44 45 46 47  48 49 4a 4b 4c 4d 4e 4f  |@ABCDEFGHIJKLMNO|
00000050  50 51 52 53 54 55 56 57  58 59 5a 5b 5c 5d 5e 5f  |PQRSTUVWXYZ[\]^_|
00000060  60 61 62 63 64 65 66 67  68 69 6a 6b 6c 6d 6e 6f  |`abcdefghijklmno|
00000070  70 71 72 73 74 75 76 77  78 79 7a 7b 7c 7d 7e 7f  |pqrstuvwxyz{|}~.|
00000080  80 81 82 83 84 85 86 87  88 89 8a 8b 8c 8d 8e 8f  |................|
00000090  90 91 92 93 94 95 96 97  98 99 9a 9b 9c 9d 9e 9f  |................|
000000a0  a0 a1 a2 a3 a4 a5 a6 a7  a8 a9 aa ab ac ad ae af  |................|
000000b0  b0 b1 b2 b3 b4 b5 b6 b7  b8 b9 ba bb bc bd be bf  |................|
000000c0  c0 c1 c2 c3 c4 c5 c6 c7  c8 c9 ca cb cc cd ce cf  |................|
000000d0  d0 d1 d2 d3 d4 d5 d6 d7  d8 d9 da db dc dd de df  |................|
000000e0  e0 e1 e2 e3 e4 e5 e6 e7  e8 e9 ea eb ec ed ee ef  |................|
000000f0  f0 f1 f2 f3 f4 f5 f6 f7  f8 f9 fa fb fc fd fe ff  |................|
```
