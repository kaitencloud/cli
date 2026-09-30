package cmd

import (
	"fmt"

	"github.com/kaitencloud/cli/internal/config"
	"github.com/kaitencloud/cli/internal/output"
	"github.com/spf13/cobra"
)

func newDoctorCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Validate CLI configuration and API connectivity",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, cfg, err := newClient(cmd)
			if err != nil {
				return err
			}

			ctx, cancel := commandContext(cmd)
			defer cancel()

			instances, err := client.Instances.List(ctx, nil)
			if err != nil {
				return fmt.Errorf("connectivity check failed: %w", err)
			}

			result := struct {
				BaseURL        string `json:"base_url" yaml:"base_url"`
				AuthToken      string `json:"auth_token,omitempty" yaml:"auth_token,omitempty"`
				InstancesCount int    `json:"instances_count" yaml:"instances_count"`
				Status         string `json:"status" yaml:"status"`
			}{
				BaseURL:        cfg.BaseURL,
				AuthToken:      config.MaskToken(cfg.AuthToken),
				InstancesCount: len(instances),
				Status:         "ok",
			}

			return writeStructured(cmd, output.FormatYAML, result, output.Table{
				Columns: []string{"STATUS", "BASE URL", "INSTANCES", "AUTH TOKEN"},
				Rows: [][]string{{
					result.Status,
					result.BaseURL,
					fmt.Sprintf("%d", result.InstancesCount),
					result.AuthToken,
				}},
			})
		},
	}
}
