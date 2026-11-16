package cmd

import (
	"github.com/kaitencloud/cli/internal/output"
	"github.com/kaitencloud/sdk-go"
	"github.com/spf13/cobra"
)

func newDashboardCommand() *cobra.Command {
	var period string

	cmd := &cobra.Command{
		Use:   "dashboard",
		Short: "Show the organization's summary: what needs attention and the period's figures",
		Long: `Show the console's home as one document, computed by Kaiten over the whole
organization: what needs attention, the period's figures, the meters closest to
their limits, usage reports per day and per instance, and what runs where.

It needs Kaiten 26.11.1 or later, and a credential holding read:customers,
read:instances and read:licenses. Blocks about a module the credential cannot
read are left out.`,
		Args: cobra.NoArgs,
		Example: `  kaiten dashboard
  kaiten dashboard --range 7d --output json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var params *sdk.DashboardSummaryParams
			if period := optionalEnum[sdk.DashboardRange](cmd, "range", period); period != nil {
				params = &sdk.DashboardSummaryParams{Range: period}
			}

			client, _, err := newClient(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd)
			defer cancel()
			summary, err := client.Dashboard.Summary(ctx, params)
			if err != nil {
				return err
			}
			return writeStructured(cmd, output.FormatYAML, summary, output.Table{})
		},
	}
	cmd.Flags().StringVar(&period, "range", "", "The period the rates and lists cover: 7d, 30d or 90d (default: Kaiten's, 30d)")
	return cmd
}
