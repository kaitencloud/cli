package cmd

import (
	"fmt"

	"github.com/kaitencloud/cli/internal/input"
	"github.com/kaitencloud/cli/internal/output"
	"github.com/kaitencloud/sdk-go"
	"github.com/spf13/cobra"
)

func newInstancesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "instances",
		Short: "Manage instances",
		Args:  cobra.NoArgs,
		RunE:  runHelp,
	}

	cmd.AddCommand(newInstancesListCommand())
	cmd.AddCommand(newInstancesGetCommand())
	cmd.AddCommand(newInstancesCreateCommand())
	cmd.AddCommand(newInstancesUpdateCommand())
	cmd.AddCommand(newInstancesDeleteCommand())
	cmd.AddCommand(newInstancesAuditTrailsCommand())
	cmd.AddCommand(newInstancesUsageCommand())

	return cmd
}

func newInstancesListCommand() *cobra.Command {
	var includeDeleted bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List instances",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			items, err := client.Instances.List(ctx, &sdk.InstancesListOptions{IncludeDeleted: includeDeleted})
			if err != nil {
				return err
			}
			rows := make([][]string, 0, len(items))
			for _, item := range items {
				rows = append(rows, []string{
					item.Name,
					deref(item.Slug),
					deref(item.CustomerSlug),
					deref(item.LicenseSlug),
				})
			}
			return writeStructured(cmd, output.FormatTable, items, output.Table{
				Columns: []string{"NAME", "SLUG", "CUSTOMER", "LICENSE"},
				Rows:    rows,
			})
		},
	}
	cmd.Flags().BoolVar(&includeDeleted, "include-deleted", false, "Include soft-deleted instances")
	return cmd
}

func newInstancesGetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "get <instance-slug>",
		Short: "Get an instance",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			item, err := client.Instances.Get(ctx, args[0])
			if err != nil {
				return err
			}
			return writeStructured(cmd, output.FormatYAML, item, output.Table{})
		},
	}
}

func newInstancesCreateCommand() *cobra.Command {
	var file, payload string
	var inline instanceInputFlags
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an instance from --file, --payload, or inline flags",
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
			item, err := client.Instances.Create(ctx, inputValue)
			if err != nil {
				return err
			}
			return writeStructured(cmd, output.FormatYAML, item, output.Table{})
		},
	}
	addInputSourceFlags(cmd, &file, &payload, "instance")
	inline.register(cmd)
	return cmd
}

func newInstancesUpdateCommand() *cobra.Command {
	var file, payload string
	var inline instanceInputFlags
	cmd := &cobra.Command{
		Use:   "update <instance-slug>",
		Short: "Update an instance from --file, --payload, or inline flags",
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
			if err := client.Instances.Update(ctx, args[0], inputValue); err != nil {
				return err
			}
			return writeMessage(cmd, "Instance updated")
		},
	}
	addInputSourceFlags(cmd, &file, &payload, "instance")
	inline.register(cmd)
	return cmd
}

func newInstancesDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <instance-slug>",
		Short: "Delete an instance",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			if err := confirmDestructive(cmd, "delete", fmt.Sprintf("instance %q", args[0])); err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			if err := client.Instances.Delete(ctx, args[0]); err != nil {
				return err
			}
			return writeMessage(cmd, "Instance deleted")
		},
	}
	addConfirmFlag(cmd)
	return cmd
}

func newInstancesAuditTrailsCommand() *cobra.Command {
	var eventName string
	var after string
	var before string
	var limit int32
	var offset int32

	cmd := &cobra.Command{
		Use:   "audit-trails <instance-slug>",
		Short: "List instance audit trails",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			afterTime, err := parseTimeFlag(after, "after")
			if err != nil {
				return err
			}
			beforeTime, err := parseTimeFlag(before, "before")
			if err != nil {
				return err
			}

			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			items, err := client.Instances.ListAuditTrails(ctx, args[0], &sdk.AuditTrailsOptions{
				EventName: eventName,
				After:     afterTime,
				Before:    beforeTime,
				Limit:     optionalInt32(limit),
				Offset:    optionalInt32(offset),
			})
			if err != nil {
				return err
			}
			rows := make([][]string, 0, len(items))
			for _, item := range items {
				rows = append(rows, []string{
					item.Id,
					item.EventName,
					item.EventType,
					formatTimeValue(item.Timestamp),
				})
			}
			return writeStructured(cmd, output.FormatTable, items, output.Table{
				Columns: []string{"ID", "EVENT", "EVENT TYPE", "TIMESTAMP"},
				Rows:    rows,
			})
		},
	}

	cmd.Flags().StringVar(&eventName, "event-name", "", "Filter by event name")
	cmd.Flags().StringVar(&after, "after", "", "Only include events after this RFC3339 timestamp")
	cmd.Flags().StringVar(&before, "before", "", "Only include events before this RFC3339 timestamp")
	cmd.Flags().Int32Var(&limit, "limit", 0, "Limit results")
	cmd.Flags().Int32Var(&offset, "offset", 0, "Offset results")

	return cmd
}

func newInstancesUsageCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "usage",
		Short: "Inspect or report instance entitlement usage",
		Args:  cobra.NoArgs,
		RunE:  runHelp,
	}
	cmd.AddCommand(newInstancesUsageListCommand())
	cmd.AddCommand(newInstancesUsageGetCommand())
	cmd.AddCommand(newInstancesUsageReportCommand())
	return cmd
}

func newInstancesUsageListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list <instance-slug>",
		Short: "List entitlement usage metrics for an instance",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			items, err := client.Instances.ListEntitlementUsageMetrics(ctx, args[0])
			if err != nil {
				return err
			}
			rows := make([][]string, 0, len(items))
			for _, item := range items {
				rows = append(rows, []string{
					item.EntitlementSlug,
					item.LicenseSlug,
					jsonString(item.Value),
				})
			}
			return writeStructured(cmd, output.FormatTable, items, output.Table{
				Columns: []string{"ENTITLEMENT", "LICENSE", "VALUE"},
				Rows:    rows,
			})
		},
	}
}

func newInstancesUsageGetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "get <instance-slug> <entitlement-slug>",
		Short: "Get one entitlement usage metric for an instance",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			item, err := client.Instances.GetEntitlementUsageMetric(ctx, args[0], args[1])
			if err != nil {
				return err
			}
			return writeStructured(cmd, output.FormatYAML, item, output.Table{})
		},
	}
}

func newInstancesUsageReportCommand() *cobra.Command {
	var file string
	var payload string
	var value float64
	var behavior string
	var metadataFile string
	var metadataPayload string

	cmd := &cobra.Command{
		Use:   "report <instance-slug> <entitlement-slug> --value <number>",
		Short: "Report entitlement usage for an instance",
		Args:  cobra.ExactArgs(2),
		Example: `  # Add to the recorded usage
  kaiten instances usage report acme-prod seats --value 3

  # Overwrite it instead, and attach metadata
  kaiten instances usage report acme-prod seats --value 12 --behavior set \
    --metadata-payload '{"source":"nightly-sync"}'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			hasInline := anyFlagChanged(cmd, "value", "behavior", "metadata-file", "metadata-payload")
			inputValue, err := resolveInput(file, payload, hasInline, func() (sdk.UsageReportInput, error) {
				if err := requiredInlineFlag(cmd, "value"); err != nil {
					return sdk.UsageReportInput{}, err
				}
				metadata, err := loadUsageMetadata(metadataFile, metadataPayload)
				if err != nil {
					return sdk.UsageReportInput{}, err
				}

				return sdk.UsageReportInput{
					Value:    value,
					Behavior: sdk.EntitlementUsageBehavior(behavior),
					Metadata: metadata,
				}, nil
			})
			if err != nil {
				return err
			}

			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			item, err := client.Instances.ReportEntitlementUsageMetric(ctx, args[0], args[1], inputValue)
			if err != nil {
				return err
			}
			return writeStructured(cmd, output.FormatYAML, item, output.Table{})
		},
	}
	addInputSourceFlags(cmd, &file, &payload, "usage report")
	cmd.Flags().Float64Var(&value, "value", 0, "Usage value to report")
	cmd.Flags().StringVar(&behavior, "behavior", string(sdk.Append), "Usage behavior: append or set")
	cmd.Flags().StringVar(&metadataFile, "metadata-file", "", "Optional JSON or YAML metadata file")
	cmd.Flags().StringVar(&metadataPayload, "metadata-payload", "", "Optional inline JSON or YAML metadata object")
	return cmd
}

// loadUsageMetadata reads the optional metadata object of a usage report from whichever of
// the two mutually exclusive flags carries it, or returns nil when neither does.
func loadUsageMetadata(metadataFile, metadataPayload string) (map[string]any, error) {
	switch {
	case metadataFile != "" && metadataPayload != "":
		return nil, errUsagef("use only one of --metadata-file or --metadata-payload")
	case metadataFile != "":
		return input.LoadFile[map[string]any](metadataFile)
	case metadataPayload != "":
		return input.LoadString[map[string]any](metadataPayload)
	default:
		return nil, nil
	}
}

// optionalInt32 omits a paging bound the caller left at its zero value: the API applies its
// own default for an absent limit or offset, which sending 0 would override.
func optionalInt32(value int32) *int32 {
	if value == 0 {
		return nil
	}
	return &value
}

// instanceInputFlags is the inline form of an instance payload, shared by create and update.
type instanceInputFlags struct {
	name             string
	description      string
	customerID       string
	licenseID        string
	deploymentZoneID string
	metadata         string
	startLicenseDate string
	endLicenseDate   string
	slug             string
}

func (f *instanceInputFlags) register(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.name, "name", "", "Instance name")
	cmd.Flags().StringVar(&f.description, "description", "", "Instance description")
	cmd.Flags().StringVar(&f.customerID, "customer-id", "", "Customer ID")
	cmd.Flags().StringVar(&f.licenseID, "license-id", "", "License ID")
	cmd.Flags().StringVar(&f.deploymentZoneID, "deployment-zone-id", "", "Deployment zone ID")
	cmd.Flags().StringVar(&f.metadata, "metadata-json", "", "Metadata as inline JSON or YAML object")
	cmd.Flags().StringVar(&f.startLicenseDate, "start-license-date", "", "Start license date in RFC3339 format")
	cmd.Flags().StringVar(&f.endLicenseDate, "end-license-date", "", "End license date in RFC3339 format")
	cmd.Flags().StringVar(&f.slug, "slug", "", "Instance slug")
}

func (f *instanceInputFlags) changed(cmd *cobra.Command) bool {
	return anyFlagChanged(cmd,
		"name", "description", "customer-id", "license-id", "deployment-zone-id",
		"metadata-json", "start-license-date", "end-license-date", "slug",
	)
}

func (f *instanceInputFlags) build(cmd *cobra.Command) func() (sdk.InstanceInput, error) {
	return func() (sdk.InstanceInput, error) {
		for _, required := range []struct{ value, flag string }{
			{f.name, "name"},
			{f.description, "description"},
			{f.customerID, "customer-id"},
			{f.licenseID, "license-id"},
		} {
			if err := requiredInlineString(required.value, required.flag); err != nil {
				return sdk.InstanceInput{}, err
			}
		}

		startLicenseDate, err := requiredInlineTime(f.startLicenseDate, "start-license-date")
		if err != nil {
			return sdk.InstanceInput{}, err
		}
		endLicenseDate, err := requiredInlineTime(f.endLicenseDate, "end-license-date")
		if err != nil {
			return sdk.InstanceInput{}, err
		}

		metadata, err := parseRequiredMapFlag(f.metadata, "metadata-json")
		if err != nil {
			return sdk.InstanceInput{}, err
		}

		return sdk.InstanceInput{
			Name:             f.name,
			Description:      f.description,
			CustomerID:       f.customerID,
			LicenseID:        f.licenseID,
			DeploymentZoneID: optionalString(cmd, "deployment-zone-id", f.deploymentZoneID),
			Metadata:         metadata,
			StartLicenseDate: startLicenseDate,
			EndLicenseDate:   endLicenseDate,
			Slug:             optionalString(cmd, "slug", f.slug),
		}, nil
	}
}
