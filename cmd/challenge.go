package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/ajesus37/hCTF2/internal/tui"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var challengeCmd = &cobra.Command{Use: "challenge", Short: "Manage and browse challenges", Aliases: []string{"ch"}}
var challengeListCmd = &cobra.Command{Use: "list", Short: "List all challenges", RunE: runChallengeList}
var challengeGetCmd = &cobra.Command{Use: "get <id>", Short: "Show challenge details", Args: cobra.ExactArgs(1), RunE: runChallengeGet}
var challengeCreateCmd = &cobra.Command{Use: "create", Short: "Create a challenge (admin)", RunE: runChallengeCreate}
var challengeDeleteCmd = &cobra.Command{Use: "delete <id>", Short: "Delete a challenge (admin)", Args: cobra.ExactArgs(1), RunE: runChallengeDelete}

var (
	createTitle       string
	createCategory    string
	createDifficulty  string
	createDescription string
	createPoints      int
)

func init() {
	rootCmd.AddCommand(challengeCmd)
	challengeCmd.AddCommand(challengeListCmd, challengeGetCmd, challengeCreateCmd, challengeDeleteCmd, challengeBrowseCmd)
	challengeCreateCmd.Flags().StringVar(&createTitle, "title", "", "Challenge title")
	challengeCreateCmd.Flags().StringVar(&createCategory, "category", "", "Category")
	challengeCreateCmd.Flags().StringVar(&createDifficulty, "difficulty", "", "Difficulty")
	challengeCreateCmd.Flags().StringVar(&createDescription, "description", "", "Description (markdown)")
	challengeCreateCmd.Flags().IntVar(&createPoints, "points", 100, "Point value")
}

func runChallengeList(_ *cobra.Command, _ []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}
	challenges, err := c.ListChallenges()
	if err != nil {
		return err
	}
	if jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(challenges)
	}
	cols := []tui.Column{
		{Header: "ID", Width: 10},
		{Header: "TITLE", Width: 30},
		{Header: "CATEGORY", Width: 15},
		{Header: "DIFF", Width: 12},
		{Header: "PTS", Width: 6},
	}
	var rows [][]string
	for _, ch := range challenges {
		id := ch.ID
		if len(id) > 8 {
			id = id[:8] + "..."
		}
		rows = append(rows, []string{id, ch.Title, ch.Category, ch.Difficulty, strconv.Itoa(ch.InitialPoints)})
	}
	tui.PrintTable(os.Stdout, cols, rows)
	return nil
}

func runChallengeGet(_ *cobra.Command, args []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}
	ch, err := c.GetChallenge(args[0])
	if err != nil {
		return err
	}
	if jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(ch)
	}
	fmt.Fprintf(os.Stdout, "%s  %s  [%s / %s]  %d pts\n\n",
		tui.HeaderStyle.Render(ch.Title), tui.MutedStyle.Render(ch.ID),
		ch.Category, ch.Difficulty, ch.InitialPoints)
	if ch.Description != "" {
		r, _ := glamour.NewTermRenderer(glamour.WithAutoStyle())
		out, _ := r.Render(ch.Description)
		fmt.Fprint(os.Stdout, out)
	}
	return nil
}

func runChallengeCreate(_ *cobra.Command, _ []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}
	if term.IsTerminal(int(os.Stdin.Fd())) && (createTitle == "" || createCategory == "") {
		pointsStr := strconv.Itoa(createPoints)
		if err := huh.NewForm(huh.NewGroup(
			huh.NewInput().Title("Title").Value(&createTitle),
			huh.NewInput().Title("Category").Value(&createCategory),
			huh.NewInput().Title("Difficulty").Value(&createDifficulty),
			huh.NewInput().Title("Points").Value(&pointsStr),
		)).Run(); err != nil {
			return err
		}
		if p, err := strconv.Atoi(pointsStr); err == nil {
			createPoints = p
		}
	}
	ch, err := c.CreateChallenge(createTitle, createCategory, createDifficulty, createDescription, createPoints)
	if err != nil {
		return err
	}
	if quietOutput {
		fmt.Fprintln(os.Stdout, ch.ID)
		return nil
	}
	fmt.Fprintf(os.Stdout, "Created challenge %s (%s)\n", ch.Title, ch.ID)
	return nil
}

func runChallengeDelete(_ *cobra.Command, args []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}
	if err := c.DeleteChallenge(args[0]); err != nil {
		return err
	}
	if !quietOutput {
		fmt.Fprintf(os.Stdout, "Deleted %s\n", args[0])
	}
	return nil
}

var challengeBrowseCmd = &cobra.Command{
	Use:   "browse",
	Short: "Interactively browse and select challenges",
	RunE:  runChallengeBrowse,
}

func runChallengeBrowse(_ *cobra.Command, _ []string) error {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return fmt.Errorf("browse requires an interactive terminal")
	}
	c, err := newClient()
	if err != nil {
		return err
	}
	challenges, err := c.ListChallenges()
	if err != nil {
		return err
	}
	var tuiChallenges []tui.Challenge
	for _, ch := range challenges {
		tuiChallenges = append(tuiChallenges, tui.Challenge{
			ID:       ch.ID,
			Title:    ch.Title,
			Category: ch.Category,
			Points:   ch.InitialPoints,
		})
	}
	id, err := tui.RunBrowser(tuiChallenges)
	if err != nil {
		return err
	}
	if id == "" {
		return nil
	}
	return runChallengeGet(nil, []string{id})
}
