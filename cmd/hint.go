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

var hintCmd = &cobra.Command{Use: "hint", Short: "Manage question hints (admin)"}
var hintListCmd = &cobra.Command{
	Use:   "list <question-id>",
	Short: "List hints for a question",
	Args:  cobra.ExactArgs(1),
	RunE:  runHintList,
}
var hintCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a hint (admin)",
	RunE:  runHintCreate,
}
var hintDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a hint (admin)",
	Args:  cobra.ExactArgs(1),
	RunE:  runHintDelete,
}
var hintUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update a hint (admin)",
	Args:  cobra.ExactArgs(1),
	RunE:  runHintUpdate,
}

var (
	hintQuestionID string
	hintContent    string
	hintCost       int
	huContent      string
	huCost         int
	huOrder        int
)

func init() {
	rootCmd.AddCommand(hintCmd)
	hintCmd.AddCommand(hintListCmd, hintCreateCmd, hintDeleteCmd, hintUpdateCmd)
	hintCreateCmd.Flags().StringVar(&hintQuestionID, "question", "", "Question ID")
	hintCreateCmd.Flags().StringVar(&hintContent, "content", "", "Hint content")
	hintCreateCmd.Flags().IntVar(&hintCost, "cost", 0, "Point cost to unlock")

	hintUpdateCmd.Flags().StringVar(&huContent, "content", "", "Hint content")
	hintUpdateCmd.Flags().IntVar(&huCost, "cost", -1, "Point cost to unlock")
	hintUpdateCmd.Flags().IntVar(&huOrder, "order", -1, "Display order")
}

func runHintList(_ *cobra.Command, args []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}
	hints, err := c.GetHints(args[0])
	if err != nil {
		return err
	}
	if jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(hints)
	}
	cols := []tui.Column{
		{Header: "ID", Width: 10},
		{Header: "ORDER", Width: 6},
		{Header: "COST", Width: 6},
		{Header: "UNLOCKED", Width: 9},
		{Header: "CONTENT", Width: 40},
	}
	var rows [][]string
	for _, h := range hints {
		id := h.ID
		if len(id) > 8 {
			id = id[:8] + "..."
		}
		unlocked := "no"
		if h.Unlocked {
			unlocked = "yes"
		}
		content := h.Content
		if len(content) > 38 {
			content = content[:38] + "…"
		}
		rows = append(rows, []string{id, strconv.Itoa(h.Order), strconv.Itoa(h.Cost), unlocked, content})
	}
	tui.PrintTable(os.Stdout, cols, rows)
	return nil
}

func runHintCreate(_ *cobra.Command, _ []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}
	if term.IsTerminal(int(os.Stdin.Fd())) && hintContent == "" {
		costStr := strconv.Itoa(hintCost)
		if err := huh.NewForm(
			huh.NewGroup(huh.NewInput().Title("Question ID").Value(&hintQuestionID)),
			huh.NewGroup(huh.NewText().Title("Hint content").Value(&hintContent)),
			huh.NewGroup(huh.NewInput().Title("Point cost").Value(&costStr)),
		).Run(); err != nil {
			return err
		}
		if p, err := strconv.Atoi(costStr); err == nil {
			hintCost = p
		}
	}
	if hintQuestionID == "" || hintContent == "" {
		return fmt.Errorf("--question and --content are required")
	}
	h, err := c.CreateHint(hintQuestionID, hintContent, hintCost)
	if err != nil {
		return err
	}
	if quietOutput {
		fmt.Fprintln(os.Stdout, h.ID)
		return nil
	}
	fmt.Fprintf(os.Stdout, "Created hint %s (cost: %d pts)\n", h.ID, h.Cost)
	return nil
}

func runHintDelete(_ *cobra.Command, args []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}
	if err := c.DeleteHint(args[0]); err != nil {
		return err
	}
	if !quietOutput {
		fmt.Fprintf(os.Stdout, "Deleted hint %s\n", args[0])
	}
	return nil
}

func runHintUpdate(_ *cobra.Command, args []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}
	id := args[0]

	if term.IsTerminal(int(os.Stdin.Fd())) && huContent == "" {
		h, err := c.GetHint(id)
		if err != nil {
			return err
		}
		huContent = h.Content
		if huCost < 0 {
			huCost = h.Cost
		}
		if huOrder < 0 {
			huOrder = h.Order
		}

		costStr := strconv.Itoa(huCost)
		orderStr := strconv.Itoa(huOrder)
		if err := huh.NewForm(
			huh.NewGroup(huh.NewText().Title("Content").Value(&huContent)),
			huh.NewGroup(huh.NewInput().Title("Point cost").Value(&costStr)),
			huh.NewGroup(huh.NewInput().Title("Order").Value(&orderStr)),
		).Run(); err != nil {
			return err
		}
		if v, err := strconv.Atoi(costStr); err == nil {
			huCost = v
		}
		if v, err := strconv.Atoi(orderStr); err == nil {
			huOrder = v
		}
	}

	// On the non-TTY path, fetch current values for any fields still at sentinel.
	if huCost < 0 || huOrder < 0 || huContent == "" {
		h, err := c.GetHint(id)
		if err != nil {
			return err
		}
		if huContent == "" {
			huContent = h.Content
		}
		if huCost < 0 {
			huCost = h.Cost
		}
		if huOrder < 0 {
			huOrder = h.Order
		}
	}

	if err := c.UpdateHint(id, huContent, huCost, huOrder); err != nil {
		return err
	}
	if quietOutput {
		fmt.Fprintln(os.Stdout, id)
		return nil
	}
	fmt.Fprintf(os.Stdout, "Updated hint %s\n", id)
	return nil
}
