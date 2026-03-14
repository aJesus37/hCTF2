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

// --- category ---

var categoryCmd = &cobra.Command{
	Use:     "category",
	Short:   "Manage challenge categories (admin)",
	Aliases: []string{"cat"},
}

var categoryListCmd = &cobra.Command{
	Use:   "list",
	Short: "List categories",
	RunE:  runCategoryList,
}

var categoryCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a category (admin)",
	RunE:  runCategoryCreate,
}

var categoryDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a category (admin)",
	Args:  cobra.ExactArgs(1),
	RunE:  runCategoryDelete,
}

var (
	catName  string
	catOrder int
)

// --- difficulty ---

var difficultyCmd = &cobra.Command{
	Use:     "difficulty",
	Short:   "Manage challenge difficulties (admin)",
	Aliases: []string{"diff"},
}

var difficultyListCmd = &cobra.Command{
	Use:   "list",
	Short: "List difficulties",
	RunE:  runDifficultyList,
}

var difficultyCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a difficulty (admin)",
	RunE:  runDifficultyCreate,
}

var difficultyDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a difficulty (admin)",
	Args:  cobra.ExactArgs(1),
	RunE:  runDifficultyDelete,
}

var (
	diffName  string
	diffOrder int
)

func init() {
	rootCmd.AddCommand(categoryCmd)
	categoryCmd.AddCommand(categoryListCmd, categoryCreateCmd, categoryDeleteCmd)
	categoryCreateCmd.Flags().StringVar(&catName, "name", "", "Category name")
	categoryCreateCmd.Flags().IntVar(&catOrder, "order", 0, "Sort order")

	rootCmd.AddCommand(difficultyCmd)
	difficultyCmd.AddCommand(difficultyListCmd, difficultyCreateCmd, difficultyDeleteCmd)
	difficultyCreateCmd.Flags().StringVar(&diffName, "name", "", "Difficulty name")
	difficultyCreateCmd.Flags().IntVar(&diffOrder, "order", 0, "Sort order")
}

func runCategoryList(_ *cobra.Command, _ []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}
	cats, err := c.ListCategories()
	if err != nil {
		return err
	}
	if jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(cats)
	}
	cols := []tui.Column{
		{Header: "ID", Width: 34},
		{Header: "NAME", Width: 20},
		{Header: "ORDER", Width: 6},
	}
	var rows [][]string
	for _, cat := range cats {
		rows = append(rows, []string{tui.Truncate(cat.ID, 33), tui.Truncate(cat.Name, 19), strconv.Itoa(cat.SortOrder)})
	}
	tui.PrintTable(os.Stdout, cols, rows)
	return nil
}

func runCategoryCreate(_ *cobra.Command, _ []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}
	if term.IsTerminal(int(os.Stdin.Fd())) && catName == "" {
		if err := huh.NewForm(huh.NewGroup(
			huh.NewInput().Title("Category name").Value(&catName),
		)).Run(); err != nil {
			return err
		}
	}
	if catName == "" {
		return fmt.Errorf("--name is required")
	}
	cat, err := c.CreateCategory(catName, catOrder)
	if err != nil {
		return err
	}
	if !quietOutput {
		fmt.Fprintf(os.Stdout, "Created category %q\n", cat.Name)
	}
	return nil
}

func runCategoryDelete(_ *cobra.Command, args []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}
	if err := c.DeleteCategory(args[0]); err != nil {
		return err
	}
	if !quietOutput {
		fmt.Fprintf(os.Stdout, "Deleted category %s\n", args[0])
	}
	return nil
}

func runDifficultyList(_ *cobra.Command, _ []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}
	diffs, err := c.ListDifficulties()
	if err != nil {
		return err
	}
	if jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(diffs)
	}
	cols := []tui.Column{
		{Header: "ID", Width: 34},
		{Header: "NAME", Width: 20},
		{Header: "ORDER", Width: 6},
	}
	var rows [][]string
	for _, d := range diffs {
		rows = append(rows, []string{tui.Truncate(d.ID, 33), tui.Truncate(d.Name, 19), strconv.Itoa(d.SortOrder)})
	}
	tui.PrintTable(os.Stdout, cols, rows)
	return nil
}

func runDifficultyCreate(_ *cobra.Command, _ []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}
	if term.IsTerminal(int(os.Stdin.Fd())) && diffName == "" {
		if err := huh.NewForm(huh.NewGroup(
			huh.NewInput().Title("Difficulty name").Value(&diffName),
		)).Run(); err != nil {
			return err
		}
	}
	if diffName == "" {
		return fmt.Errorf("--name is required")
	}
	d, err := c.CreateDifficulty(diffName, diffOrder)
	if err != nil {
		return err
	}
	if !quietOutput {
		fmt.Fprintf(os.Stdout, "Created difficulty %q\n", d.Name)
	}
	return nil
}

func runDifficultyDelete(_ *cobra.Command, args []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}
	if err := c.DeleteDifficulty(args[0]); err != nil {
		return err
	}
	if !quietOutput {
		fmt.Fprintf(os.Stdout, "Deleted difficulty %s\n", args[0])
	}
	return nil
}
