package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/ajesus37/hCTF2/internal/client"
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
	Use:   "create [name]",
	Short: "Create a category (admin)",
	Args:  cobra.MaximumNArgs(1),
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
	Use:   "create [name]",
	Short: "Create a difficulty (admin)",
	Args:  cobra.MaximumNArgs(1),
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

// settingItem is a generic representation of a category or difficulty row.
type settingItem struct {
	ID        string
	Name      string
	SortOrder int
}

// printSettingItems renders a table of setting items (categories or difficulties).
func printSettingItems(items []settingItem) {
	cols := []tui.Column{
		{Header: "ID", Width: 34},
		{Header: "NAME", Width: 20},
		{Header: "ORDER", Width: 6},
	}
	var rows [][]string
	for _, item := range items {
		rows = append(rows, []string{tui.Truncate(item.ID, 33), tui.Truncate(item.Name, 19), strconv.Itoa(item.SortOrder)})
	}
	tui.PrintTable(os.Stdout, cols, rows)
}

// runSettingList is a shared list handler for categories and difficulties.
// listFn calls the API and returns raw JSON-encodable items; toItem converts each to settingItem.
func runSettingList[T any](listFn func() ([]T, error), toItem func(T) settingItem) error {
	items, err := listFn()
	if err != nil {
		return err
	}
	if jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(items)
	}
	var rows []settingItem
	for _, item := range items {
		rows = append(rows, toItem(item))
	}
	printSettingItems(rows)
	return nil
}

// runSettingCreate is a shared create handler for categories and difficulties.
// name and order are pointers to the flag variables; label is "Category" or "Difficulty";
// createFn calls the API.
func runSettingCreate(args []string, name *string, order *int, label string, createFn func(*client.Client, string, int) (settingItem, error)) error {
	if len(args) > 0 {
		*name = args[0]
	}
	c, err := newClient()
	if err != nil {
		return err
	}
	if term.IsTerminal(int(os.Stdin.Fd())) && *name == "" {
		if err := huh.NewForm(huh.NewGroup(
			huh.NewInput().Title(label+" name").Value(name),
		)).Run(); err != nil {
			return err
		}
	}
	if *name == "" {
		return fmt.Errorf("--name is required")
	}
	item, err := createFn(c, *name, *order)
	if err != nil {
		return err
	}
	if !quietOutput {
		fmt.Fprintf(os.Stdout, "Created %s %q\n", label, item.Name)
	}
	return nil
}

// runSettingDelete is a shared delete handler for categories and difficulties.
func runSettingDelete(args []string, label string, deleteFn func(*client.Client, string) error) error {
	c, err := newClient()
	if err != nil {
		return err
	}
	if err := deleteFn(c, args[0]); err != nil {
		return err
	}
	if !quietOutput {
		fmt.Fprintf(os.Stdout, "Deleted %s %s\n", label, args[0])
	}
	return nil
}

func runCategoryList(_ *cobra.Command, _ []string) error {
	return runSettingList(
		func() ([]client.Category, error) {
			c, err := newClient()
			if err != nil {
				return nil, err
			}
			return c.ListCategories()
		},
		func(cat client.Category) settingItem {
			return settingItem{ID: cat.ID, Name: cat.Name, SortOrder: cat.SortOrder}
		},
	)
}

func runCategoryCreate(_ *cobra.Command, args []string) error {
	return runSettingCreate(args, &catName, &catOrder, "category", func(c *client.Client, name string, order int) (settingItem, error) {
		cat, err := c.CreateCategory(name, order)
		if err != nil {
			return settingItem{}, err
		}
		return settingItem{ID: cat.ID, Name: cat.Name, SortOrder: cat.SortOrder}, nil
	})
}

func runCategoryDelete(_ *cobra.Command, args []string) error {
	return runSettingDelete(args, "category", func(c *client.Client, id string) error {
		return c.DeleteCategory(id)
	})
}

func runDifficultyList(_ *cobra.Command, _ []string) error {
	return runSettingList(
		func() ([]client.Difficulty, error) {
			c, err := newClient()
			if err != nil {
				return nil, err
			}
			return c.ListDifficulties()
		},
		func(d client.Difficulty) settingItem {
			return settingItem{ID: d.ID, Name: d.Name, SortOrder: d.SortOrder}
		},
	)
}

func runDifficultyCreate(_ *cobra.Command, args []string) error {
	return runSettingCreate(args, &diffName, &diffOrder, "difficulty", func(c *client.Client, name string, order int) (settingItem, error) {
		d, err := c.CreateDifficulty(name, order)
		if err != nil {
			return settingItem{}, err
		}
		return settingItem{ID: d.ID, Name: d.Name, SortOrder: d.SortOrder}, nil
	})
}

func runDifficultyDelete(_ *cobra.Command, args []string) error {
	return runSettingDelete(args, "difficulty", func(c *client.Client, id string) error {
		return c.DeleteDifficulty(id)
	})
}
