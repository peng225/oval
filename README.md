# Oval

## What is Oval?

Oval is a data validation tool for S3-compatible object storages.

## Motivation

Storage systems can hardly achieve 100% of data integrity. Data is usually damaged by two factors; failures and software bugs.
For failures, there are many techniques to reduce the impact, e.g. RAID, replication, erasure coding, backup. Though it is impossible to protect storage systems perfectly from data loss due to failures, the impact of failures can be controlled by paying the cost of data redundancy.

On the other hand, data corruption due to software bugs is unacceptable. It is hard to estimate the damage of data loss due to bugs because bugs can cause every bad thing.
For example, some bugs may destroy all redundant data copies. Unfortunately, no storage system is free from software bugs, but continuous efforts to reduce data corruption bugs are meaningful.

Some storage systems have internal defense mechanisms against data corruption, e.g. periodic CRC check, synchronous T10 DIF check.
However, these mechanisms can never be perfect solutions for two reasons. First, they have a theoretical limit on the ability to detect data corruption.
Secondly, these mechanisms themselves may also have bugs since they are also implemented by humans.

So, what should we do? One possible solution is to test storage systems in an end-to-end manner.
The simplest way to check data integrity is as follows.

1. Generate the random data.
2. Write the data which is generated in step 1.
3. Read the just-written data, and compare it with the original data which is generated in step 1.

However, this procedure is not enough;
There may be data-destroying bugs in the background data balancing process, or data may get corrupted by the concurrent data write, etc.
Generally speaking, those kinds of bugs are hard to detect.

To reveal the root cause of those bugs,
it is effective to do whatever operations to test storage systems while running I/O benchmark, and check the data integrity in the I/O benchmark tool.
For block storage, there are some brilliant tools for this purpose (cf. [vdbench](https://www.oracle.com/technetwork/server-storage/vdbench-1901683.pdf)),
but object storages seem to lack those data validation tools. That is why Oval was developed.

### Isn't the checksum not enough?

S3 interface provides a way to check the data integrity by [checksum](https://docs.aws.amazon.com/AmazonS3/latest/userguide/checking-object-integrity.html). It ensures that the data is not corrupted during the entire lifetime of each object. Isn't it enough for data integrity testing?

In my understanding, no, it's not. Storage systems can have a horrible bug, which provokes returning OK to a PUT request without writing the data to any persistent medium. Then we throw a GET request to the object, finding that the older one is returned. Since the older object is not corrupted, we cannot detect the bug by checking the checksum value.

## How Oval works

Oval checks the data integrity during the object I/O benchmarks.
Oval stores the expected data contents for each object in memory,
and compares the actual data with it every time objects are read.

To detect the data corruption immediately, Oval issues get requests not only at random,
but also right before and after the put and delete operations.

Oval splits the data into several data units each of which has the size of 256 bytes,
and embeds the following information in the contents of the data unit itself:

- Bucket name
- Object's key name
- Write count (the generation of data)
- Offset of the data unit
- Worker ID
- Unix time
  - This is not used for data validation, but is helpful to investigate when the corrupted data was written.

If the read data include some unexpected content,
Oval dumps the actual binary data,
and you can investigate the root cause of the data corruption using the dump data.

## How to use

### Example 1: Success case

```console
$ ./oval --size 4k-16k --time 5s --num_obj 1024 --num_worker 4 --bucket "test-bucket,test-bucket2" --endpoint http://localhost:9000
2024-03-04T22:14:33.2+09:00 INFO worker.go:35 Worker info (runnerID=0, workerID=0x614, key=(head=ov0000000000, tail=ov00000000ff))
2024-03-04T22:14:33.201+09:00 INFO worker.go:35 Worker info (runnerID=0, workerID=0x615, key=(head=ov0001000000, tail=ov00010000ff))
2024-03-04T22:14:33.201+09:00 INFO worker.go:35 Worker info (runnerID=0, workerID=0x616, key=(head=ov0002000000, tail=ov00020000ff))
2024-03-04T22:14:33.202+09:00 INFO worker.go:35 Worker info (runnerID=0, workerID=0x617, key=(head=ov0003000000, tail=ov00030000ff))
2024-03-04T22:14:33.358+09:00 INFO runner.go:164 Clearing bucket. (bucket=test-bucket)
2024-03-04T22:14:33.608+09:00 INFO runner.go:169 Bucket cleared successfully.
2024-03-04T22:14:33.613+09:00 INFO runner.go:164 Clearing bucket. (bucket=test-bucket2)
2024-03-04T22:14:33.8+09:00 INFO runner.go:169 Bucket cleared successfully.
2024-03-04T22:14:33.8+09:00 INFO runner.go:177 Validation start.
2024-03-04T22:14:38.818+09:00 INFO runner.go:217 Validation finished.
2024-03-04T22:14:38.818+09:00 INFO stat.go:42 Statistics report. (report=(putCount=493, numUploadedParts=493, getCount=456, getForValidationCount=927, listCount=0, deleteCount=426))
```

### Example 2: Data corruption case

```console
$ ./oval --size 4k-16k --time 5s --num_obj 1024 --num_worker 4 --bucket "test-bucket,test-bucket2" --endpoint http://localhost:9000
2024-03-04T22:15:27.467+09:00 INFO worker.go:35 Worker info (runnerID=0, workerID=0x5619, key=(head=ov0000000000, tail=ov00000000ff))
2024-03-04T22:15:27.467+09:00 INFO worker.go:35 Worker info (runnerID=0, workerID=0x561a, key=(head=ov0001000000, tail=ov00010000ff))
2024-03-04T22:15:27.467+09:00 INFO worker.go:35 Worker info (runnerID=0, workerID=0x561b, key=(head=ov0002000000, tail=ov00020000ff))
2024-03-04T22:15:27.468+09:00 INFO worker.go:35 Worker info (runnerID=0, workerID=0x561c, key=(head=ov0003000000, tail=ov00030000ff))
2024-03-04T22:15:27.483+09:00 INFO runner.go:164 Clearing bucket. (bucket=test-bucket)
2024-03-04T22:15:27.75+09:00 INFO runner.go:169 Bucket cleared successfully.
2024-03-04T22:15:27.752+09:00 INFO runner.go:164 Clearing bucket. (bucket=test-bucket2)
2024-03-04T22:15:27.848+09:00 INFO runner.go:169 Bucket cleared successfully.
2024-03-04T22:15:27.848+09:00 INFO runner.go:177 Validation start.
2024-03-04T22:15:29.143+09:00 ERROR worker.go:116 data validation error occurred after put.
- WriteCount is wrong. (expected = "2", actual = "1")
- OffsetInObject is wrong. (expected = "0", actual = "256")
00000000  74 65 73 74 2d 62 75 63  6b 65 74 20 20 20 20 20  |test-bucket     |
          ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^ bucket name
00000010  6f 76 30 30 30 32 30 30  30 30 65 32 01 00 00 00  |ov00020000e2....|
          ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^ key name
                                               ^^^^^^^^^^^ write count
00000020  00 01 00 00 1b 56 00 00  a5 37 02 85 d5 12 06 00  |.....V...7......|
          ^^^^^^^^^^^ byte offset in this object
                      ^^^^^^^^^^^ worker ID
                                   ^^^^^^^^^^^^^^^^^^^^^^^ unix time (micro sec)
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
 (runnerID=0, workerID=0x561b)
2024-03-04T22:15:29.144+09:00 ERROR worker.go:214 operation error S3: DeleteObject, https response error StatusCode: 0, RequestID: , HostID: , canceled, context canceled (runnerID=0, workerID=0x5619)
2024-03-04T22:15:29.144+09:00 ERROR worker.go:136 operation error S3: GetObject, https response error StatusCode: 0, RequestID: , HostID: , canceled, context canceled (runnerID=0, workerID=0x561c)
2024-03-04T22:15:29.144+09:00 ERROR worker.go:92 operation error S3: PutObject, https response error StatusCode: 0, RequestID: , HostID: , canceled, context canceled (runnerID=0, workerID=0x561a)
2024-03-04T22:15:29.144+09:00 INFO runner.go:217 Validation finished.
2024-03-04T22:15:29.144+09:00 INFO stat.go:42 Statistics report. (report=(putCount=146, numUploadedParts=146, getCount=128, getForValidationCount=249, listCount=0, deleteCount=102))
2024-03-04T22:15:29.144+09:00 ERROR root.go:97 r.Run() failed.
```

## Execution mode

Oval has two execution modes; the single-process mode and the multi-process mode. In the single-process mode, Oval runs as the single-process in a single node. That is the easiest way to use Oval.

However, in some test scenarios, a single process mode is insufficient. Bugs in storage systems are often revealed when some error happens in the storage system. One of the most common forms of error is timeout. When a timeout occurs for some internal process in the storage systems, it must be handled properly to clean things up. These error handling logics are error-prone.

Another common source of bugs is concurrency control. For example, processing many client requests may sometimes require a locking mechanism or other concurrent programming techniques. They have been producing many hard-to-find bugs for a long history.

One of the best ways to make these things happen artificially and find hidden bugs is to stress the storage system. The multi-process mode of Oval was developed for this purpose.

In the multi-process mode, follower processes run as web application servers first. Then, the leader process issues HTTP requests to all followers, and followers start their workload. The leader periodically checks whether each follower's job has finished. After all the work has been done, the leader collects the result and reports it to the user.

### How to use the multi-process mode

1. Start as many follower processes as you need.
2. Run the leader process.
3. Stop follower processes after you finish your tests.

### Example

#### follower1

```console
$ ./oval follower --follower_port 8080
2024-03-04T22:17:56.847+09:00 INFO follower.go:57 Start server. (port=8080)
2024-03-04T22:18:15.297+09:00 INFO follower.go:77 Received a start request.
2024-03-04T22:18:15.298+09:00 INFO follower.go:159 Start follower params. (ID=0, Context={http://localhost:9000 [test-bucket test-bucket2] 1024 4 4096 16384 0 []}, OpeRatio=[0.3333333333333333 0.3333333333333333 0.3333333333333333 0], TimeInMs=5000, MultipartThresh=104857600)
2024-03-04T22:18:15.299+09:00 INFO worker.go:35 Worker info (runnerID=0, workerID=0x2983, key=(head=ov0000000000, tail=ov00000000ff))
2024-03-04T22:18:15.3+09:00 INFO worker.go:35 Worker info (runnerID=0, workerID=0x2984, key=(head=ov0001000000, tail=ov00010000ff))
2024-03-04T22:18:15.3+09:00 INFO worker.go:35 Worker info (runnerID=0, workerID=0x2985, key=(head=ov0002000000, tail=ov00020000ff))
2024-03-04T22:18:15.3+09:00 INFO worker.go:35 Worker info (runnerID=0, workerID=0x2986, key=(head=ov0003000000, tail=ov00030000ff))
2024-03-04T22:18:15.35+09:00 INFO runner.go:164 Clearing bucket. (bucket=test-bucket)
2024-03-04T22:18:15.551+09:00 INFO runner.go:169 Bucket cleared successfully.
2024-03-04T22:18:15.554+09:00 INFO runner.go:164 Clearing bucket. (bucket=test-bucket2)
2024-03-04T22:18:15.687+09:00 INFO runner.go:169 Bucket cleared successfully.
2024-03-04T22:18:15.687+09:00 INFO runner.go:177 Validation start.
2024-03-04T22:18:20.73+09:00 INFO runner.go:217 Validation finished.
2024-03-04T22:18:20.73+09:00 INFO stat.go:42 Statistics report. (report=(putCount=266, numUploadedParts=266, getCount=206, getForValidationCount=494, listCount=0, deleteCount=222))
```

#### follower2

```console
$ ./oval follower --follower_port 8081
2024-03-04T22:18:02.368+09:00 INFO follower.go:57 Start server. (port=8081)
2024-03-04T22:18:15.301+09:00 INFO follower.go:77 Received a start request.
2024-03-04T22:18:15.302+09:00 INFO follower.go:159 Start follower params. (ID=1, Context={http://localhost:9000 [test-bucket test-bucket2] 1024 4 4096 16384 0 []}, OpeRatio=[0.3333333333333333 0.3333333333333333 0.3333333333333333 0], TimeInMs=5000, MultipartThresh=104857600)
2024-03-04T22:18:15.303+09:00 INFO worker.go:35 Worker info (runnerID=1, workerID=0xc2c1, key=(head=ov0100000000, tail=ov01000000ff))
2024-03-04T22:18:15.303+09:00 INFO worker.go:35 Worker info (runnerID=1, workerID=0xc2c2, key=(head=ov0101000000, tail=ov01010000ff))
2024-03-04T22:18:15.303+09:00 INFO worker.go:35 Worker info (runnerID=1, workerID=0xc2c3, key=(head=ov0102000000, tail=ov01020000ff))
2024-03-04T22:18:15.304+09:00 INFO worker.go:35 Worker info (runnerID=1, workerID=0xc2c4, key=(head=ov0103000000, tail=ov01030000ff))
2024-03-04T22:18:15.35+09:00 INFO runner.go:164 Clearing bucket. (bucket=test-bucket)
2024-03-04T22:18:15.464+09:00 INFO runner.go:169 Bucket cleared successfully.
2024-03-04T22:18:15.468+09:00 INFO runner.go:164 Clearing bucket. (bucket=test-bucket2)
2024-03-04T22:18:15.507+09:00 INFO runner.go:169 Bucket cleared successfully.
2024-03-04T22:18:15.507+09:00 INFO runner.go:177 Validation start.
2024-03-04T22:18:20.523+09:00 INFO runner.go:217 Validation finished.
2024-03-04T22:18:20.523+09:00 INFO stat.go:42 Statistics report. (report=(putCount=268, numUploadedParts=268, getCount=251, getForValidationCount=497, listCount=0, deleteCount=227))
```

#### leader

```console
$ ./oval leader --follower_list "http://localhost:8080,http://localhost:8081" --size 4k-16k --time 5s --num_obj 1024 --num_worker 4 --bucket "test-bucket,test-bucket2" --endpoint http://localhost:9000
2024-03-04T22:18:15.303+09:00 INFO leader.go:73 Sent start requests to all followers.
2024-03-04T22:18:20.835+09:00 INFO leader.go:81 The report from http://localhost:8081:OK
2024-03-04T22:18:20.835+09:00 INFO leader.go:81 The report from http://localhost:8080:OK
```

## Testing before and after the blackout

Oval provides functionality to save and load the execution context. This feature enables the data integrity test before and after the blackout. A typical test scenario is as follows.

1. Run Oval and save the execution context.
2. Stop and start the storage system.
3. Run Oval again using the saved context.

This kind of test is effective for revealing a bug of failing to persist the data.

### Example

```console
# Use `--save` option to save the execution context.
$ ./oval --size 4k-16k --time 5s --num_obj 1024 --num_worker 4 --bucket "test-bucket,test-bucket2" --endpoint http://localhost:9000 --save test.json
2024-03-04T22:20:50.177+09:00 INFO worker.go:35 Worker info (runnerID=0, workerID=0x50e4, key=(head=ov0000000000, tail=ov00000000ff))
2024-03-04T22:20:50.177+09:00 INFO worker.go:35 Worker info (runnerID=0, workerID=0x50e5, key=(head=ov0001000000, tail=ov00010000ff))
2024-03-04T22:20:50.178+09:00 INFO worker.go:35 Worker info (runnerID=0, workerID=0x50e6, key=(head=ov0002000000, tail=ov00020000ff))
2024-03-04T22:20:50.178+09:00 INFO worker.go:35 Worker info (runnerID=0, workerID=0x50e7, key=(head=ov0003000000, tail=ov00030000ff))
2024-03-04T22:20:50.209+09:00 INFO runner.go:164 Clearing bucket. (bucket=test-bucket)
2024-03-04T22:20:50.343+09:00 INFO runner.go:169 Bucket cleared successfully.
2024-03-04T22:20:50.346+09:00 INFO runner.go:164 Clearing bucket. (bucket=test-bucket2)
2024-03-04T22:20:50.494+09:00 INFO runner.go:169 Bucket cleared successfully.
2024-03-04T22:20:50.494+09:00 INFO runner.go:177 Validation start.
2024-03-04T22:20:55.532+09:00 INFO runner.go:217 Validation finished.
2024-03-04T22:20:55.532+09:00 INFO stat.go:42 Statistics report. (report=(putCount=393, numUploadedParts=393, getCount=341, getForValidationCount=733, listCount=0, deleteCount=330))

# Use ``--load` option to load the saved context.
$ ./oval --time 3s --load test.json
2024-03-04T22:21:30.354+09:00 INFO worker.go:35 Worker info (runnerID=0, workerID=0x50e4, key=(head=ov0000000000, tail=ov00000000ff))
2024-03-04T22:21:30.354+09:00 INFO worker.go:35 Worker info (runnerID=0, workerID=0x50e5, key=(head=ov0001000000, tail=ov00010000ff))
2024-03-04T22:21:30.354+09:00 INFO worker.go:35 Worker info (runnerID=0, workerID=0x50e6, key=(head=ov0002000000, tail=ov00020000ff))
2024-03-04T22:21:30.355+09:00 INFO worker.go:35 Worker info (runnerID=0, workerID=0x50e7, key=(head=ov0003000000, tail=ov00030000ff))
2024-03-04T22:21:30.362+09:00 INFO runner.go:177 Validation start.
2024-03-04T22:21:33.377+09:00 INFO runner.go:217 Validation finished.
2024-03-04T22:21:33.377+09:00 INFO stat.go:42 Statistics report. (report=(putCount=339, numUploadedParts=339, getCount=329, getForValidationCount=670, listCount=0, deleteCount=318))
```

## Internals

The component diagram of the multi-process mode is as follows.

```mermaid
flowchart TB
    Leader -- Start workload/Get the result ---> HTTPserver1[HTTP server1]
    Leader -- Start workload/Get the result ---> HTTPserver2[HTTP server2]

    subgraph Leader_process
      Leader
    end

    subgraph Follower process1
      direction TB
      HTTPserver1 -- Start workload/Get the result -->runner1
      runner1 --> worker1-1
      runner1 --> worker1-2
    end

    subgraph Follower process2
      direction TB
      HTTPserver2 -- Start workload/Get the result --> runner2
      runner2 --> worker2-1
      runner2 --> worker2-2
    end

    worker1-1 -- Object I/O --> storage[S3-compatible storage]
    worker1-2 -- Object I/O --> storage[S3-compatible storage]
    worker2-1 -- Object I/O --> storage[S3-compatible storage]
    worker2-2 -- Object I/O --> storage[S3-compatible storage]
```

