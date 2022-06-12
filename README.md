# What is this

Oval is a data validation tool for S3 compatible object storages.

# Motivation

No storage systems can achieve 100% of data integrity. Unfortunately, data corruption sometimes occurs, and the business of storage users gets impacted terribly.

There is a lot of possible cause of data corruption. Among them, the data corruption due to software bugs is catastrophic. Though some storage systems have an internal defense mechanism against data corruption, it is sometimes useless when the data get corrupted in an undetectable way, or if there are bugs in the defense mechanism itself. In that case, data will get corrupted silently.

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

```
$ ./oval -size 4k-16k --time 10 -num_obj 1000 -num_worker 4 -bucket test-bucket
Validation start.
profile.go:175: profile: cpu profiling enabled, cpu.pprof
Validation finished.
Statistics report.
put count: 1157
get count: 3296
delete count: 1050
profile.go:175: profile: cpu profiling disabled, cpu.pprof
```