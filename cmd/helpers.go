package cmd

import (
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"golang.org/x/term"
)

// confirmIfTTY shows a huh confirmation prompt when running on a TTY.
// Returns (true, nil) if confirmed, (false, nil) if declined, (false, err) on error.
// If not a TTY, returns (true, nil) so non-interactive callers always proceed.
func confirmIfTTY(msg string) (bool, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return true, nil
	}
	confirm := false
	if err := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().Title(msg).Value(&confirm),
	)).Run(); err != nil {
		return false, err
	}
	return confirm, nil
}

// boolToYesNo converts a boolean to "yes" or "no".
func boolToYesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// abortedMsg prints "Aborted." to stdout.
func abortedMsg() {
	fmt.Fprintln(os.Stdout, "Aborted.")
}
