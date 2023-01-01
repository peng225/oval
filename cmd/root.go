package cmd

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/peng225/oval/argparser"
	"github.com/peng225/oval/runner"
	"github.com/spf13/cobra"
)

var (
	numObj       int64
	numWorker    int
	sizePattern  string
	execTime     time.Duration
	bucketNames  []string
	opeRatioStr  string
	endpoint     string
	profiler     bool
	saveFileName string
	loadFileName string

	minSize, maxSize int
	opeRatio         []float64
	execContext      *runner.ExecutionContext
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "oval",
	Short: "A data validation tool for S3-compatible object storages",
	Long: `A data validation tool for S3-compatible object storages.
If no subcommands are specified, Oval runs in the single-process mode.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: func(cmd *cobra.Command, args []string) {
		handleCommonFlags()

		// Check if a file with the name "saveFileName" exists.
		_, err := os.Stat(saveFileName)
		if err == nil {
			fmt.Print(`A file "` + saveFileName + `" already exists. Are you sure to overwrite it? (y/N) `)
			var userInput string
			_, err = fmt.Scan(&userInput)
			if err != nil {
				log.Fatal(err)
			}
			if userInput != "y" {
				saveFileName = ""
				log.Println("Execution was canceled.")
				return
			}
		}

		var r *runner.Runner
		if loadFileName == "" {
			r = runner.NewRunner(execContext, opeRatio, execTime.Milliseconds(), profiler, loadFileName, 0)
		} else {
			r = runner.NewRunnerFromLoadFile(loadFileName, opeRatio, execTime.Milliseconds(), profiler)
		}
		err = r.Run(nil)
		if err != nil {
			log.Fatal("r.Run() failed.")
		}

		if saveFileName != "" {
			err := r.SaveContext(saveFileName)
			if err != nil {
				log.Fatal(err)
			}
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.oval.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	// rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	defineCommonFlags(rootCmd)
	rootCmd.Flags().BoolVar(&profiler, "profiler", false, "Enable profiler.")
	rootCmd.Flags().StringVar(&saveFileName, "save", "", "File name to save the execution context.")
	rootCmd.Flags().StringVar(&loadFileName, "load", "", "File name to load the execution context.")

	rootCmd.MarkFlagsMutuallyExclusive("bucket", "load")
	rootCmd.MarkFlagsMutuallyExclusive("endpoint", "load")
}

func handleCommonFlags() {
	var err error
	minSize, maxSize, err = argparser.ParseSize(sizePattern)
	if err != nil {
		log.Fatal(err)
	}
	opeRatio, err = argparser.ParseOpeRatio(opeRatioStr)
	if err != nil {
		log.Fatal(err)
	}

	if numObj%int64(numWorker) != 0 {
		log.Printf("warning: The number of objects (%d) is not divisible by the number of workers (%d). Only %d objects will be used.\n",
			numObj, numWorker, numObj/int64(numWorker*numWorker))
	}

	execContext = &runner.ExecutionContext{
		Endpoint:    endpoint,
		BucketNames: bucketNames,
		NumObj:      numObj,
		NumWorker:   numWorker,
		MinSize:     minSize,
		MaxSize:     maxSize,
	}
}

func defineCommonFlags(cmd *cobra.Command) {
	cmd.Flags().Int64Var(&numObj, "num_obj", 10, "The maximum number of objects.")
	cmd.Flags().IntVar(&numWorker, "num_worker", 1, "The number of workers per process.")
	cmd.Flags().StringVar(&sizePattern, "size", "4k", "The size of object. Should be in the form like \"8k\" or \"4k-2m\". The unit \"g\" or \"G\" is not allowed.")
	cmd.Flags().DurationVar(&execTime, "time", time.Second*3, "Time duration for run the workload.")
	cmd.Flags().StringSliceVar(&bucketNames, "bucket", nil, "The name list of the buckets. e.g. \"bucket1,bucket2\"")
	cmd.Flags().StringVar(&opeRatioStr, "ope_ratio", "1,1,1", "The ration of put, get and delete operations. e.g. \"2,3,1\"")
	cmd.Flags().StringVar(&endpoint, "endpoint", "", "The endpoint URL and TCP port number. e.g. \"http://127.0.0.1:9000\"")
}
