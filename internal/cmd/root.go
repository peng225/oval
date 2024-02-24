package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/peng225/oval/internal/argparser"
	"github.com/peng225/oval/internal/logger"
	"github.com/peng225/oval/internal/runner"
	"github.com/spf13/cobra"
)

var (
	numObj             int
	numWorker          int
	sizePattern        string
	execTime           time.Duration
	bucketNames        []string
	opeRatioStr        string
	endpoint           string
	multipartThreshStr string
	profiler           bool
	saveFileName       string
	loadFileName       string
	caCertFileName     string
	logFormat          string

	minSize, maxSize int
	opeRatio         []float64
	multipartThresh  int
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
		handleSubCommonFlags()

		// Check if a file with the name "saveFileName" exists.
		_, err := os.Stat(saveFileName)
		if err == nil {
			fmt.Print(`A file "` + saveFileName + `" already exists. Are you sure to overwrite it? (y/N) `)
			var userInput string
			_, err = fmt.Scan(&userInput)
			if err != nil {
				slog.Error(err.Error())
				os.Exit(1)
			}
			if userInput != "y" {
				saveFileName = ""
				slog.Info("Execution was canceled.")
				return
			}
		}

		if caCertFileName != "" {
			// Check if a file with the name "caCertFileName" exists.
			_, err = os.Stat(caCertFileName)
			if err != nil {
				slog.Error(err.Error())
				os.Exit(1)
			}
		}

		var r *runner.Runner
		if loadFileName == "" {
			r = runner.NewRunner(execContext, opeRatio, execTime.Milliseconds(),
				profiler, loadFileName, 0, multipartThresh, caCertFileName)
		} else {
			r = runner.NewRunnerFromLoadFile(loadFileName, opeRatio, execTime.Milliseconds(),
				profiler, multipartThresh, caCertFileName)
		}
		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
		defer stop()
		err = r.InitBucket(ctx)
		if err != nil {
			slog.Error("r.InitBucket() failed.", "err", err)
			if ctx.Err() == context.Canceled {
				return
			}
			os.Exit(1)
		}
		err = r.Run(ctx)
		if err != nil {
			slog.Error("r.Run() failed.")
			if ctx.Err() == context.Canceled {
				return
			}
			os.Exit(1)
		}

		if saveFileName != "" {
			err := r.SaveContext(saveFileName)
			if err != nil {
				slog.Error(err.Error())
				os.Exit(1)
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
	defineSubCommonFlags(rootCmd)
	rootCmd.Flags().BoolVar(&profiler, "profiler", false, "Enable profiler.")
	rootCmd.Flags().StringVar(&saveFileName, "save", "", "File name to save the execution context.")
	rootCmd.Flags().StringVar(&loadFileName, "load", "", "File name to load the execution context.")
	rootCmd.Flags().StringVar(&caCertFileName, "cacert", "", "File name of CA certificate.")

	rootCmd.MarkFlagsMutuallyExclusive("bucket", "load")
	rootCmd.MarkFlagsMutuallyExclusive("endpoint", "load")
}

func handleCommonFlags() {
	err := logger.SetLogFormat(logFormat)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}

func handleSubCommonFlags() {
	var err error
	minSize, maxSize, err = argparser.ParseSize(sizePattern)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
	opeRatio, err = argparser.ParseOpeRatio(opeRatioStr)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
	multipartThresh, err = argparser.ParseMultipartThresh(multipartThreshStr)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	if numWorker >= 256 {
		slog.Error("The number of workers must be less than 256.")
		os.Exit(1)
	}

	if numObj > 0x1000000 {
		slog.Error("The number of objects must be less than 16777216.")
		os.Exit(1)
	}

	if numObj < numWorker {
		slog.Error("The number of objects must be larger than or equal to the number of workers.")
		os.Exit(1)
	}

	if execTime < 0 {
		slog.Error("The execution time must be larger than or equal to 0.")
		os.Exit(1)
	}

	if numObj%numWorker != 0 {
		slog.Warn(fmt.Sprintf("The number of objects (%d) is not divisible by the number of workers (%d). Only %d objects will be used.",
			numObj, numWorker, numObj/numWorker*numWorker))
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
	cmd.Flags().StringVar(&logFormat, "log", "plane", `Log format ("plane" or "json").`)
}

func defineSubCommonFlags(cmd *cobra.Command) {
	cmd.Flags().IntVar(&numObj, "num_obj", 10, "The maximum number of objects per process.")
	cmd.Flags().IntVar(&numWorker, "num_worker", 1, "The number of workers per process.")
	cmd.Flags().StringVar(&sizePattern, "size", "4k", `The size of object. Should be in the form like "8k" or "4k-2m". Only "k", "m" and "g" is allowed as an unit.`)
	cmd.Flags().DurationVar(&execTime, "time", time.Second*3, "Time duration for run the workload. The value 0 means to run infinitely.")
	cmd.Flags().StringSliceVar(&bucketNames, "bucket", nil, "The name list of the buckets. e.g. \"bucket1,bucket2\"")
	cmd.Flags().StringVar(&opeRatioStr, "ope_ratio", "1,1,1,0", "The ration of put, get, delete and list operations. e.g. \"2,3,1,1\"")
	cmd.Flags().StringVar(&endpoint, "endpoint", "", "The endpoint URL and TCP port number. e.g. \"http://127.0.0.1:9000\"")
	cmd.Flags().StringVar(&multipartThreshStr, "multipart_thresh", "100m", `The threshold of the object size to switch to the multipart upload. Only "k", "m" and "g" is allowed as an unit.`)
}
