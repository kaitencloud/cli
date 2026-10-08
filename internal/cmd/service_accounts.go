package cmd

import (
	"fmt"

	"github.com/kaitencloud/cli/internal/output"
	"github.com/kaitencloud/sdk-go"
	"github.com/spf13/cobra"
)

func newServiceAccountsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service-accounts",
		Short: "Manage service accounts",
		Args:  cobra.NoArgs,
		RunE:  runHelp,
	}

	cmd.AddCommand(newServiceAccountsListCommand())
	cmd.AddCommand(newServiceAccountsGetCommand())
	cmd.AddCommand(newServiceAccountsCreateCommand())
	cmd.AddCommand(newServiceAccountsUpdateCommand())
	cmd.AddCommand(newServiceAccountsTokensCommand())

	return cmd
}

func newServiceAccountsListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List service accounts",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			items, err := client.ServiceAccounts.List(ctx)
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

func newServiceAccountsGetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "get <service-account-slug>",
		Short: "Get a service account",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			item, err := client.ServiceAccounts.Get(ctx, args[0])
			if err != nil {
				return err
			}
			return writeStructured(cmd, output.FormatYAML, item, output.Table{})
		},
	}
}

func newServiceAccountsCreateCommand() *cobra.Command {
	var file, payload string
	var inline serviceAccountInputFlags
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a service account from --file, --payload, or inline flags",
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
			item, err := client.ServiceAccounts.Create(ctx, inputValue)
			if err != nil {
				return err
			}
			return writeStructured(cmd, output.FormatYAML, item, output.Table{})
		},
	}
	addInputSourceFlags(cmd, &file, &payload, "service account")
	inline.register(cmd)
	inline.registerSlug(cmd)
	return cmd
}

func newServiceAccountsUpdateCommand() *cobra.Command {
	var file, payload string
	var inline serviceAccountInputFlags
	cmd := &cobra.Command{
		Use:   "update <service-account-slug>",
		Short: "Update a service account from --file, --payload, or inline flags",
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
			if err := client.ServiceAccounts.Update(ctx, args[0], inputValue); err != nil {
				return err
			}
			return writeMessage(cmd, "Service account updated")
		},
	}
	addInputSourceFlags(cmd, &file, &payload, "service account")
	inline.register(cmd)
	return cmd
}

func newServiceAccountsTokensCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tokens",
		Short: "Manage service account tokens",
		Args:  cobra.NoArgs,
		RunE:  runHelp,
	}
	cmd.AddCommand(newServiceAccountsTokensListCommand())
	cmd.AddCommand(newServiceAccountsTokensCreateCommand())
	cmd.AddCommand(newServiceAccountsTokensDeleteCommand())
	return cmd
}

func newServiceAccountsTokensListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list <service-account-slug>",
		Short: "List tokens for a service account",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			items, err := client.ServiceAccounts.ListTokens(ctx, args[0])
			if err != nil {
				return err
			}
			rows := make([][]string, 0, len(items))
			for _, item := range items {
				rows = append(rows, []string{item.Name, deref(item.Slug), formatTime(item.ExpiresAt), joinStrings(item.Scopes)})
			}
			return writeStructured(cmd, output.FormatTable, items, output.Table{
				Columns: []string{"NAME", "SLUG", "EXPIRES AT", "SCOPES"},
				Rows:    rows,
			})
		},
	}
}

func newServiceAccountsTokensCreateCommand() *cobra.Command {
	var file, payload string
	var inline tokenInputFlags
	cmd := &cobra.Command{
		Use:   "create <service-account-slug>",
		Short: "Create a token from --file, --payload, or inline flags",
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
			item, err := client.ServiceAccounts.CreateToken(ctx, args[0], inputValue)
			if err != nil {
				return err
			}
			return writeStructured(cmd, output.FormatYAML, item, output.Table{})
		},
	}
	addInputSourceFlags(cmd, &file, &payload, "token")
	inline.register(cmd)
	return cmd
}

func newServiceAccountsTokensDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <service-account-slug> <token-slug>",
		Short: "Delete a token from a service account",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			if err := confirmDestructive(cmd, "delete", fmt.Sprintf("token %q from service account %q", args[1], args[0])); err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			if err := client.ServiceAccounts.DeleteToken(ctx, args[0], args[1]); err != nil {
				return err
			}
			return writeMessage(cmd, "Service account token deleted")
		},
	}
	addConfirmFlag(cmd)
	return cmd
}

// serviceAccountInputFlags is the inline form of a service account payload, shared by
// create and update.
type serviceAccountInputFlags struct {
	name string
	slug string
}

func (f *serviceAccountInputFlags) register(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.name, "name", "", "Service account name")
}

// registerSlug adds --slug to create only. The API never renames a service account:
// its update accepts the slug already in the path and refuses any other, so the
// flag could only restate the argument or be refused.
func (f *serviceAccountInputFlags) registerSlug(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.slug, "slug", "", "Service account slug")
}

func (f *serviceAccountInputFlags) changed(cmd *cobra.Command) bool {
	return anyFlagChanged(cmd, "name", "slug")
}

func (f *serviceAccountInputFlags) build(cmd *cobra.Command) func() (sdk.ServiceAccountInput, error) {
	return func() (sdk.ServiceAccountInput, error) {
		if err := requiredInlineString(f.name, "name"); err != nil {
			return sdk.ServiceAccountInput{}, err
		}

		return sdk.ServiceAccountInput{
			Name: f.name,
			Slug: optionalString(cmd, "slug", f.slug),
		}, nil
	}
}

// tokenInputFlags is the inline form of a service account token payload.
type tokenInputFlags struct {
	name      string
	slug      string
	scopes    []string
	expiresAt string
}

func (f *tokenInputFlags) register(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.name, "name", "", "Token name")
	cmd.Flags().StringVar(&f.slug, "slug", "", "Token slug")
	cmd.Flags().StringArrayVar(&f.scopes, "scope", nil, "Token scope, repeat for multiple values")
	cmd.Flags().StringVar(&f.expiresAt, "expires-at", "", "Expiration timestamp in RFC3339 format")
}

func (f *tokenInputFlags) changed(cmd *cobra.Command) bool {
	return anyFlagChanged(cmd, "name", "slug", "scope", "expires-at")
}

func (f *tokenInputFlags) build(cmd *cobra.Command) func() (sdk.TokenInput, error) {
	return func() (sdk.TokenInput, error) {
		if err := requiredInlineString(f.name, "name"); err != nil {
			return sdk.TokenInput{}, err
		}
		expiresAt, err := parseTimeFlag(f.expiresAt, "expires-at")
		if err != nil {
			return sdk.TokenInput{}, err
		}

		return sdk.TokenInput{
			Name:      f.name,
			Slug:      optionalString(cmd, "slug", f.slug),
			Scopes:    f.scopes,
			ExpiresAt: expiresAt,
		}, nil
	}
}
