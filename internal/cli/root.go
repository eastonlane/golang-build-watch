package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"golang-build-watch/internal/build"
	"golang-build-watch/internal/config"
	"golang-build-watch/internal/render"
	"golang-build-watch/internal/target"
)

const autoOutputPathValue = "__auto__"

var outputTimestampNow = time.Now

func NewRootCmd(buildVersion ...string) *cobra.Command {
	if len(buildVersion) == 0 {
		return newRootCmdWithVersion("")
	}
	return newRootCmdWithVersion(buildVersion[0])
}

func newRootCmdWithVersion(buildVersion string) *cobra.Command {
	var buildTarget string
	var allPackages bool
	var example bool
	var debug bool
	var configPath string
	var outputFormatRaw string
	var outputPath string

	cmd := &cobra.Command{
		Use:           "gbwatch <module-root>",
		Short:         "Measure approximate build time per package for a Go module",
		Args:          rootArgs(&example),
		RunE:          newRunE(&buildTarget, &allPackages, &example, &debug, &configPath, &outputFormatRaw, &outputPath),
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       buildVersion,
	}

	cmd.Flags().StringVarP(&buildTarget, "target", "t", "./...", "build target relative to module root")
	cmd.Flags().StringVar(&configPath, "config", "", "path to a config file; defaults to ~/.gbwatch.toml")
	cmd.Flags().StringVarP(&outputFormatRaw, "format", "f", "", "output format: pretty, json, toml, or csv")
	cmd.Flags().StringVarP(&outputPath, "output-path", "o", "", "write the rendered output to this file instead of stdout")
	cmd.Flags().Lookup("output-path").NoOptDefVal = autoOutputPathValue
	cmd.Flags().BoolVar(&allPackages, "all-packages", false, "include packages outside the target module in timing output")
	cmd.Flags().BoolVar(&example, "example", false, "print mock build timing output and exit")
	cmd.Flags().BoolVar(&debug, "debug", false, "enable debug logging")
	return cmd
}

func rootArgs(example *bool) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		_, normalizedArgs := normalizeOutputPathArgs(flagValue(cmd, "output-path"), args)
		if *example {
			if len(normalizedArgs) > 1 {
				return errors.New("accepts at most one positional module-root argument in example mode")
			}
			return nil
		}
		if len(normalizedArgs) != 1 {
			return errors.New("requires exactly one module-root positional argument")
		}
		if strings.TrimSpace(normalizedArgs[0]) == "" {
			return errors.New("module-root cannot be empty")
		}
		return nil
	}
}

func newRunE(
	buildTarget *string,
	allPackages *bool,
	example *bool,
	debug *bool,
	configPath *string,
	outputFormatRaw *string,
	outputPath *string,
) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		outputPathValue, normalizedArgs := normalizeOutputPathArgs(*outputPath, args)
		moduleRoot := ""
		if len(normalizedArgs) > 0 {
			moduleRoot = normalizedArgs[0]
		}

		appCfg, err := config.Load(config.NewCLIOverrides(
			moduleRoot,
			*example,
			*configPath,
			flagString(cmd, "target", *buildTarget),
			flagBool(cmd, "all-packages", *allPackages),
			flagBool(cmd, "debug", *debug),
			outputFormatOverride(cmd, *outputFormatRaw),
			flagString(cmd, "output-path", outputPathValue),
		))
		if err != nil {
			return err
		}

		executor, targetValue, err := executorForConfig(cmd, appCfg)
		if err != nil {
			return err
		}

		renderer, err := rendererForConfig(appCfg)
		if err != nil {
			return err
		}

		resolvedOutputPath, outputMode := resolveOutputPath(cmd, appCfg, targetValue)

		result := runWithLoading(
			context.Background(),
			executor,
			targetValue,
			cmd.ErrOrStderr(),
			!appCfg.Example && !appCfg.Settings.Debug,
		)

		output, err := renderer.Render(result)
		if err != nil {
			return err
		}
		if outputMode != outputModeStdout {
			if !shouldWriteOutputFile(result) {
				if err := printPrettyConsoleResult(cmd, result); err != nil {
					return err
				}
				if result.Stderr != "" {
					fmt.Fprintln(cmd.ErrOrStderr(), result.Stderr)
				}
				return result.Err
			}
			if err := writeOutputFile(resolvedOutputPath, output); err != nil {
				return err
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "wrote output file: %s\n", resolvedOutputPath)
		} else {
			if _, err := fmt.Fprint(cmd.OutOrStdout(), output); err != nil {
				return err
			}
		}
		if result.Stderr != "" {
			fmt.Fprintln(cmd.ErrOrStderr(), result.Stderr)
		}
		return result.Err
	}
}

func normalizeOutputPathArgs(outputPath string, args []string) (string, []string) {
	if outputPath == autoOutputPathValue && len(args) > 0 {
		return strings.TrimSpace(args[len(args)-1]), args[:len(args)-1]
	}
	return outputPath, args
}

func executorForConfig(cmd *cobra.Command, appCfg config.AppConfig) (build.Executor, target.BuildTarget, error) {
	if appCfg.Example {
		return build.ExampleRunner{
			IncludeAllPackages: appCfg.Settings.IncludeAllPackages,
		}, target.BuildTarget{}, nil
	}

	targetValue, err := target.Resolve(appCfg.ModuleRoot, appCfg.Settings.Target)
	if err != nil {
		return nil, target.BuildTarget{}, err
	}

	return build.Runner{
		IncludeAllPackages: appCfg.Settings.IncludeAllPackages,
		Logger:             debugLogger(cmd.ErrOrStderr(), appCfg.Settings.Debug),
	}, targetValue, nil
}

func rendererForConfig(appCfg config.AppConfig) (render.Renderer, error) {
	outputFormat := appCfg.Settings.OutputFormat
	switch outputFormat {
	case config.OutputFormatPretty:
		return render.TableRenderer{}, nil
	case config.OutputFormatJSON:
		return render.JSONRenderer{}, nil
	case config.OutputFormatTOML:
		return render.TOMLRenderer{}, nil
	case config.OutputFormatCSV:
		return render.CSVRenderer{}, nil
	default:
		return nil, fmt.Errorf("unsupported output format: %s", outputFormat)
	}
}

func debugLogger(w interface{ Write([]byte) (int, error) }, enabled bool) *slog.Logger {
	if !enabled {
		return nil
	}

	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
}

func runWithLoading(ctx context.Context, executor build.Executor, t target.BuildTarget, w io.Writer, enabled bool) build.Result {
	if !enabled {
		return executor.Run(ctx, t)
	}

	type runResult struct {
		result build.Result
	}

	done := make(chan runResult, 1)
	go func() {
		done <- runResult{result: executor.Run(ctx, t)}
	}()

	ticker := time.NewTicker(120 * time.Millisecond)
	defer ticker.Stop()

	frames := []string{"|", "/", "-", `\`}
	index := 0
	fmt.Fprintf(w, "\r[%s] building target %s", frames[index], t.BuildArg)
	index++

	for {
		select {
		case wrapped := <-done:
			fmt.Fprint(w, "\r                              \r")
			return wrapped.result
		case <-ticker.C:
			fmt.Fprintf(w, "\r[%s] building target %s", frames[index%len(frames)], t.BuildArg)
			index++
		}
	}
}

func flagBool(cmd *cobra.Command, name string, value bool) *bool {
	if !cmd.Flags().Changed(name) {
		return nil
	}
	v := value
	return &v
}

func flagString(cmd *cobra.Command, name, value string) *string {
	if !cmd.Flags().Changed(name) {
		return nil
	}
	v := value
	return &v
}

func flagValue(cmd *cobra.Command, name string) string {
	value, err := cmd.Flags().GetString(name)
	if err != nil {
		return ""
	}
	return value
}

func outputFormatOverride(cmd *cobra.Command, raw string) *config.OutputFormat {
	if cmd.Flags().Changed("format") {
		format, err := config.ParseOutputFormat(raw)
		if err != nil {
			invalid := config.OutputFormat(raw)
			return &invalid
		}
		return &format
	}
	return nil
}

func writeOutputFile(path string, output string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(output), 0o644); err != nil {
		_ = os.Remove(path)
		return fmt.Errorf("write output file: %w", err)
	}
	return nil
}

func shouldWriteOutputFile(result build.Result) bool {
	return result.Err == nil && len(result.Timing.Packages) > 0
}

func printPrettyConsoleResult(cmd *cobra.Command, result build.Result) error {
	output, err := (render.TableRenderer{}).Render(result)
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(cmd.ErrOrStderr(), output)
	return err
}

type outputPathMode int

const (
	outputModeStdout outputPathMode = iota
	outputModeAuto
	outputModeExplicit
)

func resolveOutputPath(cmd *cobra.Command, appCfg config.AppConfig, targetValue target.BuildTarget) (string, outputPathMode) {
	if !cmd.Flags().Changed("output-path") && strings.TrimSpace(appCfg.Settings.OutputPath) == "" {
		return "", outputModeStdout
	}

	outputPath := strings.TrimSpace(appCfg.Settings.OutputPath)
	if outputPath == autoOutputPathValue {
		return defaultOutputPath(appCfg, targetValue), outputModeAuto
	}
	return appendTimestampToPath(outputPath, appCfg.Settings.OutputFormat), outputModeExplicit
}

func defaultOutputPath(appCfg config.AppConfig, targetValue target.BuildTarget) string {
	moduleName := defaultOutputModuleName(appCfg, targetValue)
	targetName := formatOutputTargetSegment(defaultOutputTargetName(appCfg, targetValue))
	timestamp := outputTimestampNow().Format("20060102-150405")
	filename := fmt.Sprintf("gbwatch-%s-%s-%s.%s", moduleName, targetName, timestamp, outputFileExtension(appCfg.Settings.OutputFormat))
	return filepath.Join(".", filename)
}

func defaultOutputModuleName(appCfg config.AppConfig, targetValue target.BuildTarget) string {
	if !appCfg.Example && strings.TrimSpace(targetValue.ModuleRoot) != "" {
		return sanitizeOutputSegment(filepath.Base(targetValue.ModuleRoot))
	}
	return "example"
}

func defaultOutputTargetName(appCfg config.AppConfig, targetValue target.BuildTarget) string {
	if strings.TrimSpace(targetValue.BuildArg) != "" {
		return targetValue.BuildArg
	}
	if strings.TrimSpace(appCfg.Settings.Target) != "" {
		return appCfg.Settings.Target
	}
	return "./..."
}

func appendTimestampToPath(path string, format config.OutputFormat) string {
	timestamp := outputTimestampNow().Format("20060102-150405")
	base, ext := splitOutputPath(path, format)
	return base + "-" + timestamp + ext
}

func splitOutputPath(path string, format config.OutputFormat) (string, string) {
	if format == config.OutputFormatPretty {
		ext := filepath.Ext(path)
		return strings.TrimSuffix(path, ext), ext
	}

	expectedExt := "." + outputFileExtension(format)
	if strings.EqualFold(filepath.Ext(path), expectedExt) {
		return strings.TrimSuffix(path, filepath.Ext(path)), expectedExt
	}

	return strings.TrimSuffix(path, filepath.Ext(path)), expectedExt
}

func outputFileExtension(format config.OutputFormat) string {
	switch format {
	case config.OutputFormatJSON:
		return "json"
	case config.OutputFormatTOML:
		return "toml"
	case config.OutputFormatCSV:
		return "csv"
	default:
		return "txt"
	}
}

var invalidOutputPathChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func sanitizeOutputSegment(value string) string {
	trimmed := strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	trimmed = strings.TrimPrefix(trimmed, "./")
	trimmed = strings.Trim(trimmed, "/.")
	trimmed = invalidOutputPathChars.ReplaceAllString(trimmed, "-")
	trimmed = strings.Trim(trimmed, "-")
	if trimmed == "" {
		return "root"
	}
	return trimmed
}

func formatOutputTargetSegment(targetValue string) string {
	normalized := strings.TrimSpace(strings.ReplaceAll(targetValue, "\\", "/"))
	switch normalized {
	case "", ".", "./":
		return "root-package"
	case "...", "./...":
		return "all-packages"
	}

	if strings.HasPrefix(normalized, "./") {
		normalized = strings.TrimPrefix(normalized, "./")
	}

	replacer := strings.NewReplacer("/", "-", ".", "-")
	return sanitizeOutputSegment(replacer.Replace(normalized))
}
