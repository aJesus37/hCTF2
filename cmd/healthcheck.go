package cmd

import (
	"fmt"
	"net/http"
	"os"

	"github.com/spf13/cobra"
)

var healthcheckPort int

var healthcheckCmd = &cobra.Command{
	Use:   "healthcheck",
	Short: "Check server health (for use in Docker HEALTHCHECK)",
	RunE: func(cmd *cobra.Command, args []string) error {
		url := fmt.Sprintf("http://localhost:%d/healthz", healthcheckPort)
		resp, err := http.Get(url) //nolint:noctx
		if err != nil {
			fmt.Fprintf(os.Stderr, "healthcheck failed: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			fmt.Fprintf(os.Stderr, "healthcheck failed: status %d\n", resp.StatusCode)
			os.Exit(1)
		}
		return nil
	},
}

func init() {
	healthcheckCmd.Flags().IntVar(&healthcheckPort, "port", 8090, "Server port to check")
	rootCmd.AddCommand(healthcheckCmd)
}
