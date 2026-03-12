package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/ajesus37/hCTF2/internal/tui"
	"github.com/spf13/cobra"
)

var scoreboardCmd = &cobra.Command{
	Use:   "scoreboard",
	Short: "Show the current scoreboard",
	RunE:  runScoreboard,
}

func init() {
	rootCmd.AddCommand(scoreboardCmd)
}

func runScoreboard(_ *cobra.Command, _ []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}
	entries, err := c.GetScoreboard()
	if err != nil {
		return err
	}
	if jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(entries)
	}
	if len(entries) == 0 {
		fmt.Println("No scoreboard entries found.")
		return nil
	}
	cols := []tui.Column{
		{Header: "RANK", Width: 6},
		{Header: "USER", Width: 25},
		{Header: "TEAM", Width: 20},
		{Header: "SCORE", Width: 8},
		{Header: "SOLVES", Width: 7},
	}
	var rows [][]string
	for _, e := range entries {
		team := ""
		if e.TeamName != nil {
			team = *e.TeamName
		}
		rows = append(rows, []string{
			strconv.Itoa(e.Rank),
			e.UserName,
			team,
			strconv.Itoa(e.Points),
			strconv.Itoa(e.SolveCount),
		})
	}
	tui.PrintTable(os.Stdout, cols, rows)
	return nil
}
