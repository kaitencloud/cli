package cmd

import (
	"fmt"

	"github.com/kaitencloud/cli/internal/output"
	"github.com/kaitencloud/sdk-go"
	"github.com/spf13/cobra"
)

func newEntitlementGroupsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "entitlement-groups",
		Short: "Manage entitlement groups",
		Args:  cobra.NoArgs,
		RunE:  runHelp,
	}

	cmd.AddCommand(newEntitlementGroupsListCommand())
	cmd.AddCommand(newEntitlementGroupsGetCommand())
	cmd.AddCommand(newEntitlementGroupsCreateCommand())
	cmd.AddCommand(newEntitlementGroupsUpdateCommand())
	cmd.AddCommand(newEntitlementGroupsDeleteCommand())
	cmd.AddCommand(newEntitlementGroupsAddEntitlementCommand())
	cmd.AddCommand(newEntitlementGroupsRemoveEntitlementCommand())
	cmd.AddCommand(newEntitlementGroupsUsageCommand())

	return cmd
}

func newEntitlementGroupsListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List entitlement groups",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			items, err := client.EntitlementGroups.List(ctx)
			if err != nil {
				return err
			}
			rows := make([][]string, 0, len(items))
			for _, item := range items {
				rows = append(rows, []string{item.Name, deref(item.Slug), deref(item.Id)})
			}
			return writeStructured(cmd, output.FormatTable, items, output.Table{
				Columns: []string{"NAME", "SLUG", "ID"},
				Rows:    rows,
			})
		},
	}
}

func newEntitlementGroupsGetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "get <entitlement-group-slug>",
		Short: "Get an entitlement group",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			item, err := client.EntitlementGroups.Get(ctx, args[0])
			if err != nil {
				return err
			}
			return writeStructured(cmd, output.FormatYAML, item, output.Table{})
		},
	}
}

func newEntitlementGroupsCreateCommand() *cobra.Command {
	var file, payload string
	var inline entitlementGroupInputFlags
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an entitlement group from --file, --payload, or inline flags",
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
			item, err := client.EntitlementGroups.Create(ctx, inputValue)
			if err != nil {
				return err
			}
			return writeStructured(cmd, output.FormatYAML, item, output.Table{})
		},
	}
	addInputSourceFlags(cmd, &file, &payload, "entitlement group")
	inline.register(cmd)
	return cmd
}

func newEntitlementGroupsUpdateCommand() *cobra.Command {
	var file, payload string
	var inline entitlementGroupInputFlags
	cmd := &cobra.Command{
		Use:   "update <entitlement-group-slug>",
		Short: "Update an entitlement group from --file, --payload, or inline flags",
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
			if err := client.EntitlementGroups.Update(ctx, args[0], inputValue); err != nil {
				return err
			}
			return writeMessage(cmd, "Entitlement group updated")
		},
	}
	addInputSourceFlags(cmd, &file, &payload, "entitlement group")
	inline.register(cmd)
	return cmd
}

func newEntitlementGroupsDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <entitlement-group-slug>",
		Short: "Delete an entitlement group",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			if err := confirmDestructive(cmd, "delete", fmt.Sprintf("entitlement group %q", args[0])); err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			if err := client.EntitlementGroups.Delete(ctx, args[0]); err != nil {
				return err
			}
			return writeMessage(cmd, "Entitlement group deleted")
		},
	}
	addConfirmFlag(cmd)
	return cmd
}

func newEntitlementGroupsAddEntitlementCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "add-entitlement <entitlement-group-slug> <entitlement-slug>",
		Short: "Add an entitlement to a group",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			if err := client.EntitlementGroups.AddEntitlement(ctx, args[0], args[1]); err != nil {
				return err
			}
			return writeMessage(cmd, "Entitlement added to group")
		},
	}
}

func newEntitlementGroupsRemoveEntitlementCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove-entitlement <entitlement-group-slug> <entitlement-slug>",
		Short: "Remove an entitlement from a group",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			if err := confirmDestructive(cmd, "remove", fmt.Sprintf("entitlement %q from entitlement group %q", args[1], args[0])); err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			if err := client.EntitlementGroups.RemoveEntitlement(ctx, args[0], args[1]); err != nil {
				return err
			}
			return writeMessage(cmd, "Entitlement removed from group")
		},
	}
	addConfirmFlag(cmd)
	return cmd
}

func newEntitlementGroupsUsageCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "usage <entitlement-group-slug> <instance-slug>",
		Short: "Get usage for an entitlement group on an instance",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			items, err := client.EntitlementGroups.GetUsage(ctx, args[0], args[1])
			if err != nil {
				return err
			}
			rows := make([][]string, 0, len(items))
			for _, item := range items {
				rows = append(rows, []string{
					item.EntitlementName,
					item.EntitlementSlug,
					item.EntitlementType,
					renderUnion(item.LicenseValue),
					renderUnion(item.UsageValue),
				})
			}
			return writeStructured(cmd, output.FormatTable, items, output.Table{
				Columns: []string{"NAME", "SLUG", "TYPE", "LICENSE VALUE", "USAGE VALUE"},
				Rows:    rows,
			})
		},
	}
}

// entitlementGroupInputFlags is the inline form of an entitlement group payload, shared by
// create and update.
type entitlementGroupInputFlags struct {
	name        string
	description string
	slug        string
}

func (f *entitlementGroupInputFlags) register(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.name, "name", "", "Entitlement group name")
	cmd.Flags().StringVar(&f.description, "description", "", "Entitlement group description")
	cmd.Flags().StringVar(&f.slug, "slug", "", "Entitlement group slug")
}

func (f *entitlementGroupInputFlags) changed(cmd *cobra.Command) bool {
	return anyFlagChanged(cmd, "name", "description", "slug")
}

func (f *entitlementGroupInputFlags) build(cmd *cobra.Command) func() (sdk.EntitlementGroupInput, error) {
	return func() (sdk.EntitlementGroupInput, error) {
		if err := requiredInlineString(f.name, "name"); err != nil {
			return sdk.EntitlementGroupInput{}, err
		}

		return sdk.EntitlementGroupInput{
			Name:        f.name,
			Description: optionalString(cmd, "description", f.description),
			Slug:        optionalString(cmd, "slug", f.slug),
		}, nil
	}
}
