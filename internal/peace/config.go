package peace

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"io.github.sohanemon/peace/internal/config"
)

func newConfigCommand(_ Runtime) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Inspect and edit persistent peace settings",
		Long:  "Read and change the peace TOML configuration file. CLI flags and PEACE_* environment variables still override file values for playback commands.",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "path",
		Short: "Print the configuration file path",
		Args:  noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, err := config.Path()
			if err != nil {
				return fmt.Errorf("peace config path: %w", err)
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), path)
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "get KEY",
		Short: "Print one configuration value",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 {
				return &cliUsageError{err: errors.New("peace config get: key is required")}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("peace config get: %w", err)
			}
			value, err := config.Value(cfg, args[0])
			if err != nil {
				return fmt.Errorf("peace config get: %w", err)
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), value)
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "set KEY VALUE",
		Short: "Change one configuration value",
		Long:  "Change one configuration value. Values are validated before the file is replaced atomically.",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 2 {
				return &cliUsageError{err: errors.New("peace config set: key and value are required")}
			}
			return nil
		},
		RunE: func(_ *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("peace config set: %w", err)
			}
			if err := config.Assign(&cfg, args[0], args[1]); err != nil {
				return fmt.Errorf("peace config set: %w", err)
			}
			if err := config.Validate(cfg); err != nil {
				return fmt.Errorf("peace config set: %w", err)
			}
			if err := config.Save(cfg); err != nil {
				return fmt.Errorf("peace config set: %w", err)
			}
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "edit",
		Short: "Open the configuration file in $VISUAL or $EDITOR",
		Args:  noArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			path, err := config.Path()
			if err != nil {
				return fmt.Errorf("peace config edit: %w", err)
			}
			if _, statErr := os.Stat(path); errors.Is(statErr, os.ErrNotExist) {
				if err := config.Save(config.DefaultConfig()); err != nil {
					return fmt.Errorf("peace config edit: %w", err)
				}
			} else if statErr != nil {
				return fmt.Errorf("peace config edit: %w", statErr)
			}
			editor := strings.TrimSpace(os.Getenv("VISUAL"))
			if editor == "" {
				editor = strings.TrimSpace(os.Getenv("EDITOR"))
			}
			if editor == "" {
				editor = "vi"
			}
			fields := strings.Fields(editor)
			command := exec.Command(fields[0], append(fields[1:], path)...)
			command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
			if err := command.Run(); err != nil {
				return fmt.Errorf("peace config edit: %w", err)
			}
			if _, err := config.Load(); err != nil {
				return fmt.Errorf("peace config edit: %w", err)
			}
			return nil
		},
	})
	return cmd
}
