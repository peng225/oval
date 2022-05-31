package main

import (
	"flag"

	"github.com/peng225/oval/argparser"
	"github.com/peng225/oval/validator"
)

func main() {
	var (
		numObj      int64
		numWorker   int
		sizePattern string
		numRound    int
		bucketName  string
	)
	flag.Int64Var(&numObj, "num_obj", 10, "The number of objects.")
	flag.IntVar(&numWorker, "num_worker", 1, "The number of workers.")
	flag.StringVar(&sizePattern, "size", "4k", "The size of object. Should be in the form like \"4k-2m\" or \"8k\".")
	flag.IntVar(&numRound, "round", 1, "The number of test rounds.")
	flag.StringVar(&bucketName, "bucket", "", "The name of the bucket.")
	flag.Parse()

	minSize, maxSize, err := argparser.SizeParse(sizePattern)
	if err != nil {
		panic(err)
	}

	v := validator.Validator{
		NumObj:     numObj,
		NumWorker:  numWorker,
		MinSize:    minSize,
		MaxSize:    maxSize,
		NumRound:   numRound,
		BucketName: bucketName,
	}

	v.Init()
	v.Run()
}
