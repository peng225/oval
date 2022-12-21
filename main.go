package main

import (
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/peng225/oval/argparser"
	"github.com/peng225/oval/runner"
)

func main() {
	var (
		numObj         int
		numWorker      int
		sizePattern    string
		time           int64
		bucketNamesStr string
		opeRatioStr    string
		endpoint       string
		profiler       bool
		saveFileName   string
		loadFileName   string
	)
	flag.IntVar(&numObj, "num_obj", 10, "The maximum number of objects.")
	flag.IntVar(&numWorker, "num_worker", 1, "The number of workers.")
	flag.StringVar(&sizePattern, "size", "4k", "The size of object. Should be in the form like \"8k\" or \"4k-2m\". The unit \"g\" or \"G\" is not allowed.")
	flag.Int64Var(&time, "time", 3, "Time duration in seconds to run the workload.")
	flag.StringVar(&bucketNamesStr, "bucket", "", "The name list of the buckets. e.g. \"bucket1,bucket2\"")
	flag.StringVar(&opeRatioStr, "ope_ratio", "1,1,1", "The ration of put, get and delete operations. e.g. \"2,3,1\"")
	flag.StringVar(&endpoint, "endpoint", "", "The endpoint URL and TCP port number. e.g. \"http://127.0.0.1:9000\"")
	flag.BoolVar(&profiler, "profiler", false, "Enable profiler.")
	flag.StringVar(&saveFileName, "save", "", "File name to save the execution context.")
	flag.StringVar(&loadFileName, "load", "", "File name to load the execution context.")
	flag.Parse()

	log.SetFlags(log.Lshortfile)

	minSize, maxSize, err := argparser.ParseSize(sizePattern)
	if err != nil {
		log.Fatal(err)
	}
	opeRatios, err := argparser.ParseOpeRatio(opeRatioStr)
	if err != nil {
		log.Fatal(err)
	}
	bucketNames := strings.Split(bucketNamesStr, ",")

	if numObj%numWorker != 0 {
		fmt.Printf("warning: The number of objects (%d) is not divisible by the number of workers (%d). Only %d objects will be used.\n",
			numObj, numWorker, numObj/numWorker*numWorker)
	}

	var r *runner.Runner
	timeInMs := time * 1000
	if loadFileName == "" {
		execContext := &runner.ExecutionContext{
			Endpoint:    endpoint,
			BucketNames: bucketNames,
			NumObj:      numObj,
			NumWorker:   numWorker,
			MinSize:     minSize,
			MaxSize:     maxSize,
		}
		r = runner.NewRunner(execContext, opeRatios, timeInMs, profiler, loadFileName)
	} else {
		r = runner.NewRunnerFromLoadFile(loadFileName, opeRatios, timeInMs, profiler)
	}
	r.Run()
	if saveFileName != "" {
		err := r.SaveContext(saveFileName)
		if err != nil {
			panic(err)
		}
	}
}
