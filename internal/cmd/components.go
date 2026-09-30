package cmd

import (
	"fmt"

	"github.com/kaitencloud/cli/internal/output"
	"github.com/kaitencloud/sdk-go"
	"github.com/spf13/cobra"
)

func newComponentsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "components",
		Short: "Manage components",
		Args:  cobra.NoArgs,
		RunE:  runHelp,
	}

	cmd.AddCommand(newComponentsListCommand())
	cmd.AddCommand(newComponentsGetCommand())
	cmd.AddCommand(newComponentsCreateCommand())
	cmd.AddCommand(newComponentsUpdateCommand())
	cmd.AddCommand(newComponentsDeleteCommand())

	return cmd
}

func newComponentsListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List components",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()

			items, err := client.Components.List(ctx)
			if err != nil {
				return err
			}
			rows := make([][]string, 0, len(items))
			for _, item := range items {
				rows = append(rows, []string{item.Name, deref(item.Slug), item.Version, deref(item.Id)})
			}
			return writeStructured(cmd, output.FormatTable, items, output.Table{
				Columns: []string{"NAME", "SLUG", "VERSION", "ID"},
				Rows:    rows,
			})
		},
	}
}

func newComponentsGetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "get <component-slug>",
		Short: "Get a component",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			item, err := client.Components.Get(ctx, args[0])
			if err != nil {
				return err
			}
			return writeStructured(cmd, output.FormatYAML, item, output.Table{})
		},
	}
}

func newComponentsCreateCommand() *cobra.Command {
	var file, payload string
	var inline componentInputFlags
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a component from --file, --payload, or inline flags",
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
			item, err := client.Components.Create(ctx, inputValue)
			if err != nil {
				return err
			}
			return writeStructured(cmd, output.FormatYAML, item, output.Table{})
		},
	}
	addInputSourceFlags(cmd, &file, &payload, "component")
	inline.register(cmd)
	return cmd
}

func newComponentsUpdateCommand() *cobra.Command {
	var file, payload string
	var inline componentInputFlags
	cmd := &cobra.Command{
		Use:   "update <component-slug>",
		Short: "Update a component from --file, --payload, or inline flags",
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
			item, err := client.Components.Update(ctx, args[0], inputValue)
			if err != nil {
				return err
			}
			return writeStructured(cmd, output.FormatYAML, item, output.Table{})
		},
	}
	addInputSourceFlags(cmd, &file, &payload, "component")
	inline.register(cmd)
	return cmd
}

func newComponentsDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <component-slug>",
		Short: "Delete a component",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			if err := confirmDestructive(cmd, "delete", fmt.Sprintf("component %q", args[0])); err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			if err := client.Components.Delete(ctx, args[0]); err != nil {
				return err
			}
			return writeMessage(cmd, "Component deleted")
		},
	}
	addConfirmFlag(cmd)
	return cmd
}

// componentInputFlags is the inline form of a component payload, shared by create and update.
type componentInputFlags struct {
	name                string
	version             string
	description         string
	previousComponentID string
	slug                string
}

func (f *componentInputFlags) register(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.name, "name", "", "Component name")
	cmd.Flags().StringVar(&f.version, "version", "", "Component version")
	cmd.Flags().StringVar(&f.description, "description", "", "Component description")
	cmd.Flags().StringVar(&f.previousComponentID, "previous-component-id", "", "Previous component ID")
	cmd.Flags().StringVar(&f.slug, "slug", "", "Component slug")
}

func (f *componentInputFlags) changed(cmd *cobra.Command) bool {
	return anyFlagChanged(cmd, "name", "version", "description", "previous-component-id", "slug")
}

func (f *componentInputFlags) build(cmd *cobra.Command) func() (sdk.ComponentInput, error) {
	return func() (sdk.ComponentInput, error) {
		if err := requiredInlineString(f.name, "name"); err != nil {
			return sdk.ComponentInput{}, err
		}
		if err := requiredInlineString(f.version, "version"); err != nil {
			return sdk.ComponentInput{}, err
		}

		return sdk.ComponentInput{
			Name:                f.name,
			Version:             f.version,
			Description:         optionalString(cmd, "description", f.description),
			PreviousComponentID: optionalString(cmd, "previous-component-id", f.previousComponentID),
			Slug:                optionalString(cmd, "slug", f.slug),
		}, nil
	}
}
