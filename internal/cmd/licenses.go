package cmd

import (
	"fmt"

	"github.com/kaitencloud/cli/internal/output"
	"github.com/kaitencloud/sdk-go"
	"github.com/spf13/cobra"
)

func newLicensesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "licenses",
		Short: "Manage licenses",
		Args:  cobra.NoArgs,
		RunE:  runHelp,
	}

	cmd.AddCommand(newLicensesListCommand())
	cmd.AddCommand(newLicensesGetCommand())
	cmd.AddCommand(newLicensesCreateCommand())
	cmd.AddCommand(newLicensesUpdateCommand())
	cmd.AddCommand(newLicensesDeleteCommand())
	cmd.AddCommand(newLicensesEntitlementsCommand())

	return cmd
}

func newLicensesListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List licenses",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			items, err := client.Licenses.List(ctx)
			if err != nil {
				return err
			}
			rows := make([][]string, 0, len(items))
			for _, item := range items {
				rows = append(rows, []string{item.Name, deref(item.Slug), string(item.Type), deref(item.Version)})
			}
			return writeStructured(cmd, output.FormatTable, items, output.Table{
				Columns: []string{"NAME", "SLUG", "TYPE", "VERSION"},
				Rows:    rows,
			})
		},
	}
}

func newLicensesGetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "get <license-slug>",
		Short: "Get a license",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			item, err := client.Licenses.Get(ctx, args[0])
			if err != nil {
				return err
			}
			return writeStructured(cmd, output.FormatYAML, item, output.Table{})
		},
	}
}

func newLicensesCreateCommand() *cobra.Command {
	var file, payload string
	var inline licenseInputFlags
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a license from --file, --payload, or inline flags",
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
			item, err := client.Licenses.Create(ctx, inputValue)
			if err != nil {
				return err
			}
			return writeStructured(cmd, output.FormatYAML, item, output.Table{})
		},
	}
	addInputSourceFlags(cmd, &file, &payload, "license")
	inline.register(cmd)
	inline.registerSlug(cmd)
	return cmd
}

func newLicensesUpdateCommand() *cobra.Command {
	var file, payload string
	var inline licenseInputFlags
	cmd := &cobra.Command{
		Use:   "update <license-slug>",
		Short: "Update a license from --file, --payload, or inline flags",
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
			if err := client.Licenses.Update(ctx, args[0], inputValue); err != nil {
				return err
			}
			return writeMessage(cmd, "License updated")
		},
	}
	addInputSourceFlags(cmd, &file, &payload, "license")
	inline.register(cmd)
	return cmd
}

func newLicensesDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <license-slug>",
		Short: "Delete a license",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			if err := confirmDestructive(cmd, "delete", fmt.Sprintf("license %q", args[0])); err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			if err := client.Licenses.Delete(ctx, args[0]); err != nil {
				return err
			}
			return writeMessage(cmd, "License deleted")
		},
	}
	addConfirmFlag(cmd)
	return cmd
}

func newLicensesEntitlementsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "entitlements",
		Short: "Manage license entitlements",
		Args:  cobra.NoArgs,
		RunE:  runHelp,
	}
	cmd.AddCommand(newLicensesEntitlementsListCommand())
	cmd.AddCommand(newLicensesEntitlementsGetCommand())
	cmd.AddCommand(newLicensesEntitlementsAssociateCommand())
	cmd.AddCommand(newLicensesEntitlementsUpdateCommand())
	cmd.AddCommand(newLicensesEntitlementsDeleteCommand())
	return cmd
}

func newLicensesEntitlementsListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list <license-slug>",
		Short: "List entitlements for a license",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			items, err := client.Licenses.ListEntitlements(ctx, args[0])
			if err != nil {
				return err
			}
			rows := make([][]string, 0, len(items))
			for _, item := range items {
				rows = append(rows, []string{
					deref(item.EntitlementName),
					deref(item.EntitlementSlug),
					string(deref(item.EntitlementType)),
					jsonString(item.Value),
				})
			}
			return writeStructured(cmd, output.FormatTable, items, output.Table{
				Columns: []string{"NAME", "SLUG", "TYPE", "VALUE"},
				Rows:    rows,
			})
		},
	}
}

func newLicensesEntitlementsGetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "get <license-slug> <entitlement-slug>",
		Short: "Get one entitlement on a license",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			item, err := client.Licenses.GetEntitlement(ctx, args[0], args[1])
			if err != nil {
				return err
			}
			return writeStructured(cmd, output.FormatYAML, item, output.Table{})
		},
	}
}

func newLicensesEntitlementsAssociateCommand() *cobra.Command {
	var inline licenseValueFlags
	cmd := &cobra.Command{
		Use:   "associate <license-slug> <entitlement-slug>",
		Short: "Associate an entitlement value with a license",
		Long: "Associate an entitlement value with a license. Exactly one of --file, --payload,\n" +
			"--number, --boolean, --object-file or --object-payload carries the value.",
		Args: cobra.ExactArgs(2),
		Example: `  kaiten licenses entitlements associate enterprise seats --number 50
  kaiten licenses entitlements associate enterprise sso --boolean=true
  kaiten licenses entitlements associate enterprise limits --object-payload '{"max":10}'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			value, err := inline.resolve(cmd)
			if err != nil {
				return err
			}
			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			if err := client.Licenses.AssociateEntitlement(ctx, args[0], args[1], value); err != nil {
				return err
			}
			return writeMessage(cmd, "License entitlement associated")
		},
	}
	inline.register(cmd)
	return cmd
}

func newLicensesEntitlementsUpdateCommand() *cobra.Command {
	var inline licenseValueFlags
	cmd := &cobra.Command{
		Use:   "update <license-slug> <entitlement-slug>",
		Short: "Update an entitlement value on a license",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			value, err := inline.resolve(cmd)
			if err != nil {
				return err
			}
			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			if err := client.Licenses.UpdateEntitlement(ctx, args[0], args[1], value); err != nil {
				return err
			}
			return writeMessage(cmd, "License entitlement updated")
		},
	}
	inline.register(cmd)
	return cmd
}

func newLicensesEntitlementsDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <license-slug> <entitlement-slug>",
		Short: "Delete an entitlement from a license",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			if err := confirmDestructive(cmd, "delete", fmt.Sprintf("entitlement %q from license %q", args[1], args[0])); err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			if err := client.Licenses.DeleteEntitlement(ctx, args[0], args[1]); err != nil {
				return err
			}
			return writeMessage(cmd, "License entitlement deleted")
		},
	}
	addConfirmFlag(cmd)
	return cmd
}

// licenseInputFlags is the inline form of a license payload, shared by create and update.
// The type flag is stored as kind because type is a Go keyword.
type licenseInputFlags struct {
	name        string
	description string
	kind        string
	version     string
	versionName string
	isDefault   bool
	slug        string
}

func (f *licenseInputFlags) register(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.name, "name", "", "License name")
	cmd.Flags().StringVar(&f.description, "description", "", "License description")
	cmd.Flags().StringVar(&f.kind, "type", "", "License type")
	cmd.Flags().StringVar(&f.version, "version", "", "License version")
	cmd.Flags().StringVar(&f.versionName, "version-name", "", "Human-readable license version name")
	cmd.Flags().BoolVar(&f.isDefault, "default", false, "Whether the license is the default")
}

// registerSlug adds --slug to create only. The API never renames a license:
// its update accepts the slug already in the path and refuses any other, so the
// flag could only restate the argument or be refused.
func (f *licenseInputFlags) registerSlug(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.slug, "slug", "", "License slug")
}

func (f *licenseInputFlags) changed(cmd *cobra.Command) bool {
	return anyFlagChanged(cmd, "name", "description", "type", "version", "version-name", "default", "slug")
}

func (f *licenseInputFlags) build(cmd *cobra.Command) func() (sdk.LicenseInput, error) {
	return func() (sdk.LicenseInput, error) {
		for _, required := range []struct{ value, flag string }{
			{f.name, "name"},
			{f.description, "description"},
			{f.kind, "type"},
			{f.version, "version"},
		} {
			if err := requiredInlineString(required.value, required.flag); err != nil {
				return sdk.LicenseInput{}, err
			}
		}
		// --default is required rather than defaulted to false: a license silently
		// created as non-default is not what the operator asked for. There is no
		// --active counterpart -- no request body publishes IsActive, so the API
		// gives this SDK no way to set it either.
		if err := requiredInlineFlag(cmd, "default"); err != nil {
			return sdk.LicenseInput{}, err
		}

		return sdk.LicenseInput{
			Name:        f.name,
			Description: f.description,
			Type:        sdk.LicenseType(f.kind),
			Version:     f.version,
			VersionName: optionalString(cmd, "version-name", f.versionName),
			IsDefault:   f.isDefault,
			Slug:        optionalString(cmd, "slug", f.slug),
		}, nil
	}
}
