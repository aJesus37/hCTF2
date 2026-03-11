package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/ajesus37/hCTF2/internal/tui"
	"github.com/spf13/cobra"
)

var competitionCmd = &cobra.Command{Use: "competition", Short: "Competition management", Aliases: []string{"comp"}}
var compListCmd = &cobra.Command{Use: "list", Short: "List competitions", RunE: runCompList}
var compCreateCmd = &cobra.Command{Use: "create <name>", Short: "Create a competition (admin)", Args: cobra.ExactArgs(1), RunE: runCompCreate}
var compStartCmd = &cobra.Command{Use: "start <id>", Short: "Force-start a competition (admin)", Args: cobra.ExactArgs(1), RunE: runCompStart}
var compEndCmd = &cobra.Command{Use: "end <id>", Short: "Force-end a competition (admin)", Args: cobra.ExactArgs(1), RunE: runCompEnd}

func init() {
	rootCmd.AddCommand(competitionCmd)
	competitionCmd.AddCommand(compListCmd, compCreateCmd, compStartCmd, compEndCmd)
}

func runCompList(_ *cobra.Command, _ []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}
	comps, err := c.ListCompetitions()
	if err != nil {
		return err
	}
	if jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(comps)
	}
	cols := []tui.Column{
		{Header: "ID", Width: 6},
		{Header: "NAME", Width: 30},
		{Header: "STATUS", Width: 12},
	}
	var rows [][]string
	for _, co := range comps {
		rows = append(rows, []string{strconv.FormatInt(co.ID, 10), co.Name, co.Status})
	}
	tui.PrintTable(os.Stdout, cols, rows)
	return nil
}

func runCompCreate(_ *cobra.Command, args []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}
	co, err := c.CreateCompetition(args[0])
	if err != nil {
		return err
	}
	if quietOutput {
		fmt.Fprintln(os.Stdout, co.ID)
		return nil
	}
	fmt.Fprintf(os.Stdout, "Created competition %q (id: %d)\n", co.Name, co.ID)
	return nil
}

func runCompStart(_ *cobra.Command, args []string) error {
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid competition id: %s", args[0])
	}
	c, err := newClient()
	if err != nil {
		return err
	}
	if err := c.ForceStartCompetition(id); err != nil {
		return err
	}
	if !quietOutput {
		fmt.Fprintf(os.Stdout, "Started competition %d\n", id)
	}
	return nil
}

func runCompEnd(_ *cobra.Command, args []string) error {
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid competition id: %s", args[0])
	}
	c, err := newClient()
	if err != nil {
		return err
	}
	if err := c.ForceEndCompetition(id); err != nil {
		return err
	}
	if !quietOutput {
		fmt.Fprintf(os.Stdout, "Ended competition %d\n", id)
	}
	return nil
}
