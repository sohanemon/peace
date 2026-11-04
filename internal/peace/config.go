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
			path, err := userConfigPath(cmd)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), path)
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "get KEY",
		Short: "Print one configuration value",
		Args:  exactUsageArgs(1, "key is required"),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadUserSettings(cmd)
			if err != nil {
				return err
			}
			value, err := config.Value(cfg, args[0])
			if err != nil {
				return fmt.Errorf("%s: %w", cmd.CommandPath(), err)
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), value)
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "set KEY VALUE",
		Short: "Change one configuration value",
		Long:  "Change one configuration value. Values are validated before the file is replaced atomically.",
		Args:  exactUsageArgs(2, "key and value are required"),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadUserSettings(cmd)
			if err != nil {
				return err
			}
			if err := config.Assign(&cfg, args[0], args[1]); err != nil {
				return fmt.Errorf("%s: %w", cmd.CommandPath(), err)
			}
			return saveUserSettings(cmd, cfg)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "edit",
		Short: "Open the configuration file in $VISUAL or $EDITOR",
		Args:  noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, err := userConfigPath(cmd)
			if err != nil {
				return err
			}
			if _, statErr := os.Stat(path); errors.Is(statErr, os.ErrNotExist) {
				if err := saveUserSettings(cmd, config.DefaultConfig()); err != nil {
					return err
				}
			} else if statErr != nil {
				return fmt.Errorf("%s: %w", cmd.CommandPath(), statErr)
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
				return fmt.Errorf("%s: %w", cmd.CommandPath(), err)
			}
			_, err = loadUserSettings(cmd)
			return err
		},
	})
	return cmd
}

func exactUsageArgs(count int, message string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) != count {
			return &cliUsageError{err: fmt.Errorf("%s: %s", cmd.CommandPath(), message)}
		}
		return nil
	}
}

func userConfigPath(cmd *cobra.Command) (string, error) {
	path, err := config.Path()
	if err != nil {
		return "", fmt.Errorf("%s: %w", cmd.CommandPath(), err)
	}
	return path, nil
}

func loadUserSettings(cmd *cobra.Command) (config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return config.Config{}, fmt.Errorf("%s: %w", cmd.CommandPath(), err)
	}
	return cfg, nil
}

func saveUserSettings(cmd *cobra.Command, cfg config.Config) error {
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("%s: %w", cmd.CommandPath(), err)
	}
	return nil
}
