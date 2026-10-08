package cmd

import (
	"fmt"

	"github.com/kaitencloud/cli/internal/output"
	"github.com/kaitencloud/sdk-go"
	"github.com/spf13/cobra"
)

func newCustomersCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "customers",
		Short: "Manage customers",
		Args:  cobra.NoArgs,
		RunE:  runHelp,
	}

	cmd.AddCommand(newCustomersListCommand())
	cmd.AddCommand(newCustomersGetCommand())
	cmd.AddCommand(newCustomersCreateCommand())
	cmd.AddCommand(newCustomersUpdateCommand())
	cmd.AddCommand(newCustomersDeleteCommand())

	return cmd
}

func newCustomersListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List customers",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			items, err := client.Customers.List(ctx)
			if err != nil {
				return err
			}
			rows := make([][]string, 0, len(items))
			for _, item := range items {
				rows = append(rows, []string{item.Name, deref(item.Slug), deref(item.ExternalCustomerId), deref(item.Id)})
			}
			return writeStructured(cmd, output.FormatTable, items, output.Table{
				Columns: []string{"NAME", "SLUG", "EXTERNAL ID", "ID"},
				Rows:    rows,
			})
		},
	}
}

func newCustomersGetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "get <customer-slug>",
		Short: "Get a customer",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			item, err := client.Customers.Get(ctx, args[0])
			if err != nil {
				return err
			}
			return writeStructured(cmd, output.FormatYAML, item, output.Table{})
		},
	}
}

func newCustomersCreateCommand() *cobra.Command {
	var file, payload string
	var inline customerInputFlags
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a customer from --file, --payload, or inline flags",
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
			item, err := client.Customers.Create(ctx, inputValue)
			if err != nil {
				return err
			}
			return writeStructured(cmd, output.FormatYAML, item, output.Table{})
		},
	}
	addInputSourceFlags(cmd, &file, &payload, "customer")
	inline.register(cmd)
	inline.registerSlug(cmd)
	return cmd
}

func newCustomersUpdateCommand() *cobra.Command {
	var file, payload string
	var inline customerInputFlags
	cmd := &cobra.Command{
		Use:   "update <customer-slug>",
		Short: "Update a customer from --file, --payload, or inline flags",
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
			if err := client.Customers.Update(ctx, args[0], inputValue); err != nil {
				return err
			}
			return writeMessage(cmd, "Customer updated")
		},
	}
	addInputSourceFlags(cmd, &file, &payload, "customer")
	inline.register(cmd)
	return cmd
}

func newCustomersDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <customer-slug>",
		Short: "Delete a customer",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			if err := confirmDestructive(cmd, "delete", fmt.Sprintf("customer %q", args[0])); err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			if err := client.Customers.Delete(ctx, args[0]); err != nil {
				return err
			}
			return writeMessage(cmd, "Customer deleted")
		},
	}
	addConfirmFlag(cmd)
	return cmd
}

// customerInputFlags is the inline form of a customer payload, shared by create and update.
type customerInputFlags struct {
	name               string
	externalCustomerID string
	slug               string
}

func (f *customerInputFlags) register(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.name, "name", "", "Customer name")
	cmd.Flags().StringVar(&f.externalCustomerID, "external-customer-id", "", "External customer ID")
}

// registerSlug adds --slug to create only. The API never renames a customer:
// its update accepts the slug already in the path and refuses any other, so the
// flag could only restate the argument or be refused.
func (f *customerInputFlags) registerSlug(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.slug, "slug", "", "Customer slug")
}

func (f *customerInputFlags) changed(cmd *cobra.Command) bool {
	return anyFlagChanged(cmd, "name", "external-customer-id", "slug")
}

func (f *customerInputFlags) build(cmd *cobra.Command) func() (sdk.CustomerInput, error) {
	return func() (sdk.CustomerInput, error) {
		if err := requiredInlineString(f.name, "name"); err != nil {
			return sdk.CustomerInput{}, err
		}

		return sdk.CustomerInput{
			Name:               f.name,
			ExternalCustomerID: optionalString(cmd, "external-customer-id", f.externalCustomerID),
			Slug:               optionalString(cmd, "slug", f.slug),
		}, nil
	}
}
