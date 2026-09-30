package cmd

import (
	"fmt"

	"github.com/kaitencloud/cli/internal/output"
	"github.com/kaitencloud/sdk-go"
	"github.com/spf13/cobra"
)

func newEntitlementsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "entitlements",
		Short: "Manage entitlements",
		Args:  cobra.NoArgs,
		RunE:  runHelp,
	}

	cmd.AddCommand(newEntitlementsListCommand())
	cmd.AddCommand(newEntitlementsGetCommand())
	cmd.AddCommand(newEntitlementsCreateCommand())
	cmd.AddCommand(newEntitlementsUpdateCommand())
	cmd.AddCommand(newEntitlementsDeleteCommand())

	return cmd
}

func newEntitlementsListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List entitlements",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			items, err := client.Entitlements.List(ctx)
			if err != nil {
				return err
			}
			rows := make([][]string, 0, len(items))
			for _, item := range items {
				rows = append(rows, []string{item.Name, deref(item.Slug), string(deref(item.Type))})
			}
			return writeStructured(cmd, output.FormatTable, items, output.Table{
				Columns: []string{"NAME", "SLUG", "TYPE"},
				Rows:    rows,
			})
		},
	}
}

func newEntitlementsGetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "get <entitlement-slug>",
		Short: "Get an entitlement",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			item, err := client.Entitlements.Get(ctx, args[0])
			if err != nil {
				return err
			}
			return writeStructured(cmd, output.FormatYAML, item, output.Table{})
		},
	}
}

func newEntitlementsCreateCommand() *cobra.Command {
	var file, payload string
	var inline entitlementInputFlags
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an entitlement from --file, --payload, or inline flags",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			inputValue, err := resolveInput(file, payload, inline.changed(cmd), inline.build(cmd))
			if err != nil {
				return err
			}
			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			item, err := client.Entitlements.Create(ctx, inputValue)
			if err != nil {
				return err
			}
			return writeStructured(cmd, output.FormatYAML, item, output.Table{})
		},
	}
	addInputSourceFlags(cmd, &file, &payload, "entitlement")
	inline.register(cmd)
	return cmd
}

func newEntitlementsUpdateCommand() *cobra.Command {
	var file, payload string
	var inline entitlementInputFlags
	cmd := &cobra.Command{
		Use:   "update <entitlement-slug>",
		Short: "Update an entitlement from --file, --payload, or inline flags",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			inputValue, err := resolveInput(file, payload, inline.changed(cmd), inline.build(cmd))
			if err != nil {
				return err
			}
			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			if err := client.Entitlements.Update(ctx, args[0], inputValue); err != nil {
				return err
			}
			return writeMessage(cmd, "Entitlement updated")
		},
	}
	addInputSourceFlags(cmd, &file, &payload, "entitlement")
	inline.register(cmd)
	return cmd
}

func newEntitlementsDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <entitlement-slug>",
		Short: "Delete an entitlement",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			if err := confirmDestructive(cmd, "delete", fmt.Sprintf("entitlement %q", args[0])); err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			if err := client.Entitlements.Delete(ctx, args[0]); err != nil {
				return err
			}
			return writeMessage(cmd, "Entitlement deleted")
		},
	}
	addConfirmFlag(cmd)
	return cmd
}

// entitlementInputFlags is the inline form of an entitlement payload, shared by create and
// update. The type flag is stored as kind because type is a Go keyword.
type entitlementInputFlags struct {
	name              string
	description       string
	kind              string
	aggregationMethod string
	groupSlugs        []string
	slug              string
}

func (f *entitlementInputFlags) register(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.name, "name", "", "Entitlement name")
	cmd.Flags().StringVar(&f.description, "description", "", "Entitlement description")
	cmd.Flags().StringVar(&f.kind, "type", "", "Entitlement type")
	cmd.Flags().StringVar(&f.aggregationMethod, "aggregation-method", "", "Entitlement aggregation method")
	cmd.Flags().StringArrayVar(&f.groupSlugs, "group-slug", nil, "Entitlement group slug, repeat for multiple values")
	cmd.Flags().StringVar(&f.slug, "slug", "", "Entitlement slug")
}

func (f *entitlementInputFlags) changed(cmd *cobra.Command) bool {
	return anyFlagChanged(cmd, "name", "description", "type", "aggregation-method", "group-slug", "slug")
}

func (f *entitlementInputFlags) build(cmd *cobra.Command) func() (sdk.EntitlementInput, error) {
	return func() (sdk.EntitlementInput, error) {
		if err := requiredInlineString(f.name, "name"); err != nil {
			return sdk.EntitlementInput{}, err
		}

		return sdk.EntitlementInput{
			Name:              f.name,
			Description:       optionalString(cmd, "description", f.description),
			Type:              optionalEnum[sdk.EntitlementType](cmd, "type", f.kind),
			AggregationMethod: optionalEnum[sdk.EntitlementAggregationMethod](cmd, "aggregation-method", f.aggregationMethod),
			GroupSlugs:        f.groupSlugs,
			Slug:              optionalString(cmd, "slug", f.slug),
		}, nil
	}
}
