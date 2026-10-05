package cmd

import (
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/kaitencloud/cli/internal/output"
	"github.com/kaitencloud/sdk-go"
	"github.com/spf13/cobra"
)

func newInstancesUsageHistoryCommand() *cobra.Command {
	var from, to, transactionID string
	var limit int32

	cmd := &cobra.Command{
		Use:   "history <instance-slug> <entitlement-slug>",
		Short: "List the usage reports of an instance's entitlement",
		Long: `List the usage reports Kaiten accepted for one instance and entitlement, in
the order it accepted them, with the counter before and after each one and the
limit it was gated on.

The range defaults to the last 30 days. A --from earlier than the start of the
organization's usage history is refused (exit code 5).`,
		Args: cobra.ExactArgs(2),
		Example: `  kaiten instances usage history acme-prod tokens
  kaiten instances usage history acme-prod tokens --from 2026-10-01T00:00:00Z --limit 100
  kaiten instances usage history acme-prod tokens --transaction-id llm-call:9f2c:tokens`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fromTime, err := parseTimeFlag(from, "from")
			if err != nil {
				return err
			}
			toTime, err := parseTimeFlag(to, "to")
			if err != nil {
				return err
			}

			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			items, err := client.Instances.ListUsageReports(ctx, args[0], args[1], &sdk.UsageReportsOptions{
				From:          fromTime,
				To:            toTime,
				TransactionID: transactionID,
				Limit:         optionalInt32(limit),
			})
			if err != nil {
				return err
			}

			rows := make([][]string, 0, len(items))
			for _, item := range items {
				rows = append(rows, []string{
					fmt.Sprint(item.ReportSeq),
					formatTimeValue(item.ReportedAt),
					string(item.Behavior),
					item.ReportedValue,
					item.ValueBefore,
					item.ValueAfter,
					item.Delta,
					stringOrEmpty(item.LimitValue),
					stringOrEmpty(item.TransactionId),
				})
			}
			return writeStructured(cmd, output.FormatTable, items, output.Table{
				Columns: []string{"SEQ", "REPORTED AT", "BEHAVIOR", "VALUE", "BEFORE", "AFTER", "DELTA", "LIMIT", "TRANSACTION ID"},
				Rows:    rows,
			})
		},
	}

	cmd.Flags().StringVar(&from, "from", "", "Only reports accepted at or after this RFC3339 timestamp (default: 30 days before --to)")
	cmd.Flags().StringVar(&to, "to", "", "Only reports accepted before this RFC3339 timestamp (default: now)")
	cmd.Flags().StringVar(&transactionID, "transaction-id", "", "Only the report sent with this idempotency key")
	cmd.Flags().Int32Var(&limit, "limit", 0, "Fetch at most this many reports")

	return cmd
}

func newInstancesUsageExportCommand() *cobra.Command {
	var from, to, format, outputFile string
	var instance, instanceID, entitlement, entitlementID string

	cmd := &cobra.Command{
		Use:   "export [<instance-slug> <entitlement-slug>]",
		Short: "Export usage reports as CSV or NDJSON",
		Long: `Stream usage reports as CSV (with a header row) or NDJSON (one report per
line), to stdout or to --output-file.

With an instance and an entitlement, the export covers that pair, up to 366 days
per run. Without them, it covers the whole organization, up to 31 days per run,
and --instance, --instance-id, --entitlement and --entitlement-id narrow it; the
ID filters reach deleted instances and entitlements, whose reports are kept.

The range defaults to the last 30 days. A --from earlier than the start of the
organization's usage history is refused (exit code 5). The --output flag does not
apply: the export is written as it arrives, in --format.`,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 0 && len(args) != 2 {
				return fmt.Errorf("accepts either no arguments or <instance-slug> <entitlement-slug>, received %d", len(args))
			}
			return nil
		},
		Example: `  # One instance's tokens, last 30 days, as CSV on stdout
  kaiten instances usage export acme-prod tokens > tokens.csv

  # The whole organization for September, as NDJSON in a file
  kaiten instances usage export --from 2026-09-01T00:00:00Z --to 2026-10-01T00:00:00Z \
    --format json --output-file usage-september.ndjson

  # A deleted instance's reports, by ID
  kaiten instances usage export --instance-id 3f1c... --output-file old-instance.csv`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 2 && anyFlagChanged(cmd, "instance", "instance-id", "entitlement", "entitlement-id") {
				return errUsagef("--instance, --instance-id, --entitlement and --entitlement-id narrow an organization export; drop them, or drop the two arguments")
			}
			if format != string(sdk.UsageExportCSV) && format != string(sdk.UsageExportJSON) {
				return errUsagef("--format must be csv or json")
			}
			fromTime, err := parseTimeFlag(from, "from")
			if err != nil {
				return err
			}
			toTime, err := parseTimeFlag(to, "to")
			if err != nil {
				return err
			}

			client, err := newStreamingClient(cmd)
			if err != nil {
				return err
			}
			ctx := cmd.Context()

			var export *sdk.UsageExport
			if len(args) == 2 {
				export, err = client.Instances.ExportUsageReports(ctx, args[0], args[1], &sdk.UsageExportOptions{
					From: fromTime, To: toTime, Format: sdk.UsageExportFormat(format),
				})
			} else {
				export, err = client.Instances.ExportOrganizationUsageReports(ctx, &sdk.OrganizationUsageExportOptions{
					From: fromTime, To: toTime, Format: sdk.UsageExportFormat(format),
					InstanceSlug: instance, InstanceID: instanceID,
					EntitlementSlug: entitlement, EntitlementID: entitlementID,
				})
			}
			if err != nil {
				return err
			}
			defer func() { _ = export.Body.Close() }()

			return writeExport(cmd, export, outputFile)
		},
	}

	cmd.Flags().StringVar(&from, "from", "", "Only reports accepted at or after this RFC3339 timestamp (default: 30 days before --to)")
	cmd.Flags().StringVar(&to, "to", "", "Only reports accepted before this RFC3339 timestamp (default: now)")
	cmd.Flags().StringVar(&format, "format", string(sdk.UsageExportCSV), "Export format: csv or json (NDJSON)")
	cmd.Flags().StringVar(&outputFile, "output-file", "", "Write the export to this file instead of stdout")
	cmd.Flags().StringVar(&instance, "instance", "", "Organization export: only this instance (slug)")
	cmd.Flags().StringVar(&instanceID, "instance-id", "", "Organization export: only this instance (ID; reaches a deleted one)")
	cmd.Flags().StringVar(&entitlement, "entitlement", "", "Organization export: only this entitlement (slug)")
	cmd.Flags().StringVar(&entitlementID, "entitlement-id", "", "Organization export: only this entitlement (ID; reaches a deleted one)")

	return cmd
}

// writeExport copies an export to outputFile, or to stdout when it is empty, as
// it arrives. A file is written beside its final name and renamed into place
// once complete, so an interrupted export never leaves a truncated file that
// looks finished.
func writeExport(cmd *cobra.Command, export *sdk.UsageExport, outputFile string) error {
	if outputFile == "" {
		_, err := io.Copy(cmd.OutOrStdout(), export.Body)
		return err
	}

	partial := outputFile + ".partial"
	file, err := os.Create(partial) //nolint:gosec // the path is the caller's own --output-file
	if err != nil {
		return fmt.Errorf("create %s: %w", partial, err)
	}
	written, copyErr := io.Copy(file, export.Body)
	closeErr := file.Close()
	if err := firstError(copyErr, closeErr); err != nil {
		_ = os.Remove(partial)
		return fmt.Errorf("write %s: %w", outputFile, err)
	}
	if err := os.Rename(partial, outputFile); err != nil {
		return fmt.Errorf("write %s: %w", outputFile, err)
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "wrote %d bytes to %s\n", written, outputFile)
	return nil
}

// newStreamingClient is newClient for a command whose response body can take
// longer to read than any fixed deadline: an export. Its HTTP client has no
// overall timeout -- the server must still start answering within
// defaultTimeout -- and the command runs on its own context, which a signal
// still cancels, rather than commandContext's.
func newStreamingClient(cmd *cobra.Command) (*sdk.Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone() //nolint:forcetypeassert // the standard library's own type
	transport.ResponseHeaderTimeout = defaultTimeout

	client, _, err := newClient(cmd, sdk.WithHTTPClient(&http.Client{Transport: transport}))
	return client, err
}

func stringOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func firstError(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}
