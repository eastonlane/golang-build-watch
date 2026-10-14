package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type OutputFormat string

const (
	OutputFormatPretty OutputFormat = "pretty"
	OutputFormatJSON   OutputFormat = "json"
	OutputFormatCSV    OutputFormat = "csv"
)

type Settings struct {
	Target             string
	IncludeAllPackages bool
	Debug              bool
	OutputFormat       OutputFormat
	OutputPath         string
}

type AppConfig struct {
	ModuleRoot string
	Example    bool
	ConfigPath string
	Settings   Settings
}

type CLIOverrides struct {
	ModuleRoot         string
	Example            bool
	ConfigPath         string
	Target             *string
	IncludeAllPackages *bool
	Debug              *bool
	OutputFormat       *OutputFormat
	OutputPath         *string
}

func NewCLIOverrides(
	moduleRoot string,
	example bool,
	configPath string,
	target *string,
	includeAllPackages *bool,
	debug *bool,
	outputFormat *OutputFormat,
	outputPath *string,
) CLIOverrides {
	return CLIOverrides{
		ModuleRoot:         moduleRoot,
		Example:            example,
		ConfigPath:         configPath,
		Target:             target,
		IncludeAllPackages: includeAllPackages,
		Debug:              debug,
		OutputFormat:       outputFormat,
		OutputPath:         outputPath,
	}
}

type fileConfig struct {
	Settings fileSettings `toml:"settings"`
}

type fileSettings struct {
	Target             *string `toml:"target"`
	IncludeAllPackages *bool   `toml:"all_packages"`
	Debug              *bool   `toml:"debug"`
	OutputFormat       string  `toml:"output_format"`
	ReportName         *string `toml:"report_name"`
	OutputPath         *string `toml:"output_path"`
}

func Load(cli CLIOverrides) (AppConfig, error) {
	cfg := AppConfig{
		ModuleRoot: cli.ModuleRoot,
		Example:    cli.Example,
		Settings: Settings{
			Target:             "./...",
			IncludeAllPackages: false,
			Debug:              false,
			OutputFormat:       OutputFormatPretty,
		},
	}

	configPath := strings.TrimSpace(cli.ConfigPath)
	if configPath == "" {
		defaultPath, err := DefaultConfigPath()
		if err == nil {
			configPath = defaultPath
		}
	}
	cfg.ConfigPath = configPath

	if configPath != "" {
		if err := mergeFileConfig(&cfg, configPath); err != nil {
			return AppConfig{}, err
		}
	}

	if cli.Target != nil {
		cfg.Settings.Target = strings.TrimSpace(*cli.Target)
	}
	if cli.IncludeAllPackages != nil {
		cfg.Settings.IncludeAllPackages = *cli.IncludeAllPackages
	}
	if cli.Debug != nil {
		cfg.Settings.Debug = *cli.Debug
	}
	if cli.OutputFormat != nil {
		cfg.Settings.OutputFormat = *cli.OutputFormat
	}
	if cli.OutputPath != nil {
		cfg.Settings.OutputPath = strings.TrimSpace(*cli.OutputPath)
	}

	if cfg.Settings.Target == "" {
		cfg.Settings.Target = "./..."
	}
	if cfg.Settings.OutputFormat == "" {
		cfg.Settings.OutputFormat = OutputFormatPretty
	}

	return cfg, nil
}

func ParseOutputFormat(raw string) (OutputFormat, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "table", string(OutputFormatPretty):
		return OutputFormatPretty, nil
	case string(OutputFormatJSON):
		return OutputFormatJSON, nil
	case string(OutputFormatCSV):
		return OutputFormatCSV, nil
	default:
		return "", fmt.Errorf("unknown output format: %s", raw)
	}
}

func OutputFormatFromReportName(name string) (OutputFormat, bool) {
	switch strings.ToLower(filepath.Ext(strings.TrimSpace(name))) {
	case ".json":
		return OutputFormatJSON, true
	case ".csv":
		return OutputFormatCSV, true
	case ".txt":
		return OutputFormatPretty, true
	default:
		return "", false
	}
}

func DefaultConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".gbwatch.toml"), nil
}

func mergeFileConfig(cfg *AppConfig, path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("stat config file: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("config path must be a file: %s", path)
	}

	var raw fileConfig
	if _, err := toml.DecodeFile(path, &raw); err != nil {
		return fmt.Errorf("decode config file: %w", err)
	}

	if raw.Settings.Target != nil {
		cfg.Settings.Target = strings.TrimSpace(*raw.Settings.Target)
	}
	if raw.Settings.IncludeAllPackages != nil {
		cfg.Settings.IncludeAllPackages = *raw.Settings.IncludeAllPackages
	}
	if raw.Settings.Debug != nil {
		cfg.Settings.Debug = *raw.Settings.Debug
	}
	if raw.Settings.OutputFormat != "" {
		format, err := ParseOutputFormat(raw.Settings.OutputFormat)
		if err != nil {
			return err
		}
		cfg.Settings.OutputFormat = format
	}
	if raw.Settings.OutputPath != nil {
		cfg.Settings.OutputPath = strings.TrimSpace(*raw.Settings.OutputPath)
	}
	if raw.Settings.ReportName != nil {
		cfg.Settings.OutputPath = strings.TrimSpace(*raw.Settings.ReportName)
	}
	if raw.Settings.OutputFormat == "" {
		if format, ok := OutputFormatFromReportName(cfg.Settings.OutputPath); ok {
			cfg.Settings.OutputFormat = format
		}
	}

	return nil
}
