package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ajesus37/hCTF2/internal/tui"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var teamCmd = &cobra.Command{Use: "team", Short: "Team management"}
var teamListCmd = &cobra.Command{Use: "list", Short: "List all teams", RunE: runTeamList}
var teamGetCmd = &cobra.Command{Use: "get <id>", Short: "Show team details", Args: cobra.ExactArgs(1), RunE: runTeamGet}
var teamCreateCmd = &cobra.Command{Use: "create [name]", Short: "Create a team", Args: cobra.MaximumNArgs(1), RunE: runTeamCreate}
var teamJoinCmd = &cobra.Command{Use: "join <invite-code>", Short: "Join a team by invite code", Args: cobra.ExactArgs(1), RunE: runTeamJoin}
var teamLeaveCmd = &cobra.Command{Use: "leave", Short: "Leave your current team", RunE: runTeamLeave}
var teamDisbandCmd = &cobra.Command{Use: "disband", Short: "Disband your team (owner only)", RunE: runTeamDisband}
var teamTransferCmd = &cobra.Command{
	Use:   "transfer <member-id>",
	Short: "Transfer team ownership to a member",
	Args:  cobra.ExactArgs(1),
	RunE:  runTeamTransfer,
}
var teamInviteRegenCmd = &cobra.Command{
	Use:   "invite-regen",
	Short: "Regenerate your team's invite code",
	RunE:  runTeamInviteRegen,
}

func init() {
	rootCmd.AddCommand(teamCmd)
	teamCmd.AddCommand(teamListCmd, teamGetCmd, teamCreateCmd, teamJoinCmd, teamLeaveCmd, teamDisbandCmd, teamTransferCmd, teamInviteRegenCmd)
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
	t, members, err := c.GetTeam(args[0])
	if err != nil {
		return err
	}
	if jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(map[string]interface{}{"team": t, "members": members})
	}
	fmt.Fprintf(os.Stdout, "Name:  %s\nID:    %s\n", t.Name, t.ID)
	if t.InviteID != "" {
		fmt.Fprintf(os.Stdout, "Invite code: %s\n", t.InviteID)
	}
	if len(members) > 0 {
		fmt.Fprintln(os.Stdout, "\nMembers:")
		cols := []tui.Column{
			{Header: "ID", Width: 38},
			{Header: "NAME", Width: 25},
			{Header: "EMAIL", Width: 30},
		}
		var rows [][]string
		for _, m := range members {
			name := m.Name
			if t.OwnerID != "" && m.ID == t.OwnerID {
				name += " (owner)"
			}
			rows = append(rows, []string{m.ID, name, m.Email})
		}
		tui.PrintTable(os.Stdout, cols, rows)
	}
	return nil
}

func runTeamCreate(_ *cobra.Command, args []string) error {
	teamName := ""
	if len(args) > 0 {
		teamName = args[0]
	}
	if term.IsTerminal(int(os.Stdin.Fd())) && teamName == "" {
		if err := huh.NewForm(huh.NewGroup(
			huh.NewInput().Title("Team name").Value(&teamName),
		)).Run(); err != nil {
			return err
		}
	}
	if teamName == "" {
		return fmt.Errorf("team name is required")
	}
	c, err := newClient()
	if err != nil {
		return err
	}
	t, err := c.CreateTeam(teamName)
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

func runTeamLeave(_ *cobra.Command, _ []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}
	if err := c.LeaveTeam(); err != nil {
		return err
	}
	if !quietOutput {
		fmt.Fprintln(os.Stdout, "Left team.")
	}
	return nil
}

func runTeamDisband(_ *cobra.Command, _ []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}
	ok, err := confirmIfTTY("Disband your team? This cannot be undone.")
	if err != nil {
		return err
	}
	if !ok {
		abortedMsg()
		return nil
	}
	if err := c.DisbandTeam(); err != nil {
		return err
	}
	if !quietOutput {
		fmt.Fprintln(os.Stdout, "Team disbanded.")
	}
	return nil
}

func runTeamTransfer(_ *cobra.Command, args []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}
	ok, err := confirmIfTTY(fmt.Sprintf("Transfer ownership to member %s?", args[0]))
	if err != nil {
		return err
	}
	if !ok {
		abortedMsg()
		return nil
	}
	if err := c.TransferOwnership(args[0]); err != nil {
		return err
	}
	if !quietOutput {
		fmt.Fprintln(os.Stdout, "Ownership transferred.")
	}
	return nil
}

func runTeamInviteRegen(_ *cobra.Command, _ []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}
	code, err := c.RegenerateInvite()
	if err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, code)
	return nil
}
