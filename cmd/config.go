package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Export or import the full platform configuration",
}

var configExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export the full platform config (admin)",
	RunE:  runConfigExport,
}

var configImportCmd = &cobra.Command{
	Use:   "import <file>",
	Short: "Import a platform config file (admin)",
	Args:  cobra.ExactArgs(1),
	RunE:  runConfigImport,
}

var (
	configExportOutput string
	configExportFormat string
)

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configExportCmd, configImportCmd)
	configExportCmd.Flags().StringVarP(&configExportOutput, "output", "o", "", "Write to file instead of stdout")
	configExportCmd.Flags().StringVar(&configExportFormat, "format", "json", "Output format: json or yaml")
}

func runConfigExport(_ *cobra.Command, _ []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}
	data, err := c.ExportConfig()
	if err != nil {
		return err
	}

	// Convert to YAML if requested or if output file has yaml/yml extension.
	format := strings.ToLower(configExportFormat)
	if configExportOutput != "" {
		ext := strings.ToLower(filepath.Ext(configExportOutput))
		if ext == ".yaml" || ext == ".yml" {
			format = "yaml"
		}
	}
	if format == "yaml" {
		var obj interface{}
		if err := json.Unmarshal(data, &obj); err != nil {
			return fmt.Errorf("failed to parse JSON for YAML conversion: %w", err)
		}
		data, err = yaml.Marshal(obj)
		if err != nil {
			return fmt.Errorf("failed to encode YAML: %w", err)
		}
	}

	if configExportOutput != "" {
		if err := os.WriteFile(configExportOutput, data, 0644); err != nil {
			return err
		}
		if !quietOutput {
			fmt.Fprintf(os.Stdout, "Config exported to %s\n", configExportOutput)
		}
		return nil
	}
	_, err = os.Stdout.Write(data)
	return err
}

func runConfigImport(_ *cobra.Command, args []string) error {
	file := args[0]
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}

	// Auto-detect YAML by extension; convert to JSON for the API.
	ext := strings.ToLower(filepath.Ext(file))
	if ext == ".yaml" || ext == ".yml" {
		var obj interface{}
		if err := yaml.Unmarshal(data, &obj); err != nil {
			return fmt.Errorf("failed to parse YAML: %w", err)
		}
		data, err = json.Marshal(obj)
		if err != nil {
			return fmt.Errorf("failed to re-encode JSON: %w", err)
		}
	}

	c, err := newClient()
	if err != nil {
		return err
	}
	result, err := c.ImportConfig(data)
	if err != nil {
		return err
	}
	if jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(result)
	}
	if !quietOutput {
		fmt.Fprintf(os.Stdout, "Challenges imported: %d\n", result.ChallengesImported)
		fmt.Fprintf(os.Stdout, "Competitions created: %d\n", result.CompetitionsCreated)
		if len(result.Renamed) > 0 {
			fmt.Fprintf(os.Stdout, "Renamed: %s\n", strings.Join(result.Renamed, ", "))
		}
		if len(result.Errors) > 0 {
			fmt.Fprintf(os.Stderr, "Errors:\n")
			for _, e := range result.Errors {
				fmt.Fprintf(os.Stderr, "  - %s\n", e)
			}
		}
	}
	return nil
}
