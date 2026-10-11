package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/kaitencloud/cli/internal/output"
	"github.com/kaitencloud/sdk-go"
	"github.com/spf13/cobra"
)

func newBillingCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "billing",
		Short: "Manage billing: invoices, the accounting handoff, subscriptions and the catalogue",
		Args:  cobra.NoArgs,
		RunE:  runHelp,
	}

	cmd.AddCommand(
		newBillingSettingsCommand(),
		billingCall{
			use: "capabilities", short: "Show what the configured billing provider can do",
			run: func(ctx context.Context, c *sdk.Client, _ []string) (any, error) { return c.Billing.Capabilities(ctx) },
		}.command(),
		billingCall{
			use: "health", short: "Show the billing provider's health and the work waiting on it",
			run: func(ctx context.Context, c *sdk.Client, _ []string) (any, error) { return c.Billing.Health(ctx) },
		}.command(),
		billingCall{
			use: "sync", short: "Reconcile with the billing provider, in the background",
			run: func(ctx context.Context, c *sdk.Client, _ []string) (any, error) { return c.Billing.Sync(ctx) },
		}.command(),
		billingBodyCall[sdk.ClosePeriodsScope]{
			use: "close-periods", short: "Close the billing periods whose boundary has passed",
			optional: true,
			run: func(ctx context.Context, c *sdk.Client, _ []string, in sdk.ClosePeriodsScope) (any, error) {
				return c.Billing.ClosePeriods(ctx, in)
			},
		}.command(),
		newBillingHandoffCommand(),
		newBillingInvoicesCommand(),
		newBillingSubscriptionsCommand(),
		newBillingPricesCommand(),
		newBillingAddonsCommand(),
		newBillingInstanceAddonsCommand(),
		newBillingVouchersCommand(),
		newBillingKeysCommand(),
		newBillingSessionsCommand(),
		newBillingCustomersCommand(),
	)

	return cmd
}

func groupCommand(use, short string, children ...*cobra.Command) *cobra.Command {
	cmd := &cobra.Command{Use: use, Short: short, Args: cobra.NoArgs, RunE: runHelp}
	cmd.AddCommand(children...)
	return cmd
}

// billingCall is a billing command with positional arguments and no body. A
// run that answers nil reports message instead.
type billingCall struct {
	use, short, long, message string
	destructive               string
	run                       func(ctx context.Context, c *sdk.Client, args []string) (any, error)
}

func (b billingCall) command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   b.use,
		Short: b.short,
		Long:  b.long,
		Args:  cobra.ExactArgs(positionalCount(b.use)),
		RunE: func(cmd *cobra.Command, args []string) error {
			if b.destructive != "" {
				if err := confirmDestructive(cmd, b.destructive, strings.Join(args, " ")); err != nil {
					return err
				}
			}
			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			value, err := b.run(ctx, client, args)
			if err != nil {
				return err
			}
			return writeResult(cmd, value, b.message)
		},
	}
	if b.destructive != "" {
		addConfirmFlag(cmd)
	}
	return cmd
}

// billingBodyCall is a billing command whose request body comes from --file or
// --payload. optional lets the body be left out, as an empty document.
type billingBodyCall[T any] struct {
	use, short, long, message string
	destructive               string
	optional                  bool
	run                       func(ctx context.Context, c *sdk.Client, args []string, in T) (any, error)
}

func (b billingBodyCall[T]) command() *cobra.Command {
	var file, payload string
	cmd := &cobra.Command{
		Use:   b.use,
		Short: b.short,
		Long:  b.long,
		Args:  cobra.ExactArgs(positionalCount(b.use)),
		RunE: func(cmd *cobra.Command, args []string) error {
			var in T
			if !b.optional || file != "" || payload != "" {
				var err error
				if in, err = resolveFileOrPayload[T](file, payload); err != nil {
					return err
				}
			}
			if b.destructive != "" {
				if err := confirmDestructive(cmd, b.destructive, strings.Join(args, " ")); err != nil {
					return err
				}
			}
			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			value, err := b.run(ctx, client, args, in)
			if err != nil {
				return err
			}
			return writeResult(cmd, value, b.message)
		},
	}
	addInputSourceFlags(cmd, &file, &payload, "request")
	if b.destructive != "" {
		addConfirmFlag(cmd)
	}
	return cmd
}

func positionalCount(use string) int {
	count := 0
	for _, field := range strings.Fields(use)[1:] {
		if strings.HasPrefix(field, "<") {
			count++
		}
	}
	return count
}

func writeResult(cmd *cobra.Command, value any, message string) error {
	if value == nil {
		return writeMessage(cmd, message)
	}
	return writeStructured(cmd, output.FormatYAML, value, output.Table{})
}

func newBillingSettingsCommand() *cobra.Command {
	return groupCommand("settings", "Show or change the organization's billing settings",
		billingCall{
			use: "get", short: "Show the billing settings",
			run: func(ctx context.Context, c *sdk.Client, _ []string) (any, error) { return c.Billing.Settings(ctx) },
		}.command(),
		billingBodyCall[sdk.BillingSettings]{
			use: "update", short: "Change the billing settings (a member left out is left alone)",
			run: func(ctx context.Context, c *sdk.Client, _ []string, in sdk.BillingSettings) (any, error) {
				return c.Billing.UpdateSettings(ctx, in)
			},
		}.command(),
	)
}

// --- Handoff ---------------------------------------------------------------

func newBillingHandoffCommand() *cobra.Command {
	var status string
	list := &cobra.Command{
		Use:   "list",
		Short: "List the invoices waiting for, or leased to, the accounting handoff",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			items, err := client.Billing.Handoff.List(ctx, &sdk.ListHandoffParams{Status: optionalEnum[sdk.ListHandoffParamsStatus](cmd, "status", status)})
			if err != nil {
				return err
			}
			rows := make([][]string, 0, len(items))
			for _, item := range items {
				rows = append(rows, []string{item.Id, item.CustomerSlug, formatMoney(item.Total, item.Currency), formatTime(item.IssuedAt), string(item.HandoffStatus), formatTime(item.Handoff.LeasedUntil)})
			}
			return writeStructured(cmd, output.FormatTable, items, output.Table{
				Columns: []string{"INVOICE", "CUSTOMER", "TOTAL", "ISSUED", "HANDOFF", "LEASED UNTIL"},
				Rows:    rows,
			})
		},
	}
	list.Flags().StringVar(&status, "status", "", "Only PENDING or ACKNOWLEDGED invoices")

	return groupCommand("handoff", "Hand invoices off to an accounting system: claim a leased batch, acknowledge each one",
		list, newBillingHandoffClaimCommand(), newBillingHandoffAckCommand())
}

func newBillingHandoffClaimCommand() *cobra.Command {
	var limit int32
	var lease time.Duration

	cmd := &cobra.Command{
		Use:   "claim",
		Short: "Lease a batch of invoices to hand off",
		Long: `Lease up to --limit invoices to hand off, oldest issue first. Each stays
leased to this claim for --lease: acknowledge it with "kaiten billing handoff ack"
before then, or it is offered to the next claim.

--output ndjson prints one invoice per line, each carrying its lease in
handoff.leaseId: the form a booking script reads line by line.`,
		Args: cobra.NoArgs,
		Example: `  kaiten billing handoff claim --limit 50 --lease 30m --output ndjson |
    while read -r invoice; do book "$invoice"; done`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if lease < time.Minute || lease > time.Hour {
				return errUsagef("--lease must be between 1m and 1h")
			}
			if limit < 1 || limit > 100 {
				return errUsagef("--limit must be between 1 and 100")
			}
			client, cfg, err := newClient(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			seconds := int32(lease / time.Second)
			claim, err := client.Billing.Handoff.Claim(ctx, sdk.HandoffClaimSize{Limit: &limit, LeaseSeconds: &seconds})
			if err != nil {
				return err
			}
			if cfg.Output == output.FormatNDJSON {
				return writeStructured(cmd, output.FormatNDJSON, claim.Invoices, output.Table{})
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "lease %s, until %s\n", claim.LeaseId, formatTimeValue(claim.LeasedUntil))
			rows := make([][]string, 0, len(claim.Invoices))
			for _, invoice := range claim.Invoices {
				rows = append(rows, []string{invoice.Id, invoice.CustomerSlug, deref(invoice.BillingEmail), formatMoney(invoice.Total, invoice.Currency), formatTime(invoice.IssuedAt)})
			}
			return writeStructured(cmd, output.FormatTable, claim, output.Table{
				Columns: []string{"INVOICE", "CUSTOMER", "BILLING EMAIL", "TOTAL", "ISSUED"},
				Rows:    rows,
			})
		},
	}
	cmd.Flags().Int32Var(&limit, "limit", 25, "Invoices to lease, 1 to 100")
	cmd.Flags().DurationVar(&lease, "lease", 15*time.Minute, "How long the invoices stay leased to this claim, 1m to 1h")
	return cmd
}

func newBillingHandoffAckCommand() *cobra.Command {
	var leaseID, reference string

	cmd := &cobra.Command{
		Use:   "ack <invoice-id>",
		Short: "Acknowledge the handoff of an invoice",
		Long: `Acknowledge that the accounting system booked an invoice. With --lease-id, an
invoice another claim took since is refused (exit code 5) rather than
acknowledged twice.`,
		Args:    cobra.ExactArgs(1),
		Example: `  kaiten billing handoff ack inv_123 --lease-id lease_456 --external-reference ACC-2027-0001`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			invoice, err := client.Billing.Handoff.Ack(ctx, args[0], sdk.HandoffBooking{
				LeaseId:           optionalString(cmd, "lease-id", leaseID),
				ExternalReference: optionalString(cmd, "external-reference", reference),
			})
			if err != nil {
				return err
			}
			return writeInvoice(cmd, invoice)
		},
	}
	cmd.Flags().StringVar(&leaseID, "lease-id", "", "The claim's lease: refuse the acknowledgement if another claim took the invoice")
	cmd.Flags().StringVar(&reference, "external-reference", "", "The invoice's number in the accounting system")
	return cmd
}

// --- Invoices --------------------------------------------------------------

func newBillingInvoicesCommand() *cobra.Command {
	return groupCommand("invoices", "List, export and settle invoices",
		newBillingInvoicesListCommand(),
		billingCall{
			use: "get <invoice-id>", short: "Show an invoice with its lines",
			run: func(ctx context.Context, c *sdk.Client, args []string) (any, error) {
				return c.Billing.Invoices.Get(ctx, args[0])
			},
		}.command(),
		newBillingInvoicesExportCommand(),
		newBillingInvoicesMarkPaidCommand(),
		newBillingInvoicesReasonCommand("void", "Void an invoice", "void invoice", true,
			func(ctx context.Context, c *sdk.Client, id, reason string) (sdk.Invoice, error) {
				return c.Billing.Invoices.Void(ctx, id, sdk.InvoiceVoid{Reason: reason})
			}),
		newBillingInvoicesReasonCommand("write-off", "Write an invoice off as uncollectable", "write off invoice", true,
			func(ctx context.Context, c *sdk.Client, id, reason string) (sdk.Invoice, error) {
				return c.Billing.Invoices.WriteOff(ctx, id, sdk.InvoiceWriteOff{Reason: reason})
			}),
		newBillingInvoicesReasonCommand("release-hold", "Release an invoice held for review", "", false,
			func(ctx context.Context, c *sdk.Client, id, reason string) (sdk.Invoice, error) {
				return c.Billing.Invoices.ReleaseHold(ctx, id, sdk.HoldRelease{Reason: reason})
			}),
		billingCall{
			use: "recompose <invoice-id>", short: "Compose a draft invoice again from what is known now",
			run: func(ctx context.Context, c *sdk.Client, args []string) (any, error) {
				return c.Billing.Invoices.Recompose(ctx, args[0])
			},
		}.command(),
		billingCall{
			use: "retry-push <invoice-id>", short: "Push an invoice to the billing provider again, in the background",
			run: func(ctx context.Context, c *sdk.Client, args []string) (any, error) {
				return c.Billing.Invoices.RetryPush(ctx, args[0])
			},
		}.command(),
		billingCall{
			use: "sync <invoice-id>", short: "Read an invoice's state back from the billing provider",
			run: func(ctx context.Context, c *sdk.Client, args []string) (any, error) {
				return c.Billing.Invoices.Sync(ctx, args[0])
			},
		}.command(),
		newBillingInvoicesLineReportsCommand(),
	)
}

func writeInvoice(cmd *cobra.Command, invoice sdk.Invoice) error {
	return writeStructured(cmd, output.FormatTable, invoice, output.Table{
		Columns: []string{"INVOICE", "STATUS", "KIND", "CUSTOMER", "INSTANCE", "TOTAL", "ISSUED", "DUE", "HANDOFF"},
		Rows: [][]string{{
			invoice.Id, string(invoice.Status), string(invoice.Kind), invoice.CustomerSlug, invoice.InstanceSlug,
			formatMoney(invoice.Total, invoice.Currency), formatTime(invoice.IssuedAt), formatTime(invoice.DueAt), string(invoice.HandoffStatus),
		}},
	})
}

// formatMoney renders an amount in the currency's minor units, as the API
// carries it: the CLI does not guess a currency's exponent.
func formatMoney(minor int64, currency string) string {
	return fmt.Sprintf("%d %s", minor, strings.ToUpper(currency))
}

func newBillingInvoicesListCommand() *cobra.Command {
	var statuses []string
	var overdue, held bool
	var customer, instance, from, to, updatedSince, kind, handoff string
	var limit int32

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the organization's invoices",
		Long: `List invoices, newest first. --from and --to bound the issue date;
--updated-since lists what changed since a previous run, for a sync.`,
		Args: cobra.NoArgs,
		Example: `  kaiten billing invoices list --overdue
  kaiten billing invoices list --status PUSH_FAILED --status PAYMENT_FAILED --customer acme
  kaiten billing invoices list --updated-since 2027-01-01T00:00:00Z --output json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			params := sdk.ListInvoicesParams{
				CustomerSlug:  optionalString(cmd, "customer", customer),
				InstanceSlug:  optionalString(cmd, "instance", instance),
				Kind:          optionalEnum[sdk.ListInvoicesParamsKind](cmd, "kind", kind),
				HandoffStatus: optionalEnum[sdk.ListInvoicesParamsHandoffStatus](cmd, "handoff-status", handoff),
			}
			if flagChanged(cmd, "overdue") {
				params.Overdue = &overdue
			}
			if flagChanged(cmd, "held") {
				params.Held = &held
			}
			if len(statuses) > 0 {
				values := make([]sdk.ListInvoicesParamsStatus, 0, len(statuses))
				for _, status := range statuses {
					values = append(values, sdk.ListInvoicesParamsStatus(strings.ToUpper(status)))
				}
				params.Status = &values
			}
			var err error
			if params.IssuedFrom, err = parseTimeFlag(from, "from"); err != nil {
				return err
			}
			if params.IssuedTo, err = parseTimeFlag(to, "to"); err != nil {
				return err
			}
			if params.UpdatedSince, err = parseTimeFlag(updatedSince, "updated-since"); err != nil {
				return err
			}

			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			var items []sdk.InvoiceSummary
			if limit > 0 {
				params.Limit = &limit
				page, err := client.Billing.Invoices.ListPage(ctx, &params)
				if err != nil {
					return err
				}
				items = page.Items
			} else if items, err = client.Billing.Invoices.List(ctx, &params); err != nil {
				return err
			}

			rows := make([][]string, 0, len(items))
			for _, item := range items {
				rows = append(rows, []string{
					item.Id, string(item.Status), string(item.Kind), item.CustomerSlug, item.InstanceSlug,
					formatMoney(item.Total, item.Currency), formatTime(item.IssuedAt), formatTime(item.DueAt), string(item.HandoffStatus),
				})
			}
			return writeStructured(cmd, output.FormatTable, items, output.Table{
				Columns: []string{"INVOICE", "STATUS", "KIND", "CUSTOMER", "INSTANCE", "TOTAL", "ISSUED", "DUE", "HANDOFF"},
				Rows:    rows,
			})
		},
	}
	cmd.Flags().StringArrayVar(&statuses, "status", nil, "Only invoices in this status (repeatable): DRAFT, MANUAL, PUSHED, PUSH_FAILED, PAID, PAYMENT_FAILED, UNCOLLECTIBLE, VOID")
	cmd.Flags().BoolVar(&overdue, "overdue", false, "Only invoices past their due date and unpaid")
	cmd.Flags().BoolVar(&held, "held", false, "Only invoices held for review")
	cmd.Flags().StringVar(&customer, "customer", "", "Only this customer's invoices (slug)")
	cmd.Flags().StringVar(&instance, "instance", "", "Only this instance's invoices (slug)")
	cmd.Flags().StringVar(&kind, "kind", "", "Only invoices of this kind")
	cmd.Flags().StringVar(&handoff, "handoff-status", "", "Only invoices in this handoff status: NOT_REQUIRED, PENDING, ACKNOWLEDGED")
	cmd.Flags().StringVar(&from, "from", "", "Only invoices issued at or after this RFC3339 timestamp")
	cmd.Flags().StringVar(&to, "to", "", "Only invoices issued before this RFC3339 timestamp")
	cmd.Flags().StringVar(&updatedSince, "updated-since", "", "Only invoices changed at or after this RFC3339 timestamp")
	cmd.Flags().Int32Var(&limit, "limit", 0, "Fetch one page of at most this many invoices (default: all of them)")
	return cmd
}

func newBillingInvoicesExportCommand() *cobra.Command {
	var from, to, format, granularity, outputFile, customer string

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export the invoices issued in a range as CSV or JSON",
		Long: `Export the invoices issued in [--from, --to), one row per line or per invoice,
to stdout or to --output-file. The --output flag does not apply: the document is
written as the API sends it, in --format.`,
		Args:    cobra.NoArgs,
		Example: `  kaiten billing invoices export --from 2027-01-01T00:00:00Z --to 2027-02-01T00:00:00Z --output-file january.csv`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if format != "csv" && format != "json" {
				return errUsagef("--format must be csv or json")
			}
			if granularity != "line" && granularity != "invoice" {
				return errUsagef("--granularity must be line or invoice")
			}
			if from == "" || to == "" {
				return errUsagef("--from and --to are required")
			}
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
			document, err := client.Billing.Invoices.Export(ctx, &sdk.ExportInvoicesParams{
				IssuedFrom: fromTime, IssuedTo: toTime, Format: &format, Granularity: &granularity,
				CustomerSlug: optionalString(cmd, "customer", customer),
			})
			if err != nil {
				return err
			}
			if outputFile == "" {
				_, err := cmd.OutOrStdout().Write(document)
				return err
			}
			partial := outputFile + ".partial"
			if err := os.WriteFile(partial, document, 0o600); err != nil {
				return fmt.Errorf("write %s: %w", outputFile, err)
			}
			if err := os.Rename(partial, outputFile); err != nil {
				_ = os.Remove(partial)
				return fmt.Errorf("write %s: %w", outputFile, err)
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "wrote %d bytes to %s\n", len(document), outputFile)
			return nil
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "Invoices issued at or after this RFC3339 timestamp (required)")
	cmd.Flags().StringVar(&to, "to", "", "Invoices issued before this RFC3339 timestamp (required)")
	cmd.Flags().StringVar(&format, "format", "csv", "Export format: csv or json")
	cmd.Flags().StringVar(&granularity, "granularity", "line", "One row per line or per invoice")
	cmd.Flags().StringVar(&customer, "customer", "", "Only this customer's invoices (slug)")
	cmd.Flags().StringVar(&outputFile, "output-file", "", "Write the export to this file instead of stdout")
	return cmd
}

func newBillingInvoicesMarkPaidCommand() *cobra.Command {
	var paidAt, reference, note string

	cmd := &cobra.Command{
		Use:   "mark-paid <invoice-id>",
		Short: "Record an invoice as paid outside the billing provider",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			at, err := parseTimeFlag(paidAt, "paid-at")
			if err != nil {
				return err
			}
			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			invoice, err := client.Billing.Invoices.MarkPaid(ctx, args[0], sdk.InvoicePayment{
				PaidAt:            at,
				ExternalReference: optionalString(cmd, "external-reference", reference),
				Note:              optionalString(cmd, "note", note),
			})
			if err != nil {
				return err
			}
			return writeInvoice(cmd, invoice)
		},
	}
	cmd.Flags().StringVar(&paidAt, "paid-at", "", "When it was paid, RFC3339 (default: now)")
	cmd.Flags().StringVar(&reference, "external-reference", "", "The payment's reference, e.g. a bank transfer's")
	cmd.Flags().StringVar(&note, "note", "", "A note kept with the payment")
	return cmd
}

func newBillingInvoicesReasonCommand(use, short, destructive string, confirm bool, run func(ctx context.Context, c *sdk.Client, id, reason string) (sdk.Invoice, error)) *cobra.Command {
	var reason string

	cmd := &cobra.Command{
		Use:   use + " <invoice-id>",
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(reason) == "" {
				return errUsagef("--reason is required")
			}
			if confirm {
				if err := confirmDestructive(cmd, destructive, args[0]); err != nil {
					return err
				}
			}
			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			invoice, err := run(ctx, client, args[0], reason)
			if err != nil {
				return err
			}
			return writeInvoice(cmd, invoice)
		},
	}
	cmd.Flags().StringVar(&reason, "reason", "", "Why, kept on the invoice (required)")
	if confirm {
		addConfirmFlag(cmd)
	}
	return cmd
}

func newBillingInvoicesLineReportsCommand() *cobra.Command {
	var afterSeq int64
	var limit int32

	cmd := &cobra.Command{
		Use:   "line-reports <invoice-id> <line-id>",
		Short: "List the usage reports a metered invoice line was rated from",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			params := sdk.ListInvoiceLineReportsParams{}
			if flagChanged(cmd, "after-seq") {
				params.AfterSeq = &afterSeq
			}
			params.Limit = optionalInt32(limit)
			page, err := client.Billing.Invoices.ListLineReports(ctx, args[0], args[1], &params)
			if err != nil {
				return err
			}
			return writeStructured(cmd, output.FormatYAML, page, output.Table{})
		},
	}
	cmd.Flags().Int64Var(&afterSeq, "after-seq", 0, "Only reports after this sequence number: the previous page's last")
	cmd.Flags().Int32Var(&limit, "limit", 0, "At most this many reports")
	return cmd
}

// --- Subscriptions ---------------------------------------------------------

func newBillingSubscriptionsCommand() *cobra.Command {
	return groupCommand("subscriptions", "Manage instances' subscriptions",
		billingCall{
			use: "get <instance-slug>", short: "Show an instance's billing and subscription",
			run: func(ctx context.Context, c *sdk.Client, args []string) (any, error) {
				return c.Billing.Subscriptions.Get(ctx, args[0])
			},
		}.command(),
		billingBodyCall[sdk.NewSubscription]{
			use: "subscribe <instance-slug>", short: "Subscribe an instance to a price of its licence",
			run: func(ctx context.Context, c *sdk.Client, args []string, in sdk.NewSubscription) (any, error) {
				return c.Billing.Subscriptions.Subscribe(ctx, args[0], in)
			},
		}.command(),
		billingBodyCall[sdk.SubscriptionTerms]{
			use: "update <instance-slug>", short: "Change an instance's subscription terms",
			run: func(ctx context.Context, c *sdk.Client, args []string, in sdk.SubscriptionTerms) (any, error) {
				return c.Billing.Subscriptions.Update(ctx, args[0], in)
			},
		}.command(),
		billingBodyCall[sdk.SubscriptionCancellation]{
			use: "cancel <instance-slug>", short: "Cancel an instance's subscription, now or at the end of its period",
			optional: true, destructive: "cancel the subscription of",
			run: func(ctx context.Context, c *sdk.Client, args []string, in sdk.SubscriptionCancellation) (any, error) {
				return c.Billing.Subscriptions.Cancel(ctx, args[0], in)
			},
		}.command(),
		billingCall{
			use: "reactivate <instance-slug>", short: "Withdraw a cancellation scheduled for the end of the period",
			run: func(ctx context.Context, c *sdk.Client, args []string) (any, error) {
				return c.Billing.Subscriptions.Reactivate(ctx, args[0])
			},
		}.command(),
		billingBodyCall[sdk.PlanChangeTarget]{
			use: "schedule-change <instance-slug>", short: "Schedule a change of price for the next period",
			run: func(ctx context.Context, c *sdk.Client, args []string, in sdk.PlanChangeTarget) (any, error) {
				return c.Billing.Subscriptions.SchedulePlanChange(ctx, args[0], in)
			},
		}.command(),
		billingCall{
			use: "cancel-change <instance-slug>", short: "Withdraw the scheduled change of price",
			run: func(ctx context.Context, c *sdk.Client, args []string) (any, error) {
				return c.Billing.Subscriptions.CancelPlanChange(ctx, args[0])
			},
		}.command(),
		billingCall{
			use: "upcoming-invoice <instance-slug>", short: "Show the instance's next invoice, measured so far",
			run: func(ctx context.Context, c *sdk.Client, args []string) (any, error) {
				return c.Billing.Subscriptions.UpcomingInvoice(ctx, args[0])
			},
		}.command(),
		billingCall{
			use: "invoices <instance-slug>", short: "List an instance's invoices",
			run: func(ctx context.Context, c *sdk.Client, args []string) (any, error) {
				return c.Billing.Subscriptions.ListInvoices(ctx, args[0], nil)
			},
		}.command(),
	)
}

// --- Licence prices --------------------------------------------------------

func newBillingPricesCommand() *cobra.Command {
	return groupCommand("prices", "Manage the prices of licence versions",
		billingCall{
			use: "list <license-slug>", short: "List a licence version's prices",
			run: func(ctx context.Context, c *sdk.Client, args []string) (any, error) {
				return c.Billing.LicensePrices.List(ctx, args[0], nil)
			},
		}.command(),
		billingCall{
			use: "get <license-slug> <price-id>", short: "Show a price",
			run: func(ctx context.Context, c *sdk.Client, args []string) (any, error) {
				return c.Billing.LicensePrices.Get(ctx, args[0], args[1])
			},
		}.command(),
		billingBodyCall[sdk.NewLicensePrice]{
			use: "create <license-slug>", short: "Create a price on a licence version",
			run: func(ctx context.Context, c *sdk.Client, args []string, in sdk.NewLicensePrice) (any, error) {
				return c.Billing.LicensePrices.Create(ctx, args[0], in)
			},
		}.command(),
		billingBodyCall[sdk.LicensePriceChanges]{
			use: "update <license-slug> <price-id>", short: "Change a price",
			run: func(ctx context.Context, c *sdk.Client, args []string, in sdk.LicensePriceChanges) (any, error) {
				return c.Billing.LicensePrices.Update(ctx, args[0], args[1], in)
			},
		}.command(),
		billingCall{
			use: "deprecate <license-slug> <price-id>", short: "Deprecate a price: subscriptions keep it, new ones cannot take it",
			run: func(ctx context.Context, c *sdk.Client, args []string) (any, error) {
				return c.Billing.LicensePrices.Deprecate(ctx, args[0], args[1])
			},
		}.command(),
		billingBodyCall[sdk.InvoicePreviewScenario]{
			use: "preview <license-slug>", short: "Rate a scenario against a licence version's prices, billing nothing",
			run: func(ctx context.Context, c *sdk.Client, args []string, in sdk.InvoicePreviewScenario) (any, error) {
				return c.Billing.LicensePrices.Preview(ctx, args[0], in)
			},
		}.command(),
	)
}

// --- Add-ons ---------------------------------------------------------------

func newBillingAddonsCommand() *cobra.Command {
	return groupCommand("addons", "Manage the add-on catalogue",
		billingCall{
			use: "list", short: "List the add-ons",
			run: func(ctx context.Context, c *sdk.Client, _ []string) (any, error) {
				return c.Billing.Addons.List(ctx, nil)
			},
		}.command(),
		billingCall{
			use: "get <addon-slug>", short: "Show an add-on",
			run: func(ctx context.Context, c *sdk.Client, args []string) (any, error) {
				return c.Billing.Addons.Get(ctx, args[0])
			},
		}.command(),
		billingBodyCall[sdk.NewAddon]{
			use: "create", short: "Create an add-on",
			run: func(ctx context.Context, c *sdk.Client, _ []string, in sdk.NewAddon) (any, error) {
				return c.Billing.Addons.Create(ctx, in)
			},
		}.command(),
		billingBodyCall[sdk.AddonChanges]{
			use: "update <addon-slug>", short: "Change an add-on",
			run: func(ctx context.Context, c *sdk.Client, args []string, in sdk.AddonChanges) (any, error) {
				return c.Billing.Addons.Update(ctx, args[0], in)
			},
		}.command(),
		billingCall{
			use: "delete <addon-slug>", short: "Delete a draft add-on", message: "deleted", destructive: "delete add-on",
			run: func(ctx context.Context, c *sdk.Client, args []string) (any, error) {
				return nil, c.Billing.Addons.Delete(ctx, args[0])
			},
		}.command(),
		billingCall{
			use: "publish <addon-slug>", short: "Publish a draft add-on",
			run: func(ctx context.Context, c *sdk.Client, args []string) (any, error) {
				return c.Billing.Addons.Publish(ctx, args[0])
			},
		}.command(),
		billingCall{
			use: "archive <addon-slug>", short: "Archive an add-on: attachments keep it, new ones cannot take it",
			run: func(ctx context.Context, c *sdk.Client, args []string) (any, error) {
				return c.Billing.Addons.Archive(ctx, args[0])
			},
		}.command(),
		billingCall{
			use: "unarchive <addon-slug>", short: "Bring an archived add-on back",
			run: func(ctx context.Context, c *sdk.Client, args []string) (any, error) {
				return c.Billing.Addons.Unarchive(ctx, args[0])
			},
		}.command(),
		groupCommand("compatibility", "Choose the licence families an add-on attaches under",
			billingCall{
				use: "list <addon-slug>", short: "List the licence families an add-on attaches under",
				run: func(ctx context.Context, c *sdk.Client, args []string) (any, error) {
					return c.Billing.Addons.ListCompatibility(ctx, args[0])
				},
			}.command(),
			billingCall{
				use: "add <addon-slug> <family-slug>", short: "Make an add-on attachable under a licence family", message: "added",
				run: func(ctx context.Context, c *sdk.Client, args []string) (any, error) {
					return nil, c.Billing.Addons.SetCompatibility(ctx, args[0], args[1])
				},
			}.command(),
			billingCall{
				use: "remove <addon-slug> <family-slug>", short: "Stop an add-on attaching under a licence family", message: "removed",
				destructive: "remove the compatibility of",
				run: func(ctx context.Context, c *sdk.Client, args []string) (any, error) {
					return nil, c.Billing.Addons.RemoveCompatibility(ctx, args[0], args[1])
				},
			}.command(),
		),
		groupCommand("entitlements", "Manage what an add-on grants",
			billingCall{
				use: "list <addon-slug>", short: "List what an add-on grants",
				run: func(ctx context.Context, c *sdk.Client, args []string) (any, error) {
					return c.Billing.Addons.ListEntitlements(ctx, args[0])
				},
			}.command(),
			billingCall{
				use: "get <addon-slug> <entitlement-slug>", short: "Show an add-on's grant",
				run: func(ctx context.Context, c *sdk.Client, args []string) (any, error) {
					return c.Billing.Addons.GetEntitlement(ctx, args[0], args[1])
				},
			}.command(),
			billingBodyCall[sdk.NewAddonGrant]{
				use: "assign <addon-slug>", short: "Make an add-on grant an entitlement",
				run: func(ctx context.Context, c *sdk.Client, args []string, in sdk.NewAddonGrant) (any, error) {
					return c.Billing.Addons.AssignEntitlement(ctx, args[0], in)
				},
			}.command(),
			billingBodyCall[sdk.AddonGrant]{
				use: "update <addon-slug> <entitlement-slug>", short: "Change an add-on's grant",
				run: func(ctx context.Context, c *sdk.Client, args []string, in sdk.AddonGrant) (any, error) {
					return c.Billing.Addons.UpdateEntitlement(ctx, args[0], args[1], in)
				},
			}.command(),
			billingCall{
				use: "unassign <addon-slug> <entitlement-slug>", short: "Remove an add-on's grant", message: "unassigned",
				destructive: "remove the grant of",
				run: func(ctx context.Context, c *sdk.Client, args []string) (any, error) {
					return nil, c.Billing.Addons.UnassignEntitlement(ctx, args[0], args[1])
				},
			}.command(),
		),
		groupCommand("prices", "Manage an add-on's prices",
			billingCall{
				use: "list <addon-slug>", short: "List an add-on's prices",
				run: func(ctx context.Context, c *sdk.Client, args []string) (any, error) {
					return c.Billing.Addons.ListPrices(ctx, args[0], nil)
				},
			}.command(),
			billingBodyCall[sdk.NewAddonPrice]{
				use: "create <addon-slug>", short: "Create a price on an add-on",
				run: func(ctx context.Context, c *sdk.Client, args []string, in sdk.NewAddonPrice) (any, error) {
					return c.Billing.Addons.CreatePrice(ctx, args[0], in)
				},
			}.command(),
			billingCall{
				use: "deprecate <addon-slug> <price-id>", short: "Deprecate a price of an add-on",
				run: func(ctx context.Context, c *sdk.Client, args []string) (any, error) {
					return c.Billing.Addons.DeprecatePrice(ctx, args[0], args[1])
				},
			}.command(),
		),
		groupCommand("families", "Show and change add-on families",
			billingCall{
				use: "list", short: "List the add-on families",
				run: func(ctx context.Context, c *sdk.Client, _ []string) (any, error) {
					return c.Billing.Addons.ListFamilies(ctx)
				},
			}.command(),
			billingCall{
				use: "get <family-slug>", short: "Show an add-on family",
				run: func(ctx context.Context, c *sdk.Client, args []string) (any, error) {
					return c.Billing.Addons.GetFamily(ctx, args[0])
				},
			}.command(),
			billingBodyCall[sdk.AddonFamilyVisibility]{
				use: "update <family-slug>", short: "Change an add-on family",
				run: func(ctx context.Context, c *sdk.Client, args []string, in sdk.AddonFamilyVisibility) (any, error) {
					return c.Billing.Addons.UpdateFamily(ctx, args[0], in)
				},
			}.command(),
		),
	)
}

func newBillingInstanceAddonsCommand() *cobra.Command {
	return groupCommand("instance-addons", "Attach add-ons to instances",
		billingCall{
			use: "list <instance-slug>", short: "List the add-ons attached to an instance",
			run: func(ctx context.Context, c *sdk.Client, args []string) (any, error) {
				return c.Billing.InstanceAddons.List(ctx, args[0], nil)
			},
		}.command(),
		billingBodyCall[sdk.NewInstanceAddon]{
			use: "attach <instance-slug>", short: "Attach an add-on to an instance",
			run: func(ctx context.Context, c *sdk.Client, args []string, in sdk.NewInstanceAddon) (any, error) {
				return c.Billing.InstanceAddons.Attach(ctx, args[0], in)
			},
		}.command(),
		newBillingSetQuantityCommand(),
		billingCall{
			use: "detach <instance-slug> <addon-slug>", short: "Remove an add-on from an instance", message: "detached",
			destructive: "detach",
			run: func(ctx context.Context, c *sdk.Client, args []string) (any, error) {
				return nil, c.Billing.InstanceAddons.Detach(ctx, args[0], args[1])
			},
		}.command(),
	)
}

func newBillingSetQuantityCommand() *cobra.Command {
	var quantity int32
	cmd := &cobra.Command{
		Use:   "set-quantity <instance-slug> <addon-slug>",
		Short: "Change the quantity of an attached add-on",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if quantity < 1 {
				return errUsagef("--quantity must be at least 1; detach the add-on to remove it")
			}
			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			attachment, err := client.Billing.InstanceAddons.SetQuantity(ctx, args[0], args[1], sdk.AddonQuantity{Quantity: quantity})
			if err != nil {
				return err
			}
			return writeStructured(cmd, output.FormatYAML, attachment, output.Table{})
		},
	}
	cmd.Flags().Int32Var(&quantity, "quantity", 0, "The new quantity (required)")
	return cmd
}

// --- Vouchers --------------------------------------------------------------

func newBillingVouchersCommand() *cobra.Command {
	return groupCommand("vouchers", "Manage vouchers and their redemptions",
		billingCall{
			use: "list", short: "List the vouchers",
			run: func(ctx context.Context, c *sdk.Client, _ []string) (any, error) {
				return c.Billing.Vouchers.List(ctx, nil)
			},
		}.command(),
		billingCall{
			use: "get <voucher-id>", short: "Show a voucher",
			run: func(ctx context.Context, c *sdk.Client, args []string) (any, error) {
				return c.Billing.Vouchers.Get(ctx, args[0])
			},
		}.command(),
		billingBodyCall[sdk.VoucherDraft]{
			use: "create", short: "Create a draft voucher",
			run: func(ctx context.Context, c *sdk.Client, _ []string, in sdk.VoucherDraft) (any, error) {
				return c.Billing.Vouchers.Create(ctx, in)
			},
		}.command(),
		billingBodyCall[sdk.VoucherDraft]{
			use: "update <voucher-id>", short: "Change a voucher",
			run: func(ctx context.Context, c *sdk.Client, args []string, in sdk.VoucherDraft) (any, error) {
				return c.Billing.Vouchers.Update(ctx, args[0], in)
			},
		}.command(),
		billingCall{
			use: "publish <voucher-id>", short: "Publish a draft voucher: its code can be redeemed from now on",
			run: func(ctx context.Context, c *sdk.Client, args []string) (any, error) {
				return c.Billing.Vouchers.Publish(ctx, args[0])
			},
		}.command(),
		billingCall{
			use: "archive <voucher-id>", short: "Archive a voucher: its code is redeemed no more",
			run: func(ctx context.Context, c *sdk.Client, args []string) (any, error) {
				return c.Billing.Vouchers.Archive(ctx, args[0])
			},
		}.command(),
		billingCall{
			use: "redemptions <voucher-id>", short: "List the redemptions of a voucher",
			run: func(ctx context.Context, c *sdk.Client, args []string) (any, error) {
				return c.Billing.Vouchers.ListRedemptions(ctx, args[0], nil)
			},
		}.command(),
		billingCall{
			use: "lookup <code>", short: "Find a voucher by its code",
			run: func(ctx context.Context, c *sdk.Client, args []string) (any, error) {
				return c.Billing.Vouchers.Lookup(ctx, sdk.VoucherCode{Code: args[0]})
			},
		}.command(),
		billingBodyCall[sdk.VoucherCheck]{
			use: "validate", short: "Check whether a code could be redeemed on an instance, redeeming nothing",
			run: func(ctx context.Context, c *sdk.Client, _ []string, in sdk.VoucherCheck) (any, error) {
				return c.Billing.Vouchers.Validate(ctx, in)
			},
		}.command(),
		billingCall{
			use: "instance-list <instance-slug>", short: "List the vouchers redeemed on an instance",
			run: func(ctx context.Context, c *sdk.Client, args []string) (any, error) {
				return c.Billing.InstanceVouchers.List(ctx, args[0], nil)
			},
		}.command(),
		billingCall{
			use: "redeem <instance-slug> <code>", short: "Redeem a voucher on an instance",
			run: func(ctx context.Context, c *sdk.Client, args []string) (any, error) {
				return c.Billing.InstanceVouchers.Redeem(ctx, args[0], sdk.RedeemableCode{Code: args[1]})
			},
		}.command(),
		billingBodyCall[sdk.RevocationReason]{
			use: "revoke <instance-slug> <redemption-id>", short: "Revoke a redemption",
			optional: true, destructive: "revoke redemption",
			run: func(ctx context.Context, c *sdk.Client, args []string, in sdk.RevocationReason) (any, error) {
				return c.Billing.InstanceVouchers.Revoke(ctx, args[0], args[1], in)
			},
		}.command(),
	)
}

// --- Keys, sessions, customers ---------------------------------------------

func newBillingKeysCommand() *cobra.Command {
	return groupCommand("publishable-keys", "Manage the pk_ keys a public page reads the catalogue with",
		billingCall{
			use: "list", short: "List the publishable keys",
			run: func(ctx context.Context, c *sdk.Client, _ []string) (any, error) {
				return c.Billing.PublishableKeys.List(ctx, nil)
			},
		}.command(),
		billingBodyCall[sdk.PublishableKeyDraft]{
			use: "create", short: "Create a publishable key; its secret is shown this once",
			run: func(ctx context.Context, c *sdk.Client, _ []string, in sdk.PublishableKeyDraft) (any, error) {
				return c.Billing.PublishableKeys.Create(ctx, in)
			},
		}.command(),
		billingBodyCall[sdk.PublishableKeyPatch]{
			use: "update <key-id>", short: "Change a publishable key",
			run: func(ctx context.Context, c *sdk.Client, args []string, in sdk.PublishableKeyPatch) (any, error) {
				return c.Billing.PublishableKeys.Update(ctx, args[0], in)
			},
		}.command(),
		billingCall{
			use: "revoke <key-id>", short: "Revoke a publishable key", destructive: "revoke publishable key",
			run: func(ctx context.Context, c *sdk.Client, args []string) (any, error) {
				return c.Billing.PublishableKeys.Revoke(ctx, args[0])
			},
		}.command(),
	)
}

func newBillingSessionsCommand() *cobra.Command {
	return groupCommand("customer-sessions", "Manage the kst_ sessions a customer's page acts with",
		billingBodyCall[sdk.CustomerSessionDraft]{
			use: "create", short: "Create a customer session; its secret is shown this once",
			run: func(ctx context.Context, c *sdk.Client, _ []string, in sdk.CustomerSessionDraft) (any, error) {
				return c.Billing.CustomerSessions.Create(ctx, in)
			},
		}.command(),
		billingCall{
			use: "revoke <session-id>", short: "Revoke a customer session", message: "revoked", destructive: "revoke customer session",
			run: func(ctx context.Context, c *sdk.Client, args []string) (any, error) {
				return nil, c.Billing.CustomerSessions.Revoke(ctx, args[0])
			},
		}.command(),
	)
}

func newBillingCustomersCommand() *cobra.Command {
	return groupCommand("customers", "Show a customer's billing, payment method and portal",
		billingCall{
			use: "get <customer-slug>", short: "Show a customer's billing",
			run: func(ctx context.Context, c *sdk.Client, args []string) (any, error) {
				return c.Billing.Customers.Get(ctx, args[0])
			},
		}.command(),
		billingCall{
			use: "detach-payment-method <customer-slug>", short: "Remove the customer's default payment method", message: "detached",
			destructive: "detach the payment method of",
			run: func(ctx context.Context, c *sdk.Client, args []string) (any, error) {
				return nil, c.Billing.Customers.DetachPaymentMethod(ctx, args[0])
			},
		}.command(),
		billingBodyCall[sdk.NewPaymentMethodSession]{
			use: "payment-method-session <customer-slug>", short: "Start a hosted session where the customer sets a payment method",
			run: func(ctx context.Context, c *sdk.Client, args []string, in sdk.NewPaymentMethodSession) (any, error) {
				return c.Billing.Customers.CreatePaymentMethodSession(ctx, args[0], in)
			},
		}.command(),
		billingCall{
			use: "complete-payment-method-session <customer-slug> <session-id>", short: "Read back the payment method a session set",
			run: func(ctx context.Context, c *sdk.Client, args []string) (any, error) {
				return c.Billing.Customers.CompletePaymentMethodSession(ctx, args[0], args[1])
			},
		}.command(),
		billingBodyCall[sdk.NewPortalSession]{
			use: "portal-session <customer-slug>", short: "Open the billing provider's portal for the customer",
			run: func(ctx context.Context, c *sdk.Client, args []string, in sdk.NewPortalSession) (any, error) {
				return c.Billing.Customers.CreatePortalSession(ctx, args[0], in)
			},
		}.command(),
	)
}
