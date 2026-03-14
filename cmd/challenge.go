package cmd

import (
	"encoding/json"
	"fmt"
	"io"
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
var challengeExportCmd = &cobra.Command{Use: "export", Short: "Export all challenges to JSON (admin)", RunE: runChallengeExport}
var challengeImportCmd = &cobra.Command{Use: "import <file.json>", Short: "Import challenges from JSON file (admin)", Args: cobra.ExactArgs(1), RunE: runChallengeImport}

var exportOutput string

var (
	createTitle       string
	createCategory    string
	createDifficulty  string
	createDescription string
	createPoints      int
	createVisible     bool
	createMinPoints   int
	createDecay       int

	updateTitle       string
	updateCategory    string
	updateDifficulty  string
	updateDescription string
	updatePoints      int
	updateVisible     bool
	updateMinPoints   int
	updateDecay       int
)

func init() {
	rootCmd.AddCommand(challengeCmd)
	challengeCmd.AddCommand(challengeListCmd, challengeGetCmd, challengeCreateCmd, challengeDeleteCmd, challengeBrowseCmd, challengeUpdateCmd, challengeExportCmd, challengeImportCmd)
	challengeExportCmd.Flags().StringVar(&exportOutput, "output", "", "Write JSON to file instead of stdout")
	challengeCreateCmd.Flags().StringVar(&createTitle, "title", "", "Challenge title")
	challengeCreateCmd.Flags().StringVar(&createCategory, "category", "", "Category")
	challengeCreateCmd.Flags().StringVar(&createDifficulty, "difficulty", "", "Difficulty")
	challengeCreateCmd.Flags().StringVar(&createDescription, "description", "", "Description (markdown)")
	challengeCreateCmd.Flags().IntVar(&createPoints, "points", 100, "Point value")
	challengeCreateCmd.Flags().BoolVar(&createVisible, "visible", false, "Make challenge visible")
	challengeCreateCmd.Flags().IntVar(&createMinPoints, "min-points", 0, "Minimum points (0 = disabled)")
	challengeCreateCmd.Flags().IntVar(&createDecay, "decay", 0, "Decay threshold (0 = disabled)")

	challengeUpdateCmd.Flags().StringVar(&updateTitle, "title", "", "Challenge title")
	challengeUpdateCmd.Flags().StringVar(&updateCategory, "category", "", "Category")
	challengeUpdateCmd.Flags().StringVar(&updateDifficulty, "difficulty", "", "Difficulty")
	challengeUpdateCmd.Flags().StringVar(&updateDescription, "description", "", "Description (markdown)")
	challengeUpdateCmd.Flags().IntVar(&updatePoints, "points", 0, "Point value")
	challengeUpdateCmd.Flags().BoolVar(&updateVisible, "visible", false, "Make challenge visible")
	challengeUpdateCmd.Flags().IntVar(&updateMinPoints, "min-points", 0, "Minimum points (0 = disabled)")
	challengeUpdateCmd.Flags().IntVar(&updateDecay, "decay", 0, "Decay threshold (0 = disabled)")
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
		{Header: "ID", Width: 38},
		{Header: "TITLE", Width: 30},
		{Header: "CATEGORY", Width: 16},
		{Header: "DIFF", Width: 12},
		{Header: "PTS", Width: 6},
	}
	var rows [][]string
	for _, ch := range challenges {
		rows = append(rows, []string{
			ch.ID,
			tui.Truncate(ch.Title, 29),
			tui.Truncate(ch.Category, 15),
			tui.Truncate(ch.Difficulty, 11),
			strconv.Itoa(ch.InitialPoints),
		})
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

// catDiffDesc returns a suggestion string like "(e.g. web, crypto, pwn)" from server options.
func catDiffDesc(names []string) string {
	if len(names) == 0 {
		return ""
	}
	s := "Options: "
	for i, n := range names {
		if i > 0 {
			s += ", "
		}
		s += n
	}
	return s
}

// buildChallengeGroups constructs one huh group per field so each gets its own page
// (label always visible) and back-navigation works within the single form.
func buildChallengeGroups(
	cats []client.Category, diffs []client.Difficulty,
	title, category, difficulty, description, pointsStr *string,
	visible *bool, minPointsStr, decayStr *string,
) []*huh.Group {
	catNames := make([]string, len(cats))
	for i, c := range cats {
		catNames[i] = c.Name
	}
	diffNames := make([]string, len(diffs))
	for i, d := range diffs {
		diffNames[i] = d.Name
	}

	return []*huh.Group{
		huh.NewGroup(huh.NewInput().Title("Title").Value(title)),
		huh.NewGroup(huh.NewInput().Title("Category").Description(catDiffDesc(catNames)).Value(category)),
		huh.NewGroup(huh.NewInput().Title("Difficulty").Description(catDiffDesc(diffNames)).Value(difficulty)),
		huh.NewGroup(huh.NewInput().Title("Points").Value(pointsStr)),
		huh.NewGroup(huh.NewText().Title("Description (markdown)").Value(description)),
		huh.NewGroup(huh.NewConfirm().Title("Visible?").Value(visible)),
		huh.NewGroup(huh.NewInput().Title("Minimum points (0 = disabled)").Value(minPointsStr)),
		huh.NewGroup(huh.NewInput().Title("Decay threshold (0 = disabled)").Value(decayStr)),
	}
}

func runChallengeCreate(_ *cobra.Command, _ []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}
	if term.IsTerminal(int(os.Stdin.Fd())) && (createTitle == "" || createCategory == "") {
		pointsStr := strconv.Itoa(createPoints)
		minPointsStr := strconv.Itoa(createMinPoints)
		decayStr := strconv.Itoa(createDecay)
		cats, _ := c.ListCategories()
		diffs, _ := c.ListDifficulties()

		groups := buildChallengeGroups(cats, diffs,
			&createTitle, &createCategory, &createDifficulty, &createDescription, &pointsStr,
			&createVisible, &minPointsStr, &decayStr)

		if err := huh.NewForm(groups...).Run(); err != nil {
			return err
		}

		if p, err := strconv.Atoi(pointsStr); err == nil {
			createPoints = p
		}
		if mp, err := strconv.Atoi(minPointsStr); err == nil {
			createMinPoints = mp
		}
		if d, err := strconv.Atoi(decayStr); err == nil {
			createDecay = d
		}
	}
	ch, err := c.CreateChallenge(createTitle, createCategory, createDifficulty, createDescription, createPoints, createVisible, createMinPoints, createDecay)
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
		updateVisible = ch.Visible
		if updateMinPoints == 0 {
			updateMinPoints = ch.MinimumPoints
		}
		if updateDecay == 0 {
			updateDecay = ch.DecayThreshold
		}

		cats, _ := c.ListCategories()
		diffs, _ := c.ListDifficulties()

		pointsStr := strconv.Itoa(updatePoints)
		minPointsStr := strconv.Itoa(updateMinPoints)
		decayStr := strconv.Itoa(updateDecay)
		groups := buildChallengeGroups(cats, diffs,
			&updateTitle, &updateCategory, &updateDifficulty, &updateDescription, &pointsStr,
			&updateVisible, &minPointsStr, &decayStr)

		if err := huh.NewForm(groups...).Run(); err != nil {
			return err
		}

		if p, err := strconv.Atoi(pointsStr); err == nil {
			updatePoints = p
		}
		if mp, err := strconv.Atoi(minPointsStr); err == nil {
			updateMinPoints = mp
		}
		if d, err := strconv.Atoi(decayStr); err == nil {
			updateDecay = d
		}
	}

	ch, err := c.UpdateChallenge(id, updateTitle, updateCategory, updateDifficulty, updateDescription, updatePoints, updateVisible, updateMinPoints, updateDecay)
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
	ok, err := confirmIfTTY(fmt.Sprintf("Delete challenge %s? This cannot be undone.", args[0]))
	if err != nil {
		return err
	}
	if !ok {
		abortedMsg()
		return nil
	}
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

func runChallengeExport(_ *cobra.Command, _ []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}
	data, err := c.ExportChallenges()
	if err != nil {
		return err
	}
	if exportOutput != "" {
		if err := os.WriteFile(exportOutput, data, 0644); err != nil {
			return err
		}
		if !quietOutput {
			// Count challenges by parsing the JSON array.
			var items []json.RawMessage
			_ = json.Unmarshal(data, &items)
			fmt.Fprintf(os.Stderr, "Exported %d challenges to %s\n", len(items), exportOutput)
		}
		return nil
	}
	// Print to stdout (pipeable).
	_, err = os.Stdout.Write(data)
	if err != nil {
		return err
	}
	if !quietOutput {
		var items []json.RawMessage
		_ = json.Unmarshal(data, &items)
		fmt.Fprintf(os.Stderr, "Exported %d challenges\n", len(items))
	}
	return nil
}

func runChallengeImport(_ *cobra.Command, args []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}
	var data []byte
	if args[0] == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(args[0])
	}
	if err != nil {
		return err
	}
	if err := c.ImportChallenges(data); err != nil {
		return err
	}
	if !quietOutput {
		fmt.Fprintln(os.Stdout, "Imported successfully")
	}
	return nil
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
		questionFlagMask := questions[0].FlagMask
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
					questionFlagMask = q.FlagMask
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
				Placeholder(questionFlagMask).
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
			// Refresh questions so the solved indicator updates on the next loop.
			if _, refreshed, rerr := c.GetChallengeWithQuestions(challengeID); rerr == nil {
				questions = refreshed
			}
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
