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

const (
	autoReportValue         = "__auto__"
	defaultProgressPackages = 4
)

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
	var progressPackages int

	cmd := &cobra.Command{
		Use:           "gbwatch <module-root>",
		Short:         "Measure approximate build time per package for a Go module",
		Example:       "  gbwatch help format\n  gbwatch help format csv",
		Args:          rootArgs(&example),
		RunE:          newRunE(&buildTarget, &allPackages, &example, &debug, &configPath, &outputFormatRaw, &outputPath, &progressPackages),
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       buildVersion,
	}

	cmd.Flags().StringVarP(&buildTarget, "target", "t", "./...", "build target relative to module root")
	cmd.Flags().StringVar(&configPath, "config", "", "path to a config file; defaults to ~/.gbwatch.toml")
	cmd.Flags().StringVarP(&outputFormatRaw, "format", "f", "", "output format: pretty, json, or csv")
	cmd.Flags().StringVarP(&outputPath, "report-name", "r", "", "report filename or path; .json/.csv/.txt selects format, no value generates a name (default: stdout)")
	cmd.Flags().Lookup("report-name").NoOptDefVal = autoReportValue
	defaultHelp := cmd.HelpFunc()
	cmd.SetHelpFunc(func(c *cobra.Command, args []string) {
		flag := c.Flags().Lookup("report-name")
		if flag == nil {
			defaultHelp(c, args)
			return
		}
		noOptDefault := flag.NoOptDefVal
		flag.NoOptDefVal = ""
		defer func() { flag.NoOptDefVal = noOptDefault }()
		defaultHelp(c, args)
	})
	cmd.Flags().BoolVar(&allPackages, "all-packages", false, "include packages outside the target module in timing output")
	cmd.Flags().BoolVar(&example, "example", false, "print mock build timing output and exit")
	cmd.Flags().BoolVar(&debug, "debug", false, "enable debug logging")
	cmd.Flags().IntVar(&progressPackages, "progress-packages", defaultProgressPackages, fmt.Sprintf("package lines to show during a build (1-%d, including the active package)", build.MaxProgressPackages))
	help := newHelpCommand(cmd)
	cmd.AddCommand(help)
	cmd.SetHelpCommand(help)
	cmd.InitDefaultHelpFlag()
	cmd.InitDefaultVersionFlag()
	return cmd
}

func rootArgs(example *bool) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		_, normalizedArgs := normalizeOutputPathArgs(flagValue(cmd, "report-name"), args, *example)
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
	progressPackages *int,
) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if *progressPackages < 1 || *progressPackages > build.MaxProgressPackages {
			return fmt.Errorf("--progress-packages must be between 1 and %d", build.MaxProgressPackages)
		}
		outputPathValue, normalizedArgs := normalizeOutputPathArgs(*outputPath, args, *example)
		moduleRoot := ""
		if len(normalizedArgs) > 0 {
			moduleRoot = normalizedArgs[0]
		}

		formatOverride := outputFormatOverride(cmd, *outputFormatRaw)
		if formatOverride == nil && cmd.Flags().Changed("report-name") {
			if format, ok := config.OutputFormatFromReportName(outputPathValue); ok {
				formatOverride = &format
			}
		}
		appCfg, err := config.Load(config.NewCLIOverrides(
			moduleRoot,
			*example,
			*configPath,
			flagString(cmd, "target", *buildTarget),
			flagBool(cmd, "all-packages", *allPackages),
			flagBool(cmd, "debug", *debug),
			formatOverride,
			flagString(cmd, "report-name", outputPathValue),
		))
		if err != nil {
			return err
		}
		if inferred, ok := config.OutputFormatFromReportName(appCfg.Settings.OutputPath); ok && inferred != appCfg.Settings.OutputFormat {
			return fmt.Errorf("report name extension %q conflicts with output format %q", filepath.Ext(appCfg.Settings.OutputPath), appCfg.Settings.OutputFormat)
		}

		executor, targetValue, err := executorForConfig(cmd, appCfg)
		if err != nil {
			return err
		}

		renderer, err := rendererForConfig(appCfg)
		if err != nil {
			return err
		}

		outputPathTemplate, outputMode := resolveOutputPathTemplate(cmd, appCfg)

		result := runWithLoading(
			context.Background(),
			executor,
			targetValue,
			cmd.ErrOrStderr(),
			!appCfg.Example && !appCfg.Settings.Debug,
			*progressPackages,
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
			resolvedOutputPath := finalizeOutputPath(outputPathTemplate, appCfg.Settings.OutputFormat, defaultOutputModuleName(appCfg, targetValue), result)
			files := []reportFile{{extension: filepath.Ext(resolvedOutputPath), content: output}}
			if appCfg.Settings.OutputFormat == config.OutputFormatCSV {
				metadata, err := (render.MetadataJSONRenderer{}).Render(result)
				if err != nil {
					return err
				}
				files = []reportFile{{extension: ".json", content: metadata}, {extension: ".csv", content: output}}
			}
			writtenPaths, err := writeReportFiles(resolvedOutputPath, files)
			if err != nil {
				return err
			}
			for _, path := range writtenPaths {
				fmt.Fprintf(cmd.ErrOrStderr(), "wrote output file: %s\n", path)
			}
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

func normalizeOutputPathArgs(outputPath string, args []string, example bool) (string, []string) {
	moduleArgs := 1
	if example {
		moduleArgs = 0
	}
	if outputPath == autoReportValue && len(args) > moduleArgs {
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

func runWithLoading(ctx context.Context, executor build.Executor, t target.BuildTarget, w io.Writer, enabled bool, packageLines int) build.Result {
	if !enabled {
		return executor.Run(ctx, t)
	}

	done := make(chan build.Result, 1)
	updates := make(chan build.Progress, 1)
	go func() {
		if progressExecutor, ok := executor.(build.ProgressExecutor); ok {
			done <- progressExecutor.RunWithProgress(ctx, t, func(progress build.Progress) {
				select {
				case updates <- progress:
				default:
					select {
					case <-updates:
					default:
					}
					select {
					case updates <- progress:
					default:
					}
				}
			})
			return
		}
		done <- executor.Run(ctx, t)
	}()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	frames := []string{"|", "/", "-", `\`}
	index := 0
	display := newLoadingDisplay(w, t.BuildArg, packageLines)
	display.draw(frames[index])

	for {
		select {
		case result := <-done:
			select {
			case progress := <-updates:
				display.progress = progress
			default:
			}
			display.finish(result.Err)
			return result
		case progress := <-updates:
			display.progress = progress
			display.draw(frames[index%len(frames)])
		case <-ticker.C:
			index++
			display.draw(frames[index%len(frames)])
		}
	}
}

type loadingDisplay struct {
	w            io.Writer
	target       string
	packageLines int
	progress     build.Progress
	terminal     bool
	lines        int
	lastProgress string
	lastFrame    string
	lastRows     []string
	finished     bool
	failed       bool
}

func newLoadingDisplay(w io.Writer, target string, packageLines int) *loadingDisplay {
	display := &loadingDisplay{w: w, target: target, packageLines: packageLines}
	if file, ok := w.(*os.File); ok {
		if info, err := file.Stat(); err == nil {
			display.terminal = info.Mode()&os.ModeCharDevice != 0
		}
	}
	return display
}

func (d *loadingDisplay) draw(frame string) {
	rows := make([]string, 0, d.packageLines)
	completed := d.progress.Completed
	limit := d.packageLines - 1
	if d.finished {
		limit = d.packageLines
	}
	if len(completed) > limit {
		completed = completed[len(completed)-limit:]
	}
	for _, record := range completed {
		rows = append(rows, fmt.Sprintf("    %s  %s", record.Duration.Round(time.Millisecond), record.ImportPath))
	}
	if d.progress.Current != "" {
		if d.finished {
			rows = append(rows, "    "+d.progress.Current)
		} else {
			rows = append(rows, "  > "+d.progress.Current)
		}
	}
	header := fmt.Sprintf("[%s] building target %s", frame, d.target)
	if d.finished {
		header = fmt.Sprintf("[ok] built target %s", d.target)
		if d.failed {
			header = fmt.Sprintf("[failed] build target %s", d.target)
		}
	}
	if !d.terminal {
		progressText := strings.Join(rows, "\n")
		if d.finished {
			progressText = header + "\n" + progressText
		}
		if d.lines == 0 || progressText != d.lastProgress {
			fmt.Fprintln(d.w, strings.Join(append([]string{header}, rows...), "\n"))
		}
		d.lastProgress = progressText
		d.lines = len(rows) + 1
		return
	}

	var output strings.Builder
	if d.lines == 0 {
		fmt.Fprintln(&output, header)
		for range d.packageLines {
			output.WriteByte('\n')
		}
		d.lines = d.packageLines + 1
		d.lastRows = make([]string, d.packageLines)
		d.lastFrame = frame
	} else if d.finished {
		fmt.Fprintf(&output, "\x1b[%dA\r\x1b[2K%s\x1b[%dB\r", d.lines, header, d.lines)
	} else if frame != d.lastFrame {
		fmt.Fprintf(&output, "\x1b[%dA\r[%s]\x1b[%dB\r", d.lines, frame, d.lines)
		d.lastFrame = frame
	}
	for i := range d.lastRows {
		row := ""
		if i < len(rows) {
			row = rows[i]
		}
		if row == d.lastRows[i] {
			continue
		}
		distance := d.lines - i - 1
		fmt.Fprintf(&output, "\x1b[%dA\r\x1b[2K", distance)
		if row != "" {
			if i == len(rows)-1 && d.progress.Current != "" && !d.finished {
				output.WriteString("\x1b[1;36m")
			} else {
				output.WriteString("\x1b[2m")
			}
			output.WriteString(row)
			output.WriteString("\x1b[0m")
		}
		fmt.Fprintf(&output, "\x1b[%dB\r", distance)
		d.lastRows[i] = row
	}
	_, _ = io.WriteString(d.w, output.String())
}

func (d *loadingDisplay) finish(err error) {
	d.finished = true
	d.failed = err != nil
	d.draw("")
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

type reportFile struct {
	extension string
	content   string
}

func writeReportFiles(path string, reports []reportFile) ([]string, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create output directory: %w", err)
	}
	base := strings.TrimSuffix(path, filepath.Ext(path))
	for suffix := 1; ; suffix++ {
		candidateBase := base
		if suffix > 1 {
			candidateBase = fmt.Sprintf("%s-%d", base, suffix)
		}
		paths := make([]string, 0, len(reports))
		files := make([]*os.File, 0, len(reports))
		collision := false
		for _, report := range reports {
			candidate := candidateBase + report.extension
			file, err := os.OpenFile(candidate, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
			if errors.Is(err, os.ErrExist) {
				collision = true
				break
			}
			if err != nil {
				removeReportFiles(paths, files)
				return nil, fmt.Errorf("create output file: %w", err)
			}
			paths = append(paths, candidate)
			files = append(files, file)
		}
		if collision {
			removeReportFiles(paths, files)
			continue
		}
		for i, file := range files {
			if _, err := file.WriteString(reports[i].content); err != nil {
				removeReportFiles(paths, files)
				return nil, fmt.Errorf("write output file: %w", err)
			}
		}
		for i, file := range files {
			if err := file.Close(); err != nil {
				files[i] = nil
				removeReportFiles(paths, files)
				return nil, fmt.Errorf("close output file: %w", err)
			}
			files[i] = nil
		}
		return paths, nil
	}
}

func removeReportFiles(paths []string, files []*os.File) {
	for _, file := range files {
		if file != nil {
			_ = file.Close()
		}
	}
	for _, path := range paths {
		_ = os.Remove(path)
	}
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

func resolveOutputPathTemplate(cmd *cobra.Command, appCfg config.AppConfig) (string, outputPathMode) {
	if !cmd.Flags().Changed("report-name") && strings.TrimSpace(appCfg.Settings.OutputPath) == "" {
		return "", outputModeStdout
	}

	outputPath := strings.TrimSpace(appCfg.Settings.OutputPath)
	if outputPath == autoReportValue {
		return defaultOutputPathBase(appCfg), outputModeAuto
	}
	return normalizeExplicitOutputPathBase(outputPath, appCfg.Settings.OutputFormat), outputModeExplicit
}

func defaultOutputPathBase(appCfg config.AppConfig) string {
	filename := "gbwatch." + outputFileExtension(appCfg.Settings.OutputFormat)
	return filepath.Join(".", filename)
}

func defaultOutputModuleName(appCfg config.AppConfig, targetValue target.BuildTarget) string {
	if !appCfg.Example && strings.TrimSpace(targetValue.ModuleRoot) != "" {
		return sanitizeOutputSegment(filepath.Base(targetValue.ModuleRoot))
	}
	return "example"
}

func finalizeOutputPath(pathBase string, format config.OutputFormat, moduleName string, result build.Result) string {
	startTimestamp := outputTimestampNow().In(time.Local).Format("20060102-150405-0700")
	if !result.StartedAt.IsZero() {
		startTimestamp = result.StartedAt.In(time.Local).Format("20060102-150405-0700")
	}
	ext := filepath.Ext(pathBase)
	if format != config.OutputFormatPretty {
		ext = "." + outputFileExtension(format)
	}
	base := strings.TrimSuffix(pathBase, ext)
	return fmt.Sprintf("%s-%s-%s%s", base, moduleName, startTimestamp, ext)
}

func normalizeExplicitOutputPathBase(path string, format config.OutputFormat) string {
	base, ext := splitOutputPath(path, format)
	return base + ext
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
