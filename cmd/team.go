package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ajesus37/hCTF2/internal/tui"
	"github.com/spf13/cobra"
)

var teamCmd = &cobra.Command{Use: "team", Short: "Team management"}
var teamListCmd = &cobra.Command{Use: "list", Short: "List all teams", RunE: runTeamList}
var teamGetCmd = &cobra.Command{Use: "get <id>", Short: "Show team details", Args: cobra.ExactArgs(1), RunE: runTeamGet}
var teamCreateCmd = &cobra.Command{Use: "create <name>", Short: "Create a team", Args: cobra.ExactArgs(1), RunE: runTeamCreate}
var teamJoinCmd = &cobra.Command{Use: "join <invite-code>", Short: "Join a team by invite code", Args: cobra.ExactArgs(1), RunE: runTeamJoin}

func init() {
	rootCmd.AddCommand(teamCmd)
	teamCmd.AddCommand(teamListCmd, teamGetCmd, teamCreateCmd, teamJoinCmd)
}

func runTeamList(_ *cobra.Command, _ []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}
	teams, err := c.ListTeams()
	if err != nil {
		return err
	}
	if jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(teams)
	}
	cols := []tui.Column{
		{Header: "ID", Width: 38},
		{Header: "NAME", Width: 25},
	}
	var rows [][]string
	for _, t := range teams {
		rows = append(rows, []string{t.ID, t.Name})
	}
	tui.PrintTable(os.Stdout, cols, rows)
	return nil
}

func runTeamGet(_ *cobra.Command, args []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}
	t, err := c.GetTeam(args[0])
	if err != nil {
		return err
	}
	if jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(t)
	}
	fmt.Fprintf(os.Stdout, "Name:  %s\nID:    %s\n", t.Name, t.ID)
	if t.InviteID != "" {
		fmt.Fprintf(os.Stdout, "Invite code: %s\n", t.InviteID)
	}
	return nil
}

func runTeamCreate(_ *cobra.Command, args []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}
	t, err := c.CreateTeam(args[0])
	if err != nil {
		return err
	}
	if quietOutput {
		fmt.Fprintln(os.Stdout, t.ID)
		return nil
	}
	fmt.Fprintf(os.Stdout, "Created team %q (%s)\n", t.Name, t.ID)
	return nil
}

func runTeamJoin(_ *cobra.Command, args []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}
	if err := c.JoinTeam(args[0]); err != nil {
		return err
	}
	if !quietOutput {
		fmt.Fprintln(os.Stdout, "Joined team")
	}
	return nil
}
