package cmd

import (
	"fmt"

	"github.com/kaitencloud/cli/internal/output"
	"github.com/kaitencloud/sdk-go"
	"github.com/spf13/cobra"
)

func newFeatureFlagsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "feature-flags",
		Short: "Manage feature flags",
		Args:  cobra.NoArgs,
		RunE:  runHelp,
	}

	cmd.AddCommand(newFeatureFlagsListCommand())
	cmd.AddCommand(newFeatureFlagsGetCommand())
	cmd.AddCommand(newFeatureFlagsCreateCommand())
	cmd.AddCommand(newFeatureFlagsUpdateCommand())
	cmd.AddCommand(newFeatureFlagsDeleteCommand())

	return cmd
}

func newFeatureFlagsListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List feature flags",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			items, err := client.FeatureFlags.List(ctx)
			if err != nil {
				return err
			}
			rows := make([][]string, 0, len(items))
			for _, item := range items {
				rows = append(rows, []string{item.Name, deref(item.Slug), string(item.Type), item.EventName})
			}
			return writeStructured(cmd, output.FormatTable, items, output.Table{
				Columns: []string{"NAME", "SLUG", "TYPE", "EVENT"},
				Rows:    rows,
			})
		},
	}
}

func newFeatureFlagsGetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "get <feature-flag-slug>",
		Short: "Get a feature flag",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			item, err := client.FeatureFlags.Get(ctx, args[0])
			if err != nil {
				return err
			}
			return writeStructured(cmd, output.FormatYAML, item, output.Table{})
		},
	}
}

func newFeatureFlagsCreateCommand() *cobra.Command {
	var file, payload string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a feature flag from --file or --payload",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			inputValue, err := resolveFileOrPayload[sdk.FeatureFlag](file, payload)
			if err != nil {
				return err
			}
			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			item, err := client.FeatureFlags.Create(ctx, inputValue)
			if err != nil {
				return err
			}
			return writeStructured(cmd, output.FormatYAML, item, output.Table{})
		},
	}
	addInputSourceFlags(cmd, &file, &payload, "feature flag")
	return cmd
}

func newFeatureFlagsUpdateCommand() *cobra.Command {
	var file, payload string
	cmd := &cobra.Command{
		Use:   "update <feature-flag-slug>",
		Short: "Update a feature flag from --file or --payload",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			inputValue, err := resolveFileOrPayload[sdk.FeatureFlag](file, payload)
			if err != nil {
				return err
			}
			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			if err := client.FeatureFlags.Update(ctx, args[0], inputValue); err != nil {
				return err
			}
			return writeMessage(cmd, "Feature flag updated")
		},
	}
	addInputSourceFlags(cmd, &file, &payload, "feature flag")
	return cmd
}

func newFeatureFlagsDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <feature-flag-slug>",
		Short: "Delete a feature flag",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			if err := confirmDestructive(cmd, "delete", fmt.Sprintf("feature flag %q", args[0])); err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			if err := client.FeatureFlags.Delete(ctx, args[0]); err != nil {
				return err
			}
			return writeMessage(cmd, "Feature flag deleted")
		},
	}
	addConfirmFlag(cmd)
	return cmd
}
