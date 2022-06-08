package main

import (
	"flag"
	"log"

	"github.com/peng225/oval/argparser"
	"github.com/peng225/oval/validator"
)

func main() {
	var (
		numObj      int
		numWorker   int
		sizePattern string
		time        int64
		bucketName  string
	)
	// TODO: tune the percentage of each operation
	flag.IntVar(&numObj, "num_obj", 10, "The maximum number of objects.")
	flag.IntVar(&numWorker, "num_worker", 1, "The number of workers.")
	flag.StringVar(&sizePattern, "size", "4k", "The size of object. Should be in the form like \"8k\" or \"4k-2m\". The unit \"g\" or \"G\" is not allowed.")
	flag.Int64Var(&time, "time", 3, "Time duration in seconds to run the workload.")
	flag.StringVar(&bucketName, "bucket", "", "The name of the bucket.")
	flag.Parse()

	log.SetFlags(log.Lshortfile)

	minSize, maxSize, err := argparser.SizeParse(sizePattern)
	if err != nil {
		log.Fatal(err)
	}

	r := validator.Runner{
		NumObj:     numObj,
		NumWorker:  numWorker,
		MinSize:    minSize,
		MaxSize:    maxSize,
		TimeInMs:   time * 1000,
		BucketName: bucketName,
	}

	r.Init()
	r.Run()
}
