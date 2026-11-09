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
	inline.registerSlug(cmd)
	return cmd
}

func newEntitlementsUpdateCommand() *cobra.Command {
	var file, payload string
	var inline entitlementInputFlags
	cmd := &cobra.Command{
		Use:   "update <entitlement-slug>",
		Short: "Update an entitlement from --file, --payload, or inline flags",
		Long: "Update an entitlement. The API replaces an entitlement with what it is sent: a field\n" +
			"left out is reset, and a periodic entitlement's reset period and anchor have to be sent\n" +
			"back as they are. Inline flags change only the fields they name: the command reads the\n" +
			"entitlement first and sends every other field back unchanged. --file and --payload are\n" +
			"sent as written, so they carry every field the entitlement keeps.",
		Args: cobra.ExactArgs(1),
		Example: `  kaiten entitlements update monthly-orders --warning-threshold-percent 80
  kaiten entitlements update menu-items --description "Dishes on the menu" --display-order 1
  kaiten entitlements update locations --icon ""`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := commandContext(cmd)
			defer cancel()

			// The client is built on first use, so that a usage error -- no input
			// source, or two -- is reported before a configuration one.
			var client *sdk.Client
			connect := func() (*sdk.Client, error) {
				if client == nil {
					built, _, err := newClient(cmd)
					if err != nil {
						return nil, err
					}
					client = built
				}
				return client, nil
			}

			inputValue, err := resolveInput(file, payload, inline.changed(cmd), func() (sdk.EntitlementInput, error) {
				connected, err := connect()
				if err != nil {
					return sdk.EntitlementInput{}, err
				}
				current, err := connected.Entitlements.Get(ctx, args[0])
				if err != nil {
					return sdk.EntitlementInput{}, err
				}
				return inline.applyTo(cmd, current), nil
			})
			if err != nil {
				return err
			}
			connected, err := connect()
			if err != nil {
				return err
			}
			if err := connected.Entitlements.Update(ctx, args[0], inputValue); err != nil {
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
	name                    string
	description             string
	kind                    string
	aggregationMethod       string
	groupSlugs              []string
	slug                    string
	resetPeriod             string
	resetAnchor             string
	warningThresholdPercent int32
	icon                    string
	unitSingular            string
	unitPlural              string
	saleUnitSingular        string
	saleUnitPlural          string
	saleUnitFactor          float64
	userFacing              bool
	displayOrder            int32
}

// optionalFlags are the inline flags of the optional fields, the ones build and applyTo both
// set only when the invocation names them.
var optionalFlags = []string{
	"reset-period", "reset-anchor", "warning-threshold-percent",
	"icon", "unit-singular", "unit-plural", "sale-unit-singular", "sale-unit-plural", "sale-unit-factor",
	"user-facing", "display-order",
}

func (f *entitlementInputFlags) register(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.name, "name", "", "Entitlement name")
	cmd.Flags().StringVar(&f.description, "description", "", "Entitlement description")
	cmd.Flags().StringVar(&f.kind, "type", "", "Entitlement type")
	cmd.Flags().StringVar(&f.aggregationMethod, "aggregation-method", "", "Entitlement aggregation method")
	cmd.Flags().StringArrayVar(&f.groupSlugs, "group-slug", nil, "Entitlement group slug, repeat for multiple values")
	cmd.Flags().StringVar(&f.resetPeriod, "reset-period", "",
		"How often a NUMBER entitlement's usage resets: HOUR, DAY, WEEK, MONTH or YEAR (set once, never changed)")
	cmd.Flags().StringVar(&f.resetAnchor, "reset-anchor", "",
		"What the reset windows align on: CALENDAR (the default) or LICENSE_START (set once, never changed)")
	cmd.Flags().Int32Var(&f.warningThresholdPercent, "warning-threshold-percent", 0,
		"Share of a grant's value, 0 to 100, at which usage raises an early warning; 0 disables it")
	cmd.Flags().StringVar(&f.icon, "icon", "", `Icon token such as "lucide:rocket"; an empty value removes it`)
	cmd.Flags().StringVar(&f.unitSingular, "unit-singular", "", "Base unit label, singular; an empty value removes it")
	cmd.Flags().StringVar(&f.unitPlural, "unit-plural", "", "Base unit label, plural; an empty value removes it")
	cmd.Flags().StringVar(&f.saleUnitSingular, "sale-unit-singular", "", "Sale unit label, singular; an empty value removes it")
	cmd.Flags().StringVar(&f.saleUnitPlural, "sale-unit-plural", "", "Sale unit label, plural; an empty value removes it")
	cmd.Flags().Float64Var(&f.saleUnitFactor, "sale-unit-factor", 0, "Base units in one sale unit")
	cmd.Flags().BoolVar(&f.userFacing, "user-facing", false, "Show the entitlement in customer-facing components")
	cmd.Flags().Int32Var(&f.displayOrder, "display-order", 0, "Sort order in customer-facing components, ascending")
}

// registerSlug adds --slug to create only. The API never renames an entitlement:
// its update accepts the slug already in the path and refuses any other, so the
// flag could only restate the argument or be refused.
func (f *entitlementInputFlags) registerSlug(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.slug, "slug", "", "Entitlement slug")
}

func (f *entitlementInputFlags) changed(cmd *cobra.Command) bool {
	return anyFlagChanged(cmd, append([]string{"name", "description", "type", "aggregation-method", "group-slug", "slug"},
		optionalFlags...)...)
}

func (f *entitlementInputFlags) build(cmd *cobra.Command) func() (sdk.EntitlementInput, error) {
	return func() (sdk.EntitlementInput, error) {
		if err := requiredInlineString(f.name, "name"); err != nil {
			return sdk.EntitlementInput{}, err
		}

		input := sdk.EntitlementInput{
			Name:              f.name,
			Description:       optionalString(cmd, "description", f.description),
			Type:              optionalEnum[sdk.EntitlementType](cmd, "type", f.kind),
			AggregationMethod: optionalEnum[sdk.EntitlementAggregationMethod](cmd, "aggregation-method", f.aggregationMethod),
			GroupSlugs:        f.groupSlugs,
			Slug:              optionalString(cmd, "slug", f.slug),
		}
		f.setOptional(cmd, &input)
		return input, nil
	}
}

// applyTo is the update counterpart of build: the entitlement as it stands, with only the
// fields the invocation names changed. The API replaces an entitlement with what it is sent,
// resetting a field left out and refusing an update that drops a periodic entitlement's
// window, so every field the operator did not mention is sent back as it was read. Groups
// are the exception: left out, they stay as they are.
func (f *entitlementInputFlags) applyTo(cmd *cobra.Command, current sdk.Entitlement) sdk.EntitlementInput {
	input := sdk.EntitlementInput{
		Name:                    current.Name,
		Description:             current.Description,
		Type:                    current.Type,
		AggregationMethod:       current.AggregationMethod,
		ResetPeriod:             current.ResetPeriod,
		ResetAnchor:             current.ResetAnchor,
		WarningThresholdPercent: current.WarningThresholdPercent,
		Icon:                    current.Icon,
		UnitSingular:            current.UnitSingular,
		UnitPlural:              current.UnitPlural,
		SaleUnitSingular:        current.SaleUnitSingular,
		SaleUnitPlural:          current.SaleUnitPlural,
		SaleUnitFactor:          current.SaleUnitFactor,
		UserFacing:              current.UserFacing,
		DisplayOrder:            current.DisplayOrder,
	}

	if flagChanged(cmd, "name") {
		input.Name = f.name
	}
	if flagChanged(cmd, "description") {
		input.Description = &f.description
	}
	if flagChanged(cmd, "type") {
		input.Type = optionalEnum[sdk.EntitlementType](cmd, "type", f.kind)
	}
	if flagChanged(cmd, "aggregation-method") {
		input.AggregationMethod = optionalEnum[sdk.EntitlementAggregationMethod](cmd, "aggregation-method", f.aggregationMethod)
	}
	if flagChanged(cmd, "group-slug") {
		input.GroupSlugs = f.groupSlugs
	}
	f.setOptional(cmd, &input)

	return input
}

// setOptional writes onto input the optional fields the invocation names, for create and
// update alike. An empty label or icon removes it: the field is left out of the request,
// which is how the API is told it has no value.
func (f *entitlementInputFlags) setOptional(cmd *cobra.Command, input *sdk.EntitlementInput) {
	label := func(flag, value string, field **string) {
		if !flagChanged(cmd, flag) {
			return
		}
		if value == "" {
			*field = nil
			return
		}
		*field = &value
	}
	label("icon", f.icon, &input.Icon)
	label("unit-singular", f.unitSingular, &input.UnitSingular)
	label("unit-plural", f.unitPlural, &input.UnitPlural)
	label("sale-unit-singular", f.saleUnitSingular, &input.SaleUnitSingular)
	label("sale-unit-plural", f.saleUnitPlural, &input.SaleUnitPlural)

	if flagChanged(cmd, "reset-period") {
		input.ResetPeriod = optionalEnum[sdk.EntitlementResetPeriod](cmd, "reset-period", f.resetPeriod)
	}
	if flagChanged(cmd, "reset-anchor") {
		input.ResetAnchor = optionalEnum[sdk.EntitlementResetAnchor](cmd, "reset-anchor", f.resetAnchor)
	}
	if flagChanged(cmd, "warning-threshold-percent") {
		input.WarningThresholdPercent = &f.warningThresholdPercent
	}
	if flagChanged(cmd, "sale-unit-factor") {
		input.SaleUnitFactor = &f.saleUnitFactor
	}
	if flagChanged(cmd, "user-facing") {
		input.UserFacing = &f.userFacing
	}
	if flagChanged(cmd, "display-order") {
		input.DisplayOrder = &f.displayOrder
	}
}
