package main

import (
	"flag"
	"log"
	"strings"

	"github.com/peng225/oval/argparser"
	"github.com/peng225/oval/multiprocess"
	"github.com/peng225/oval/runner"
)

func main() {
	var (
		numObj         int64
		numWorker      int
		sizePattern    string
		time           int64
		bucketNamesStr string
		opeRatioStr    string
		endpoint       string
		profiler       bool
		saveFileName   string
		loadFileName   string

		// Flags for the multi-process mode
		leader          bool
		follower        bool
		followerListStr string
		followerPort    int
	)
	const (
		invalidPortNumber = -1
	)
	flag.Int64Var(&numObj, "num_obj", 10, "The maximum number of objects.")
	flag.IntVar(&numWorker, "num_worker", 1, "The number of workers per process.")
	flag.StringVar(&sizePattern, "size", "4k", "The size of object. Should be in the form like \"8k\" or \"4k-2m\". The unit \"g\" or \"G\" is not allowed.")
	flag.Int64Var(&time, "time", 3, "Time duration in seconds to run the workload.")
	flag.StringVar(&bucketNamesStr, "bucket", "", "The name list of the buckets. e.g. \"bucket1,bucket2\"")
	flag.StringVar(&opeRatioStr, "ope_ratio", "1,1,1", "The ration of put, get and delete operations. e.g. \"2,3,1\"")
	flag.StringVar(&endpoint, "endpoint", "", "The endpoint URL and TCP port number. e.g. \"http://127.0.0.1:9000\"")
	flag.BoolVar(&profiler, "profiler", false, "Enable profiler.")
	flag.StringVar(&saveFileName, "save", "", "File name to save the execution context.")
	flag.StringVar(&loadFileName, "load", "", "File name to load the execution context.")
	flag.BoolVar(&leader, "leader", false, "The process run as the leader.")
	flag.BoolVar(&follower, "follower", false, "The process run as a follower.")
	flag.StringVar(&followerListStr, "follower_list", "", "[For leader] The follower list.")
	flag.IntVar(&followerPort, "follower_port", invalidPortNumber, "[For follower] TCP port number to which a follower listens.")
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

	if numObj%int64(numWorker) != 0 {
		log.Printf("warning: The number of objects (%d) is not divisible by the number of workers (%d). Only %d objects will be used.\n",
			numObj, numWorker, numObj/int64(numWorker*numWorker))
	}

	if leader && follower {
		log.Fatal("Both leader and follower flags cannot be specified at the same time.")
	}

	if !leader && followerListStr != "" {
		log.Println("warning: follower_list flag is ignored because this process is not running as the leader.")
		followerListStr = ""
	}
	followerList := make([]string, 0)
	if leader {
		if followerPort != invalidPortNumber {
			log.Println("warning: follower_port flag is ignored because this process is running as the leader.")
			followerPort = invalidPortNumber
		}
		followerList, err = argparser.ParseFollowerList(followerListStr)
		if err != nil {
			log.Fatal(err)
		}
	} else if follower {
		if followerPort == invalidPortNumber {
			log.Fatalf("Invalid follower port.")
		}
	}

	if leader || follower {
		if saveFileName != "" || loadFileName != "" {
			log.Println("warning: save/load file function is not supported when the multi-process mode is enabled. save/load file will be ignored.")
			saveFileName = ""
			loadFileName = ""
		}
		if profiler {
			log.Println("warning: Profiling function is not supported when the multi-process mode is enabled. Profiler flag will be ignored.")
			profiler = false
		}
	}

	execContext := &runner.ExecutionContext{
		Endpoint:    endpoint,
		BucketNames: bucketNames,
		NumObj:      numObj,
		NumWorker:   numWorker,
		MinSize:     minSize,
		MaxSize:     maxSize,
	}
	timeInMs := time * 1000

	if leader {
		err = multiprocess.InitFollower(followerList)
		if err != nil {
			log.Fatal(err)
		}
		err = multiprocess.StartFollower(followerList, execContext,
			opeRatios, timeInMs)
		if err != nil {
			log.Fatal(err)
		}
		log.Println("Sent start requests to all followers.")
	} else if follower {
		multiprocess.StartServer(followerPort)
		// Follower processes do not go beyond this line.
	}

	if follower {
		log.Fatal("Followers must not come here.")
	}

	if leader {
		successAll, report, err := multiprocess.GetResultFromAllFollower(followerList)
		if err != nil {
			log.Println(err)
		}
		log.Print("The report from followers:\n" + report)

		if !successAll {
			log.Fatal("Some followers' workload failed.")
		}
	} else {
		// The single-process mode
		var r *runner.Runner
		if loadFileName == "" {
			r = runner.NewRunner(execContext, opeRatios, timeInMs, profiler, loadFileName, 0)
		} else {
			r = runner.NewRunnerFromLoadFile(loadFileName, opeRatios, timeInMs, profiler)
		}
		err = r.Run(nil)
		if err != nil {
			log.Fatal(err)
		}

		if saveFileName != "" {
			err := r.SaveContext(saveFileName)
			if err != nil {
				log.Fatal(err)
			}
		}

	}
}
