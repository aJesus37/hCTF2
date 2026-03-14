package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/ajesus37/hCTF2/internal/tui"
	"github.com/spf13/cobra"
)

var userCmd = &cobra.Command{Use: "user", Short: "User management (admin only)"}
var userListCmd = &cobra.Command{Use: "list", Short: "List all users", RunE: runUserList}
var userPromoteCmd = &cobra.Command{Use: "promote <id>", Short: "Grant admin to user", Args: cobra.ExactArgs(1), RunE: runUserPromote}
var userDemoteCmd = &cobra.Command{Use: "demote <id>", Short: "Revoke admin from user", Args: cobra.ExactArgs(1), RunE: runUserDemote}
var userDeleteCmd = &cobra.Command{Use: "delete <id>", Short: "Delete a user", Args: cobra.ExactArgs(1), RunE: runUserDelete}
var userProfileCmd = &cobra.Command{Use: "profile [<id>]", Short: "Show user profile stats (omit id for own profile)", Args: cobra.MaximumNArgs(1), RunE: runUserProfile}

func init() {
	rootCmd.AddCommand(userCmd)
	userCmd.AddCommand(userListCmd, userPromoteCmd, userDemoteCmd, userDeleteCmd, userProfileCmd)
}

func runUserList(_ *cobra.Command, _ []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}
	users, err := c.ListUsers()
	if err != nil {
		return err
	}
	if jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(users)
	}
	cols := []tui.Column{
		{Header: "ID", Width: 10},
		{Header: "EMAIL", Width: 30},
		{Header: "NAME", Width: 20},
		{Header: "ADMIN", Width: 6},
	}
	var rows [][]string
	for _, u := range users {
		id := tui.Truncate(u.ID, 10)
		admin := ""
		if u.IsAdmin {
			admin = tui.SolvedStyle.Render("✓")
		}
		rows = append(rows, []string{id, u.Email, u.Name, admin})
	}
	tui.PrintTable(os.Stdout, cols, rows)
	return nil
}

func runUserPromote(_ *cobra.Command, args []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}
	if err := c.PromoteUser(args[0], true); err != nil {
		return err
	}
	if !quietOutput {
		fmt.Fprintf(os.Stdout, "User %s promoted to admin\n", args[0])
	}
	return nil
}

func runUserDemote(_ *cobra.Command, args []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}
	if err := c.PromoteUser(args[0], false); err != nil {
		return err
	}
	if !quietOutput {
		fmt.Fprintf(os.Stdout, "User %s demoted\n", args[0])
	}
	return nil
}

func runUserDelete(_ *cobra.Command, args []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}
	if err := c.DeleteUser(args[0]); err != nil {
		return err
	}
	if !quietOutput {
		fmt.Fprintf(os.Stdout, "Deleted user %s\n", args[0])
	}
	return nil
}

func runUserProfile(_ *cobra.Command, args []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}
	var userID string
	if len(args) > 0 {
		userID = args[0]
	}
	profile, err := c.GetUserProfile(userID)
	if err != nil {
		return err
	}
	if jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(profile)
	}
	cols := []tui.Column{
		{Header: "FIELD", Width: 20},
		{Header: "VALUE", Width: 40},
	}
	team := ""
	if profile.TeamName != nil {
		team = *profile.TeamName
	}
	rows := [][]string{
		{"ID", profile.UserID},
		{"Name", profile.Name},
	}
	if profile.Email != "" {
		rows = append(rows, []string{"Email", profile.Email})
	}
	if team != "" {
		rows = append(rows, []string{"Team", team})
	}
	rows = append(rows,
		[]string{"Rank", strconv.Itoa(profile.Rank)},
		[]string{"Total Points", strconv.Itoa(profile.TotalPoints)},
		[]string{"Solves", strconv.Itoa(profile.SolvedCount)},
		[]string{"Total Submissions", strconv.Itoa(profile.TotalSubmissions)},
	)
	tui.PrintTable(os.Stdout, cols, rows)
	return nil
}
