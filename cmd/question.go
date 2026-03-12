package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/ajesus37/hCTF2/internal/tui"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var questionCmd = &cobra.Command{Use: "question", Short: "Manage challenge questions (admin)", Aliases: []string{"q"}}
var questionListCmd = &cobra.Command{Use: "list <challenge-id>", Short: "List questions for a challenge", Args: cobra.ExactArgs(1), RunE: runQuestionList}
var questionCreateCmd = &cobra.Command{Use: "create", Short: "Create a question (admin)", RunE: runQuestionCreate}
var questionDeleteCmd = &cobra.Command{Use: "delete <id>", Short: "Delete a question (admin)", Args: cobra.ExactArgs(1), RunE: runQuestionDelete}

var (
	qChallengeID string
	qName        string
	qFlag        string
	qPoints      int
)

func init() {
	rootCmd.AddCommand(questionCmd)
	questionCmd.AddCommand(questionListCmd, questionCreateCmd, questionDeleteCmd)
	questionCreateCmd.Flags().StringVar(&qChallengeID, "challenge", "", "Challenge ID")
	questionCreateCmd.Flags().StringVar(&qName, "name", "", "Question name")
	questionCreateCmd.Flags().StringVar(&qFlag, "flag", "", "Flag value")
	questionCreateCmd.Flags().IntVar(&qPoints, "points", 100, "Point value")
}

func runQuestionList(_ *cobra.Command, args []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}
	qs, err := c.ListQuestions(args[0])
	if err != nil {
		return err
	}
	if jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(qs)
	}
	cols := []tui.Column{
		{Header: "ID", Width: 10},
		{Header: "NAME", Width: 30},
		{Header: "FLAG MASK", Width: 20},
		{Header: "PTS", Width: 6},
	}
	var rows [][]string
	for _, q := range qs {
		id := q.ID
		if len(id) > 8 {
			id = id[:8] + "..."
		}
		mask := ""
		if q.FlagMask != "" {
			mask = q.FlagMask
		}
		rows = append(rows, []string{id, q.Name, mask, strconv.Itoa(q.Points)})
	}
	tui.PrintTable(os.Stdout, cols, rows)
	return nil
}

func runQuestionCreate(_ *cobra.Command, _ []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}
	if term.IsTerminal(int(os.Stdin.Fd())) && qName == "" {
		pointsStr := strconv.Itoa(qPoints)
		if err := huh.NewForm(huh.NewGroup(
			huh.NewInput().Title("Challenge ID").Value(&qChallengeID),
			huh.NewInput().Title("Question name").Value(&qName),
			huh.NewInput().Title("Flag").Value(&qFlag),
			huh.NewInput().Title("Points").Value(&pointsStr),
		)).Run(); err != nil {
			return err
		}
		if p, err := strconv.Atoi(pointsStr); err == nil {
			qPoints = p
		}
	}
	if qChallengeID == "" || qFlag == "" {
		return fmt.Errorf("--challenge and --flag are required")
	}
	q, err := c.CreateQuestion(qChallengeID, qName, qFlag, qPoints)
	if err != nil {
		return err
	}
	if quietOutput {
		fmt.Fprintln(os.Stdout, q.ID)
		return nil
	}
	fmt.Fprintf(os.Stdout, "Created question %s (%s)\n", q.Name, q.ID)
	return nil
}

func runQuestionDelete(_ *cobra.Command, args []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}
	if err := c.DeleteQuestion(args[0]); err != nil {
		return err
	}
	if !quietOutput {
		fmt.Fprintf(os.Stdout, "Deleted question %s\n", args[0])
	}
	return nil
}
