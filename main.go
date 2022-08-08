package main

import (
	"flag"
	"log"

	"github.com/peng225/oval/argparser"
	"github.com/peng225/oval/validator"
)

func main() {
	var (
		numObj       int
		numWorker    int
		sizePattern  string
		time         int64
		bucketName   string
		opeRatioStr  string
		endpoint     string
		profiler     bool
		saveFileName string
		loadFileName string
	)
	flag.IntVar(&numObj, "num_obj", 10, "The maximum number of objects.")
	flag.IntVar(&numWorker, "num_worker", 1, "The number of workers.")
	flag.StringVar(&sizePattern, "size", "4k", "The size of object. Should be in the form like \"8k\" or \"4k-2m\". The unit \"g\" or \"G\" is not allowed.")
	flag.Int64Var(&time, "time", 3, "Time duration in seconds to run the workload.")
	flag.StringVar(&bucketName, "bucket", "", "The name of the bucket.")
	flag.StringVar(&opeRatioStr, "ope_ratio", "1,1,1", "The ration of put, get and delete operations. Eg. \"2,3,1\"")
	flag.StringVar(&endpoint, "endpoint", "", "The endpoint URL and TCP port number. Eg. \"http://127.0.0.1:9000\"")
	flag.BoolVar(&profiler, "profiler", false, "Enable profiler.")
	flag.StringVar(&saveFileName, "save", "", "File name to save the execution context.")
	flag.StringVar(&loadFileName, "load", "", "File name to load the execution context.")
	flag.Parse()

	log.SetFlags(log.Lshortfile)

	minSize, maxSize, err := argparser.SizeParse(sizePattern)
	if err != nil {
		log.Fatal(err)
	}
	opeRatios, err := argparser.OpeRatioParse(opeRatioStr)
	if err != nil {
		log.Fatal(err)
	}

	var r *validator.Runner
	timeInMs := time * 1000
	if loadFileName == "" {
		execContext := &validator.ExecutionContext{
			Endpoint:   endpoint,
			BucketName: bucketName,
			NumObj:     numObj,
			NumWorker:  numWorker,
			MinSize:    minSize,
			MaxSize:    maxSize,
		}
		r = validator.NewRunner(execContext, opeRatios, timeInMs, profiler, loadFileName)
	} else {
		r = validator.NewRunnerFromLoadFile(loadFileName, opeRatios, timeInMs, profiler)
	}
	r.Run()
	if saveFileName != "" {
		err := r.SaveContext(saveFileName)
		if err != nil {
			panic(err)
		}
	}
}
