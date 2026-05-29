package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"golang-build-watch/internal/build"
	"golang-build-watch/internal/config"
	"golang-build-watch/internal/render"
	"golang-build-watch/internal/target"
)

func NewRootCmd() *cobra.Command {
	var buildTarget string
	var allPackages bool
	var example bool
	var jsonOutput bool
	var debug bool
	var configPath string

	cmd := &cobra.Command{
		Use:           "gbwatch <module-root>",
		Short:         "Measure approximate build time per package for a Go module",
		Args:          rootArgs(&example),
		RunE:          newRunE(&buildTarget, &allPackages, &example, &jsonOutput, &debug, &configPath),
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.Flags().StringVar(&buildTarget, "target", ".", "build target relative to module root")
	cmd.Flags().StringVar(&configPath, "config", "", "path to a config file; defaults to ~/.gbwatch.toml")
	cmd.Flags().BoolVar(&allPackages, "all-packages", false, "include packages outside the target module in timing output")
	cmd.Flags().BoolVar(&example, "example", false, "print mock build timing output and exit")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "render build timing output as JSON")
	cmd.Flags().BoolVar(&debug, "debug", false, "enable debug logging")
	return cmd
}

func rootArgs(example *bool) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if *example {
			if len(args) > 1 {
				return errors.New("accepts at most one positional module-root argument in example mode")
			}
			return nil
		}
		if len(args) != 1 {
			return errors.New("requires exactly one module-root positional argument")
		}
		if strings.TrimSpace(args[0]) == "" {
			return errors.New("module-root cannot be empty")
		}
		return nil
	}
}

func newRunE(
	buildTarget *string,
	allPackages *bool,
	example *bool,
	jsonOutput *bool,
	debug *bool,
	configPath *string,
) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		moduleRoot := ""
		if len(args) > 0 {
			moduleRoot = args[0]
		}

		appCfg, err := config.Load(config.NewCLIOverrides(
			moduleRoot,
			*example,
			*configPath,
			flagString(cmd, "target", *buildTarget),
			flagBool(cmd, "all-packages", *allPackages),
			flagBool(cmd, "debug", *debug),
			outputFormatOverride(cmd, *jsonOutput),
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
		if _, err := fmt.Fprint(cmd.OutOrStdout(), output); err != nil {
			return err
		}
		if result.Stderr != "" {
			fmt.Fprintln(cmd.ErrOrStderr(), result.Stderr)
		}
		return result.Err
	}
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
	case config.OutputFormatJSON:
		return render.JSONRenderer{}, nil
	case config.OutputFormatTable:
		return render.TableRenderer{}, nil
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

func outputFormatOverride(cmd *cobra.Command, jsonOutput bool) *config.OutputFormat {
	if !cmd.Flags().Changed("json") {
		return nil
	}
	format := config.OutputFormatTable
	if jsonOutput {
		format = config.OutputFormatJSON
	}
	return &format
}
