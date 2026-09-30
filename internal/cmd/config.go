package cmd

import (
	"fmt"

	"github.com/kaitencloud/cli/internal/config"
	"github.com/kaitencloud/cli/internal/output"
	"github.com/spf13/cobra"
)

// configKeys are the settings "config set" and "config unset" accept. They double as the
// shell-completion candidates for the key argument.
var configKeys = []string{"base-url", "auth-token", "output"}

func newConfigCommand() *cobra.Command {
	configCmd := &cobra.Command{
		Use:         "config",
		Short:       "Manage persisted Kaiten CLI configuration",
		Annotations: map[string]string{skipOutputValidationAnnotation: "true"},
		Args:        cobra.NoArgs,
		RunE:        runHelp,
	}

	configCmd.AddCommand(newConfigViewCommand())
	configCmd.AddCommand(newConfigSetCommand())
	configCmd.AddCommand(newConfigUnsetCommand())

	return configCmd
}

func newConfigViewCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "view",
		Short: "Show the persisted Kaiten CLI configuration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			fileCfg, path, err := config.Load()
			if err != nil {
				return err
			}

			resolved, err := runtimeConfig(cmd)
			if err != nil {
				return err
			}

			payload := struct {
				Path     string `json:"path" yaml:"path"`
				File     any    `json:"file" yaml:"file"`
				Resolved any    `json:"resolved" yaml:"resolved"`
			}{
				Path: path,
				File: struct {
					BaseURL   string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
					AuthToken string `json:"auth_token,omitempty" yaml:"auth_token,omitempty"`
					Output    string `json:"output,omitempty" yaml:"output,omitempty"`
				}{
					BaseURL:   fileCfg.BaseURL,
					AuthToken: config.MaskToken(fileCfg.AuthToken),
					Output:    fileCfg.Output,
				},
				Resolved: struct {
					BaseURL   string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
					AuthToken string `json:"auth_token,omitempty" yaml:"auth_token,omitempty"`
					Output    string `json:"output,omitempty" yaml:"output,omitempty"`
				}{
					BaseURL:   resolved.BaseURL,
					AuthToken: config.MaskToken(resolved.AuthToken),
					Output:    resolved.Output,
				},
			}

			format := resolved.Output
			if format != output.FormatJSON && format != output.FormatYAML {
				format = output.FormatYAML
			}
			return output.Write(cmd.OutOrStdout(), format, payload, output.Table{})
		},
	}
}

func newConfigSetCommand() *cobra.Command {
	return &cobra.Command{
		Use:       "set <key> <value>",
		Short:     "Persist a configuration value",
		Long:      "Persist a configuration value. The supported keys are base-url, auth-token and output.",
		Args:      cobra.ExactArgs(2),
		ValidArgs: configKeys,
		Example: `  kaiten config set base-url https://kaiten.example.com/api
  kaiten config set output json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := config.Load()
			if err != nil {
				return err
			}

			key := args[0]
			value := args[1]

			switch key {
			case "base-url":
				cfg.BaseURL = value
			case "auth-token":
				cfg.AuthToken = value
			case "output":
				if err := output.ValidateFormat(value); err != nil {
					return errUsage(err)
				}
				cfg.Output = value
			default:
				return errUsagef("unsupported config key %q", key)
			}

			path, err := config.Save(cfg)
			if err != nil {
				return err
			}

			return writeMessage(cmd, fmt.Sprintf("Saved %s in %s", key, path))
		},
	}
}

func newConfigUnsetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:       "unset <key>",
		Short:     "Remove a persisted configuration value",
		Long:      "Remove a persisted configuration value. The supported keys are base-url, auth-token and output.",
		Args:      cobra.ExactArgs(1),
		ValidArgs: configKeys,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, currentPath, err := config.Load()
			if err != nil {
				return err
			}

			key := args[0]
			switch key {
			case "base-url":
				cfg.BaseURL = ""
			case "auth-token":
				cfg.AuthToken = ""
			case "output":
				cfg.Output = ""
			default:
				return errUsagef("unsupported config key %q", key)
			}

			if err := confirmDestructive(cmd, "remove", fmt.Sprintf("%s from %s", key, currentPath)); err != nil {
				return err
			}

			path, err := config.Save(cfg)
			if err != nil {
				return err
			}

			return writeMessage(cmd, fmt.Sprintf("Removed %s from %s", key, path))
		},
	}
	addConfirmFlag(cmd)
	return cmd
}
