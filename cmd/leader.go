package cmd

import (
	"log"

	"github.com/peng225/oval/multiprocess"
	"github.com/spf13/cobra"
)

var (
	followerList []string
)

// leaderCmd represents the leader command
var leaderCmd = &cobra.Command{
	Use:   "leader",
	Short: "Start the leader of the multi-process mode",
	Long:  `Start the leader of the multi-process mode.`,
	Run: func(cmd *cobra.Command, args []string) {
		handleCommonFlags()

		err := multiprocess.InitFollower(followerList)
		if err != nil {
			log.Fatal(err)
		}
		err = multiprocess.StartFollower(followerList, execContext,
			opeRatio, execTime.Milliseconds(), multipartThresh)
		if err != nil {
			log.Fatal(err)
		}
		log.Println("Sent start requests to all followers.")

		successAll, report, err := multiprocess.GetResultFromAllFollower(followerList)
		if err != nil {
			log.Println(err)
		}
		log.Print("The report from followers:\n" + report)

		if !successAll {
			log.Fatal("Some followers' workload failed.")
		}
	},
}

func init() {
	rootCmd.AddCommand(leaderCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// leaderCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// leaderCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	defineCommonFlags(leaderCmd)
	leaderCmd.Flags().StringSliceVar(&followerList, "follower_list", nil, "The follower list. e.g. \"http://localhost:8080,http://localhost:8081\"")

	leaderCmd.MarkFlagRequired("follower_list")
}
