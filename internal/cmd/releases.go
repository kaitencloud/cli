package cmd

import (
	"fmt"

	"github.com/kaitencloud/cli/internal/output"
	"github.com/kaitencloud/sdk-go"
	"github.com/spf13/cobra"
)

func newReleasesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "releases",
		Short: "Manage releases",
		Args:  cobra.NoArgs,
		RunE:  runHelp,
	}

	cmd.AddCommand(newReleasesListCommand())
	cmd.AddCommand(newReleasesGetCommand())
	cmd.AddCommand(newReleasesCreateCommand())
	cmd.AddCommand(newReleasesDeleteCommand())

	return cmd
}

func newReleasesListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List releases",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			items, err := client.Releases.List(ctx)
			if err != nil {
				return err
			}
			rows := make([][]string, 0, len(items))
			for _, item := range items {
				rows = append(rows, []string{item.Version, deref(item.Slug), deref(item.Id)})
			}
			return writeStructured(cmd, output.FormatTable, items, output.Table{
				Columns: []string{"VERSION", "SLUG", "ID"},
				Rows:    rows,
			})
		},
	}
}

func newReleasesGetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "get <release-slug>",
		Short: "Get a release",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			item, err := client.Releases.Get(ctx, args[0])
			if err != nil {
				return err
			}
			return writeStructured(cmd, output.FormatYAML, item, output.Table{})
		},
	}
}

func newReleasesCreateCommand() *cobra.Command {
	var file, payload string
	var inline releaseInputFlags
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a release from --file, --payload, or inline flags",
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
			item, err := client.Releases.Create(ctx, inputValue)
			if err != nil {
				return err
			}
			return writeStructured(cmd, output.FormatYAML, item, output.Table{})
		},
	}
	addInputSourceFlags(cmd, &file, &payload, "release")
	inline.register(cmd)
	return cmd
}

func newReleasesDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <release-slug>",
		Short: "Delete a release",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			if err := confirmDestructive(cmd, "delete", fmt.Sprintf("release %q", args[0])); err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			if err := client.Releases.Delete(ctx, args[0]); err != nil {
				return err
			}
			return writeMessage(cmd, "Release deleted")
		},
	}
	addConfirmFlag(cmd)
	return cmd
}

// releaseInputFlags is the inline form of a release payload.
type releaseInputFlags struct {
	version      string
	description  string
	componentIDs []string
	slug         string
}

func (f *releaseInputFlags) register(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.version, "version", "", "Release version")
	cmd.Flags().StringVar(&f.description, "description", "", "Release description")
	cmd.Flags().StringArrayVar(&f.componentIDs, "component-id", nil, "Component ID, repeat for multiple values")
	cmd.Flags().StringVar(&f.slug, "slug", "", "Release slug")
}

func (f *releaseInputFlags) changed(cmd *cobra.Command) bool {
	return anyFlagChanged(cmd, "version", "description", "component-id", "slug")
}

func (f *releaseInputFlags) build(cmd *cobra.Command) func() (sdk.ReleaseInput, error) {
	return func() (sdk.ReleaseInput, error) {
		if err := requiredInlineString(f.version, "version"); err != nil {
			return sdk.ReleaseInput{}, err
		}

		return sdk.ReleaseInput{
			Version:      f.version,
			Description:  optionalString(cmd, "description", f.description),
			ComponentIDs: f.componentIDs,
			Slug:         optionalString(cmd, "slug", f.slug),
		}, nil
	}
}
