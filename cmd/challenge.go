package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/ajesus37/hCTF2/internal/client"
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
var challengeUpdateCmd = &cobra.Command{Use: "update <id>", Short: "Update a challenge (admin)", Args: cobra.ExactArgs(1), RunE: runChallengeUpdate}

var (
	createTitle       string
	createCategory    string
	createDifficulty  string
	createDescription string
	createPoints      int

	updateTitle       string
	updateCategory    string
	updateDifficulty  string
	updateDescription string
	updatePoints      int
)

func init() {
	rootCmd.AddCommand(challengeCmd)
	challengeCmd.AddCommand(challengeListCmd, challengeGetCmd, challengeCreateCmd, challengeDeleteCmd, challengeBrowseCmd, challengeUpdateCmd)
	challengeCreateCmd.Flags().StringVar(&createTitle, "title", "", "Challenge title")
	challengeCreateCmd.Flags().StringVar(&createCategory, "category", "", "Category")
	challengeCreateCmd.Flags().StringVar(&createDifficulty, "difficulty", "", "Difficulty")
	challengeCreateCmd.Flags().StringVar(&createDescription, "description", "", "Description (markdown)")
	challengeCreateCmd.Flags().IntVar(&createPoints, "points", 100, "Point value")

	challengeUpdateCmd.Flags().StringVar(&updateTitle, "title", "", "Challenge title")
	challengeUpdateCmd.Flags().StringVar(&updateCategory, "category", "", "Category")
	challengeUpdateCmd.Flags().StringVar(&updateDifficulty, "difficulty", "", "Difficulty")
	challengeUpdateCmd.Flags().StringVar(&updateDescription, "description", "", "Description (markdown)")
	challengeUpdateCmd.Flags().IntVar(&updatePoints, "points", 0, "Point value")
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

const customSentinel = "__custom__"

func runChallengeCreate(_ *cobra.Command, _ []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}
	if term.IsTerminal(int(os.Stdin.Fd())) && (createTitle == "" || createCategory == "") {
		pointsStr := strconv.Itoa(createPoints)

		// Fetch categories and difficulties for the pickers (best-effort).
		cats, _ := c.ListCategories()
		diffs, _ := c.ListDifficulties()

		// Group 1: Title alone so it always fits on screen.
		if err := huh.NewForm(huh.NewGroup(
			huh.NewInput().Title("Title").Value(&createTitle),
		)).Run(); err != nil {
			return err
		}

		// Group 2: Category, Difficulty, Points, Description.
		var catSelection string
		var diffSelection string
		fields2 := []huh.Field{}

		// Category: select from existing + custom option, or plain input if none configured.
		if len(cats) > 0 {
			opts := make([]huh.Option[string], len(cats)+1)
			for i, cat := range cats {
				opts[i] = huh.NewOption(cat.Name, cat.Name)
			}
			opts[len(cats)] = huh.NewOption("Other (type custom)…", customSentinel)
			fields2 = append(fields2,
				huh.NewSelect[string]().Title("Category").Options(opts...).Value(&catSelection),
			)
		} else {
			fields2 = append(fields2, huh.NewInput().Title("Category").Value(&createCategory))
		}

		// Difficulty: select from existing + custom option, or plain input if none configured.
		if len(diffs) > 0 {
			opts := make([]huh.Option[string], len(diffs)+1)
			for i, d := range diffs {
				opts[i] = huh.NewOption(d.Name, d.Name)
			}
			opts[len(diffs)] = huh.NewOption("Other (type custom)…", customSentinel)
			fields2 = append(fields2,
				huh.NewSelect[string]().Title("Difficulty").Options(opts...).Value(&diffSelection),
			)
		} else {
			fields2 = append(fields2, huh.NewInput().Title("Difficulty").Value(&createDifficulty))
		}

		fields2 = append(fields2,
			huh.NewInput().Title("Points").Value(&pointsStr),
			huh.NewText().Title("Description (markdown)").Value(&createDescription),
		)

		if err := huh.NewForm(huh.NewGroup(fields2...)).Run(); err != nil {
			return err
		}

		// Resolve category.
		if len(cats) > 0 {
			if catSelection == customSentinel {
				if err := huh.NewForm(huh.NewGroup(
					huh.NewInput().Title("Custom category").Value(&createCategory),
				)).Run(); err != nil {
					return err
				}
			} else {
				createCategory = catSelection
			}
		}

		// Resolve difficulty.
		if len(diffs) > 0 {
			if diffSelection == customSentinel {
				if err := huh.NewForm(huh.NewGroup(
					huh.NewInput().Title("Custom difficulty").Value(&createDifficulty),
				)).Run(); err != nil {
					return err
				}
			} else {
				createDifficulty = diffSelection
			}
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

func runChallengeUpdate(_ *cobra.Command, args []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}
	id := args[0]

	// On TTY with no flags, pre-fill form from current values.
	if term.IsTerminal(int(os.Stdin.Fd())) && updateTitle == "" {
		ch, err := c.GetChallenge(id)
		if err != nil {
			return err
		}
		updateTitle = ch.Title
		updateCategory = ch.Category
		updateDifficulty = ch.Difficulty
		updateDescription = ch.Description
		if updatePoints == 0 {
			updatePoints = ch.InitialPoints
		}

		cats, _ := c.ListCategories()
		diffs, _ := c.ListDifficulties()

		// Group 1: Title alone so it always fits on screen.
		if err := huh.NewForm(huh.NewGroup(
			huh.NewInput().Title("Title").Value(&updateTitle),
		)).Run(); err != nil {
			return err
		}

		// Group 2: Category, Difficulty, Points, Description.
		pointsStr := strconv.Itoa(updatePoints)
		var catSelection string
		var diffSelection string
		fields2 := []huh.Field{}

		if len(cats) > 0 {
			opts := make([]huh.Option[string], len(cats)+1)
			for i, cat := range cats {
				opts[i] = huh.NewOption(cat.Name, cat.Name)
			}
			opts[len(cats)] = huh.NewOption("Other (type custom)…", customSentinel)
			fields2 = append(fields2, huh.NewSelect[string]().Title("Category").Options(opts...).Value(&catSelection))
		} else {
			fields2 = append(fields2, huh.NewInput().Title("Category").Value(&updateCategory))
		}

		if len(diffs) > 0 {
			opts := make([]huh.Option[string], len(diffs)+1)
			for i, d := range diffs {
				opts[i] = huh.NewOption(d.Name, d.Name)
			}
			opts[len(diffs)] = huh.NewOption("Other (type custom)…", customSentinel)
			fields2 = append(fields2, huh.NewSelect[string]().Title("Difficulty").Options(opts...).Value(&diffSelection))
		} else {
			fields2 = append(fields2, huh.NewInput().Title("Difficulty").Value(&updateDifficulty))
		}

		fields2 = append(fields2,
			huh.NewInput().Title("Points").Value(&pointsStr),
			huh.NewText().Title("Description (markdown)").Value(&updateDescription),
		)

		if err := huh.NewForm(huh.NewGroup(fields2...)).Run(); err != nil {
			return err
		}

		if len(cats) > 0 {
			if catSelection == customSentinel {
				if err := huh.NewForm(huh.NewGroup(huh.NewInput().Title("Custom category").Value(&updateCategory))).Run(); err != nil {
					return err
				}
			} else if catSelection != "" {
				updateCategory = catSelection
			}
		}
		if len(diffs) > 0 {
			if diffSelection == customSentinel {
				if err := huh.NewForm(huh.NewGroup(huh.NewInput().Title("Custom difficulty").Value(&updateDifficulty))).Run(); err != nil {
					return err
				}
			} else if diffSelection != "" {
				updateDifficulty = diffSelection
			}
		}

		if p, err := strconv.Atoi(pointsStr); err == nil {
			updatePoints = p
		}
	}

	ch, err := c.UpdateChallenge(id, updateTitle, updateCategory, updateDifficulty, updateDescription, updatePoints)
	if err != nil {
		return err
	}
	if quietOutput {
		fmt.Fprintln(os.Stdout, ch.ID)
		return nil
	}
	fmt.Fprintf(os.Stdout, "Updated challenge %s (%s)\n", ch.Title, ch.ID)
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
	if err := runChallengeGet(nil, []string{id}); err != nil {
		return err
	}
	return runSubmitLoop(c, id)
}

// runSubmitLoop prompts the user to pick a question and submit a flag,
// looping until they decline. Silently returns if stdin is not a TTY.
func runSubmitLoop(c *client.Client, challengeID string) error {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return nil
	}

	_, questions, err := c.GetChallengeWithQuestions(challengeID)
	if err != nil {
		return err
	}
	if len(questions) == 0 {
		fmt.Fprintln(os.Stdout, tui.MutedStyle.Render("No questions available."))
		return nil
	}

	for {
		// Pick a question (skip picker if only one).
		questionID := questions[0].ID
		questionName := questions[0].Name
		questionSolved := questions[0].Solved
		if len(questions) > 1 {
			opts := make([]huh.Option[string], len(questions))
			for i, q := range questions {
				prefix := "  "
				if q.Solved {
					prefix = "✓ "
				}
				label := fmt.Sprintf("%s%s  (%s, %d pts)", prefix, q.Name, q.FlagMask, q.Points)
				opts[i] = huh.NewOption(label, q.ID)
			}
			if err := huh.NewForm(huh.NewGroup(
				huh.NewSelect[string]().
					Title("Which question?").
					Options(opts...).
					Value(&questionID),
			)).Run(); err != nil {
				return err
			}
			for _, q := range questions {
				if q.ID == questionID {
					questionName = q.Name
					questionSolved = q.Solved
					break
				}
			}
		}

		// Offer hints if any are available.
		hints, err := c.GetHints(questionID)
		if err == nil && len(hints) > 0 {
			var viewHints bool
			_ = huh.NewForm(huh.NewGroup(
				huh.NewConfirm().
					Title(fmt.Sprintf("View hints? (%d available)", len(hints))).
					Value(&viewHints),
			)).Run()
			if viewHints {
				for _, h := range hints {
					if h.Unlocked {
						fmt.Fprintf(os.Stdout, "  Hint %d (-%d pts): %s\n", h.Order, h.Cost, h.Content)
					} else {
						fmt.Fprintf(os.Stdout, "  Hint %d (-%d pts): [locked]\n", h.Order, h.Cost)
						var unlock bool
						_ = huh.NewForm(huh.NewGroup(
							huh.NewConfirm().
								Title(fmt.Sprintf("Unlock hint %d for %d points?", h.Order, h.Cost)).
								Value(&unlock),
						)).Run()
						if unlock {
							if err := c.UnlockHint(h.ID); err != nil {
								fmt.Fprintln(os.Stderr, tui.ErrorStyle.Render(fmt.Sprintf("Unlock error: %v", err)))
							} else {
								// Re-fetch to get the content.
								refreshed, rerr := c.GetHints(questionID)
								if rerr == nil {
									for _, rh := range refreshed {
										if rh.ID == h.ID {
											fmt.Fprintf(os.Stdout, "  Hint %d: %s\n", rh.Order, rh.Content)
											break
										}
									}
								}
							}
						}
					}
				}
			}
		}

		// Already solved: show actual flag read-only, don't allow re-submission.
		if questionSolved {
			flag, err := c.GetQuestionSolution(questionID)
			if err == nil {
				fmt.Fprintln(os.Stdout, tui.SuccessStyle.Render(fmt.Sprintf("✓ %s", questionName)))
				fmt.Fprintln(os.Stdout, tui.MutedStyle.Render("Flag: ")+flag)
			} else {
				fmt.Fprintln(os.Stdout, tui.SuccessStyle.Render(fmt.Sprintf("✓ %q is already solved.", questionName)))
			}
			var again bool
			if err := huh.NewForm(huh.NewGroup(
				huh.NewConfirm().
					Title("View another question?").
					Value(&again),
			)).Run(); err != nil || !again {
				return err
			}
			continue
		}

		// Prompt for flag.
		var flag string
		if err := huh.NewForm(huh.NewGroup(
			huh.NewInput().
				Title(fmt.Sprintf("Flag for %q", questionName)).
				Placeholder("flag{...}").
				Value(&flag),
		)).Run(); err != nil {
			return err
		}

		if flag == "" {
			return nil
		}

		// Submit.
		result, err := c.SubmitFlag(questionID, flag)
		if err != nil {
			fmt.Fprintln(os.Stderr, tui.ErrorStyle.Render(fmt.Sprintf("Submit error: %v", err)))
		} else if result.Correct {
			fmt.Fprintln(os.Stdout, tui.SuccessStyle.Render("✓ Correct!"))
		} else {
			fmt.Fprintln(os.Stdout, tui.ErrorStyle.Render("✗ Incorrect, try again"))
		}

		// Ask to continue.
		var again bool
		if err := huh.NewForm(huh.NewGroup(
			huh.NewConfirm().
				Title("Submit another flag?").
				Value(&again),
		)).Run(); err != nil || !again {
			return err
		}
	}
}
