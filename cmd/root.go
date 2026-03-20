package cmd

import (
	"fmt"
	"os"
	"runtime"

	"github.com/spf13/cobra"
)

var (
	serverOverride string
	jsonOutput     bool
	quietOutput    bool
)

var rootCmd = &cobra.Command{
	Use:               "hctf",
	Short:             "hCTF — self-hosted CTF platform",
	Long:              "hCTF is a self-hosted CTF platform. Run 'hctf serve' to start the server.",
	CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
}

// Execute runs the root command with the given version string.
func Execute(version string) {
	rootCmd.Version = version
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(rootCmd.Version)
	},
}

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Print build information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("hCTF %s\n", rootCmd.Version)
		fmt.Printf("  go:      %s\n", runtime.Version())
		fmt.Printf("  os/arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
		fmt.Printf("  cpus:    %d\n", runtime.NumCPU())
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&serverOverride, "server", "", "Server URL (overrides config)")
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	rootCmd.PersistentFlags().BoolVar(&quietOutput, "quiet", false, "Minimal output")
	rootCmd.AddCommand(versionCmd, infoCmd, updateCmd)
}
