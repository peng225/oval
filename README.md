# oval
A data validation tool for S3 compatible object storages.

## How to use

```
$ go build -o oval
$ ./oval -h                                                                  
Usage of ./oval:
  -bucket string
        The name of the bucket.
  -num_obj int
        The maximum number of objects. (default 10)
  -num_worker int
        The number of workers. (default 1)
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