package cmd

import (
	"log/slog"
	"os"

	"github.com/peng225/oval/internal/multiprocess"
	"github.com/spf13/cobra"
)

const (
	invalidPortNumber = -1
)

var (
	followerPort int
)

// followerCmd represents the follower command
var followerCmd = &cobra.Command{
	Use:   "follower",
	Short: "Start a follower of the multi-process mode",
	Long:  `Start a follower of the multi-process mode.`,
	Run: func(cmd *cobra.Command, args []string) {
		handleCommonFlags()

		if followerPort <= 0 {
			slog.Error("Invalid follower port.", "followerPort", followerPort)
			os.Exit(1)
		}

		if caCertFileName != "" {
			// Check if a file with the name "caCertFileName" exists.
			_, err := os.Stat(caCertFileName)
			if err != nil {
				slog.Error(err.Error())
				os.Exit(1)
			}
		}

		multiprocess.StartServer(followerPort, caCertFileName)
		// Follower processes do not go beyond this line.
	},
}

func init() {
	rootCmd.AddCommand(followerCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// followerCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// followerCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	defineCommonFlags(followerCmd)
	followerCmd.Flags().IntVar(&followerPort, "follower_port", invalidPortNumber, "TCP port number to which the follower listens.")
	followerCmd.Flags().StringVar(&caCertFileName, "cacert", "", "File name of CA certificate.")

	err := followerCmd.MarkFlagRequired("follower_port")
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}
