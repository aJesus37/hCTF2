package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/ajesus37/hCTF2/internal/tui"
	"github.com/spf13/cobra"
)

var submissionsCompetition int64
var submissionsWatch bool

var submissionsCmd = &cobra.Command{
	Use:   "submissions",
	Short: "Show the live submission feed",
	RunE:  runSubmissions,
}

func init() {
	rootCmd.AddCommand(submissionsCmd)
	submissionsCmd.Flags().Int64VarP(&submissionsCompetition, "competition", "c", 0, "Competition ID (omit for global feed)")
	submissionsCmd.Flags().BoolVarP(&submissionsWatch, "watch", "w", false, "Refresh automatically every 5 seconds")
}

var submissionsCols = []tui.Column{
	{Header: "TIME", Width: 20},
	{Header: "USER", Width: 20},
	{Header: "CHALLENGE", Width: 22},
	{Header: "QUESTION", Width: 18},
	{Header: "CORRECT", Width: 8},
}


func runSubmissions(_ *cobra.Command, _ []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}

	render := func() error {
		subs, err := c.GetSubmissions(submissionsCompetition)
		if err != nil {
			return err
		}
		if jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(subs)
		}
		// Clear screen in watch mode.
		if submissionsWatch {
			fmt.Fprint(os.Stdout, "\033[H\033[2J")
		}
		if len(subs) == 0 {
			fmt.Println("No submissions found.")
		} else {
			var rows [][]string
			for _, s := range subs {
				correct := tui.ErrorStyle.Render("✗")
				if s.IsCorrect {
					correct = tui.SolvedStyle.Render("✓")
				}
				rows = append(rows, []string{
					tui.Truncate(s.SubmittedAt, 20),
					tui.Truncate(s.UserName, 20),
					tui.Truncate(s.ChallengeName, 22),
					tui.Truncate(s.QuestionName, 18),
					correct,
				})
			}
			tui.PrintTable(os.Stdout, submissionsCols, rows)
			if submissionsCompetition == 0 {
				fmt.Fprintf(os.Stdout, "\n%s entries (global feed)\n", strconv.Itoa(len(rows)))
			} else {
				fmt.Fprintf(os.Stdout, "\n%s entries (competition %d)\n", strconv.Itoa(len(rows)), submissionsCompetition)
			}
		}
		if submissionsWatch {
			fmt.Fprintf(os.Stdout, tui.MutedStyle.Render("Refreshing every 5s — Ctrl+C to quit")+"\n")
		}
		return nil
	}

	if err := render(); err != nil {
		return err
	}
	if !submissionsWatch {
		return nil
	}
	for {
		time.Sleep(5 * time.Second)
		if err := render(); err != nil {
			return err
		}
	}
}
