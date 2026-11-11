// Package cmd implements the kaiten CLI commands.
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/kaitencloud/cli/internal/config"
	"github.com/kaitencloud/cli/internal/output"
	"github.com/kaitencloud/sdk-go"
	"github.com/spf13/cobra"
)

const defaultTimeout = 15 * time.Second

// silenceUsageAfterValidation is the root command's persistent pre-run. cobra runs it once
// flag parsing and positional-argument validation have passed and immediately before the
// command body, which makes it the one place where "the invocation was wrong" turns into
// "the work failed". Everything after it is runtime, so usage is silenced: printing the
// whole usage block would only bury the error. exitCode reads the same signal to tell the
// two kinds of failure apart, so no other command may define a persistent pre-run — cobra
// runs only the innermost one, and this would then be skipped.
func silenceUsageAfterValidation(cmd *cobra.Command, args []string) error {
	if err := validateRootFlags(cmd, args); err != nil {
		return err
	}

	cmd.SilenceUsage = true
	return nil
}

// runHelp prints the command's help. Commands that only group subcommands use it so that
// cobra reaches their Args validator: it skips validation for commands it cannot run.
func runHelp(cmd *cobra.Command, _ []string) error {
	return cmd.Help()
}

func validateRootFlags(cmd *cobra.Command, _ []string) error {
	if skipsOutputValidation(cmd) {
		return nil
	}

	cfg, err := runtimeConfig(cmd)
	if err != nil {
		return err
	}
	return output.ValidateFormat(cfg.Output)
}

func runtimeConfig(cmd *cobra.Command) (config.Runtime, error) {
	baseURL, err := cmd.Flags().GetString("base-url")
	if err != nil {
		return config.Runtime{}, err
	}
	authToken, err := cmd.Flags().GetString("auth-token")
	if err != nil {
		return config.Runtime{}, err
	}
	format, err := cmd.Flags().GetString("output")
	if err != nil {
		return config.Runtime{}, err
	}

	return config.Resolve(baseURL, authToken, format)
}

// newClient builds an SDK client from the resolved configuration. extra options
// are applied after the CLI's own, so a command can replace the HTTP client.
func newClient(cmd *cobra.Command, extra ...sdk.Option) (*sdk.Client, config.Runtime, error) {
	cfg, err := runtimeConfig(cmd)
	if err != nil {
		return nil, config.Runtime{}, err
	}
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return nil, cfg, errUsagef("kaiten base URL is required; use --base-url, %s, or `kaiten config set base-url ...`", config.EnvBaseURL)
	}

	opts := make([]sdk.Option, 0, 2)
	if cfg.AuthToken != "" {
		opts = append(opts, sdk.WithBearerToken(cfg.AuthToken))
	}

	client, err := sdk.NewClient(cfg.BaseURL, append(opts, extra...)...)
	if err != nil {
		return nil, cfg, err
	}

	return client, cfg, nil
}

func commandContext(cmd *cobra.Command) (context.Context, context.CancelFunc) {
	return context.WithTimeout(cmd.Context(), defaultTimeout)
}

func writeStructured(cmd *cobra.Command, defaultFormat string, value any, table output.Table) error {
	cfg, err := runtimeConfig(cmd)
	if err != nil {
		return err
	}

	format := cfg.Output
	if format == "" {
		format = defaultFormat
	}
	if format == "" {
		format = output.FormatYAML
	}

	return output.Write(cmd.OutOrStdout(), format, value, table)
}

// writeMessage reports the outcome of a command that returns no payload: a bare line for table
// output, and a structured document carrying the same text for json and yaml, so that machine-
// readable output stays parseable for mutations too. Any other format value reaching here comes
// from a command that skips output validation, and is rendered as the bare line.
func writeMessage(cmd *cobra.Command, message string) error {
	cfg, err := runtimeConfig(cmd)
	if err != nil {
		return err
	}

	switch cfg.Output {
	case output.FormatJSON, output.FormatYAML:
		payload := struct {
			Message string `json:"message" yaml:"message"`
		}{
			Message: message,
		}
		return output.Write(cmd.OutOrStdout(), cfg.Output, payload, output.Table{})
	default:
		_, err := fmt.Fprintln(cmd.OutOrStdout(), message)
		return err
	}
}

func skipsOutputValidation(cmd *cobra.Command) bool {
	for current := cmd; current != nil; current = current.Parent() {
		if current.Annotations[skipOutputValidationAnnotation] == "true" {
			return true
		}
	}
	return false
}

// deref reads through an optional API field, rendering an absent one as its zero value.
// Table cells and inline flags both want "nothing" rather than a nil dereference.
func deref[T any](value *T) T {
	if value == nil {
		var zero T
		return zero
	}
	return *value
}

func formatTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.Format(time.RFC3339)
}

func formatTimeValue(value time.Time) string {
	return value.Format(time.RFC3339)
}

// renderUnion reads through a pointer to a oneOf value (an entitlement or license value
// discriminated by type) and renders it as the JSON it would send on the wire, since there
// is no single scalar representation across boolean, number and object values. A nil
// pointer -- no value configured -- renders as jsonString's own "".
func renderUnion[T any](value *T) string {
	if value == nil {
		return ""
	}
	return jsonString(*value)
}

func joinStrings(value *[]string) string {
	if value == nil {
		return ""
	}
	return strings.Join(*value, ",")
}

func jsonString(value any) string {
	if value == nil {
		return ""
	}

	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}

	return string(data)
}
