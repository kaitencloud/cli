package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

const skipOutputValidationAnnotation = "kaiten.cloud/skip-output-validation"

var (
	// Version is the kaiten CLI version, set at build time.
	Version = "dev"
	// Commit is the git commit the kaiten CLI was built from, set at build time.
	Commit = "HEAD"
	// Date is the build date of the kaiten CLI, set at build time.
	Date = "unknown"
)

// Execute runs the kaiten CLI with the given version, commit, and date, exiting the process
// with one of the documented exit codes.
func Execute(version, commit, date string) {
	os.Exit(run(version, commit, date))
}

// run executes the root command and returns the process exit code. It is separate from
// Execute so that the deferred signal cleanup runs before os.Exit does.
func run(version, commit, date string) int {
	Version = version
	Commit = commit
	Date = date

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cmd, err := NewRootCommand().ExecuteContextC(ctx)
	if err == nil {
		return exitOK
	}

	fmt.Fprintln(os.Stderr, err)
	return exitCode(cmd, err)
}

// NewRootCommand builds the root kaiten cobra command with all subcommands registered.
//
// The root command deliberately leaves Args nil so that cobra reports an unknown top-level
// command with its own "Did you mean this?" suggestions.
func NewRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "kaiten",
		Short: "Kaiten command-line interface",
		Long: `Kaiten command-line interface.

The base URL, token and output format are resolved from the flag first, then
the KAITEN_BASE_URL, KAITEN_AUTH_TOKEN and KAITEN_OUTPUT environment
variables, then the config file. Run "kaiten doctor" to see what a given
invocation resolves to, and "kaiten config view" to see what is persisted.

Every command reports the kind of failure through its exit code: 2 for a
wrong invocation, 3 for refused credentials, 4 for a missing resource, 5 for
a request the API refused, 6 for one that never completed, 130 for a run
interrupted by a signal. README.md carries the full table.`,
		Example: `  # Point one invocation at a deployment without persisting anything
  kaiten --base-url https://kaiten.example.com/api --auth-token "$TOKEN" instances list

  # Persist the same settings, then check them
  kaiten config set base-url https://kaiten.example.com/api
  kaiten doctor

  # Machine-readable output for a script
  kaiten releases list --output json

  # Destructive commands refuse a non-interactive stdin unless --yes is given
  kaiten customers delete acme --yes`,
		SilenceErrors:     true,
		RunE:              runHelp,
		PersistentPreRunE: silenceUsageAfterValidation,
	}

	rootCmd.PersistentFlags().String("base-url", "", "Kaiten API base URL")
	rootCmd.PersistentFlags().String("auth-token", "", "Kaiten API bearer token")
	rootCmd.PersistentFlags().String("output", "", "Output format: table, json, yaml")

	rootCmd.AddCommand(newVersionCommand())
	rootCmd.AddCommand(newConfigCommand())
	rootCmd.AddCommand(newDoctorCommand())
	rootCmd.AddCommand(newComponentsCommand())
	rootCmd.AddCommand(newCustomersCommand())
	rootCmd.AddCommand(newDeploymentZonesCommand())
	rootCmd.AddCommand(newEntitlementGroupsCommand())
	rootCmd.AddCommand(newEntitlementsCommand())
	rootCmd.AddCommand(newFeatureFlagsCommand())
	rootCmd.AddCommand(newInstancesCommand())
	rootCmd.AddCommand(newLicensesCommand())
	rootCmd.AddCommand(newReleasesCommand())
	rootCmd.AddCommand(newServiceAccountsCommand())

	return rootCmd
}
