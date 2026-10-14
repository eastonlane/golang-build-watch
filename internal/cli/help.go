package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newHelpCommand(root *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:   "help [topic] [format]",
		Short: "Show help topics; try 'gbwatch help format'",
		Args:  cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return root.Help()
			}
			if args[0] == "format" {
				return printFormatHelp(cmd, args[1:])
			}
			if len(args) != 1 {
				return fmt.Errorf("unknown help topic %q; try gbwatch help format", args[0]+" "+args[1])
			}
			switch args[0] {
			case "pretty", "json", "csv":
				return printFormatHelp(cmd, args)
			case "report-name":
				_, err := fmt.Fprintln(cmd.OutOrStdout(), "--report-name, -r: write a report file. Bare -r generates a name; a value sets the base name. A .json, .csv, or .txt suffix selects the format; -f must agree if supplied. File names include the module name, local build start time, and numeric UTC offset.")
				return err
			case "target":
				_, err := fmt.Fprintln(cmd.OutOrStdout(), "--target: Go package or pattern to build relative to <module-root>; default ./....")
				return err
			case "config":
				_, err := fmt.Fprintln(cmd.OutOrStdout(), "--config: TOML config file; default ~/.gbwatch.toml. Use report_name and output_format under [settings].")
				return err
			case "example":
				_, err := fmt.Fprintln(cmd.OutOrStdout(), "--example: render sample timings without a module root.")
				return err
			default:
				return fmt.Errorf("unknown help topic %q; try gbwatch help format", args[0])
			}
		},
	}
}

func printFormatHelp(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), "Formats: pretty, json, csv. Use -f/--format to select one. A report name ending in .json, .csv, or .txt selects the matching format. For CSV file layout, run gbwatch help format csv.")
		return err
	}
	var description string
	switch args[0] {
	case "pretty":
		description = "pretty: human-readable report with build metadata and per-package durations. File mode writes one .txt report."
	case "json":
		description = "json: one .json report containing build metadata and package timing details in packages."
	case "csv":
		description = "csv: stdout contains package rows only. File mode writes two files with the same stem: .json contains build metadata without packages, and .csv contains package rows with import_path,duration_ms,duration_human. Both names include the module name, local build start time, and numeric UTC offset."
	default:
		return fmt.Errorf("unknown output format %q; use pretty, json, or csv", args[0])
	}
	_, err := fmt.Fprintln(cmd.OutOrStdout(), description)
	return err
}
