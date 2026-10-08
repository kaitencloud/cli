package cmd

import (
	"fmt"

	"github.com/kaitencloud/cli/internal/output"
	"github.com/kaitencloud/sdk-go"
	"github.com/spf13/cobra"
)

func newDeploymentZonesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deployment-zones",
		Short: "Manage deployment zones",
		Args:  cobra.NoArgs,
		RunE:  runHelp,
	}

	cmd.AddCommand(newDeploymentZonesListCommand())
	cmd.AddCommand(newDeploymentZonesGetCommand())
	cmd.AddCommand(newDeploymentZonesCreateCommand())
	cmd.AddCommand(newDeploymentZonesUpdateCommand())
	cmd.AddCommand(newDeploymentZonesDeleteCommand())

	return cmd
}

func newDeploymentZonesListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List deployment zones",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			items, err := client.DeploymentZones.List(ctx)
			if err != nil {
				return err
			}
			rows := make([][]string, 0, len(items))
			for _, item := range items {
				rows = append(rows, []string{item.Name, deref(item.Slug), item.Type, deref(item.Id)})
			}
			return writeStructured(cmd, output.FormatTable, items, output.Table{
				Columns: []string{"NAME", "SLUG", "TYPE", "ID"},
				Rows:    rows,
			})
		},
	}
}

func newDeploymentZonesGetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "get <deployment-zone-slug>",
		Short: "Get a deployment zone",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			item, err := client.DeploymentZones.Get(ctx, args[0])
			if err != nil {
				return err
			}
			return writeStructured(cmd, output.FormatYAML, item, output.Table{})
		},
	}
}

func newDeploymentZonesCreateCommand() *cobra.Command {
	var file, payload string
	var inline deploymentZoneInputFlags
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a deployment zone from --file, --payload, or inline flags",
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
			item, err := client.DeploymentZones.Create(ctx, inputValue)
			if err != nil {
				return err
			}
			return writeStructured(cmd, output.FormatYAML, item, output.Table{})
		},
	}
	addInputSourceFlags(cmd, &file, &payload, "deployment zone")
	inline.register(cmd)
	inline.registerSlug(cmd)
	return cmd
}

func newDeploymentZonesUpdateCommand() *cobra.Command {
	var file, payload string
	var inline deploymentZoneInputFlags
	cmd := &cobra.Command{
		Use:   "update <deployment-zone-slug>",
		Short: "Update a deployment zone from --file, --payload, or inline flags",
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
			if err := client.DeploymentZones.Update(ctx, args[0], inputValue); err != nil {
				return err
			}
			return writeMessage(cmd, "Deployment zone updated")
		},
	}
	addInputSourceFlags(cmd, &file, &payload, "deployment zone")
	inline.register(cmd)
	return cmd
}

func newDeploymentZonesDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <deployment-zone-slug>",
		Short: "Delete a deployment zone",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			if err := confirmDestructive(cmd, "delete", fmt.Sprintf("deployment zone %q", args[0])); err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			if err := client.DeploymentZones.Delete(ctx, args[0]); err != nil {
				return err
			}
			return writeMessage(cmd, "Deployment zone deleted")
		},
	}
	addConfirmFlag(cmd)
	return cmd
}

// deploymentZoneInputFlags is the inline form of a deployment zone payload, shared by
// create and update. The type flag is stored as kind because type is a Go keyword.
type deploymentZoneInputFlags struct {
	name        string
	kind        string
	description string
	metadata    string
	releaseID   string
	slug        string
}

func (f *deploymentZoneInputFlags) register(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.name, "name", "", "Deployment zone name")
	cmd.Flags().StringVar(&f.kind, "type", "", "Deployment zone type")
	cmd.Flags().StringVar(&f.description, "description", "", "Deployment zone description")
	cmd.Flags().StringVar(&f.metadata, "metadata-json", "", "Deployment zone metadata as inline JSON or YAML object")
	cmd.Flags().StringVar(&f.releaseID, "release-id", "", "Release ID")
}

// registerSlug adds --slug to create only. The API never renames a deployment zone:
// its update accepts the slug already in the path and refuses any other, so the
// flag could only restate the argument or be refused.
func (f *deploymentZoneInputFlags) registerSlug(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.slug, "slug", "", "Deployment zone slug")
}

func (f *deploymentZoneInputFlags) changed(cmd *cobra.Command) bool {
	return anyFlagChanged(cmd, "name", "type", "description", "metadata-json", "release-id", "slug")
}

func (f *deploymentZoneInputFlags) build(cmd *cobra.Command) func() (sdk.DeploymentZoneInput, error) {
	return func() (sdk.DeploymentZoneInput, error) {
		if err := requiredInlineString(f.name, "name"); err != nil {
			return sdk.DeploymentZoneInput{}, err
		}
		if err := requiredInlineString(f.kind, "type"); err != nil {
			return sdk.DeploymentZoneInput{}, err
		}
		if err := requiredInlineString(f.description, "description"); err != nil {
			return sdk.DeploymentZoneInput{}, err
		}
		metadata, err := parseMapFlag(f.metadata, "metadata-json")
		if err != nil {
			return sdk.DeploymentZoneInput{}, err
		}

		return sdk.DeploymentZoneInput{
			Name:        f.name,
			Type:        f.kind,
			Description: f.description,
			Metadata:    metadata,
			ReleaseID:   optionalString(cmd, "release-id", f.releaseID),
			Slug:        optionalString(cmd, "slug", f.slug),
		}, nil
	}
}
