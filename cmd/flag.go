package cmd

import (
	"fmt"
	"os"

	"github.com/ajesus37/hCTF2/internal/tui"
	"github.com/spf13/cobra"
)

var flagCmd = &cobra.Command{Use: "flag", Short: "Flag submission"}
var flagSubmitCmd = &cobra.Command{
	Use:   "submit <question-id> <flag>",
	Short: "Submit a flag for a question",
	Args:  cobra.ExactArgs(2),
	RunE:  runFlagSubmit,
}

func init() {
	rootCmd.AddCommand(flagCmd)
	flagCmd.AddCommand(flagSubmitCmd)
}

func runFlagSubmit(_ *cobra.Command, args []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}
	result, err := c.SubmitFlag(args[0], args[1])
	if err != nil {
		return err
	}
	if result.Correct {
		fmt.Fprintln(os.Stdout, tui.SuccessStyle.Render(fmt.Sprintf("Correct! +%d pts", result.Points)))
	} else {
		fmt.Fprintln(os.Stderr, tui.ErrorStyle.Render("Incorrect flag"))
	}
	return nil
}
