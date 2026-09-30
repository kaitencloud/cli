package cmd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"

	"github.com/kaitencloud/sdk-go"
	"github.com/spf13/cobra"
)

// Exit codes the kaiten CLI returns to its caller. They are part of the CLI's contract:
// a script can branch on the kind of failure without parsing the error text. Keep the set
// small — a code nobody checks is worse than none — and document any addition in README.md.
const (
	// exitOK reports that the command succeeded.
	exitOK = 0
	// exitError reports a failure that does not fit any other code.
	exitError = 1
	// exitUsage reports that the command was invoked wrongly: an unknown command or flag,
	// the wrong number of arguments, a nonsensical flag combination, or a configuration
	// the CLI cannot use.
	exitUsage = 2
	// exitAuth reports that the API refused the credentials (401) or the caller was not
	// allowed to perform the operation (403).
	exitAuth = 3
	// exitNotFound reports that the addressed resource does not exist (404).
	exitNotFound = 4
	// exitRejected reports that the API understood the request and refused it: a validation
	// failure, a conflict, a rate limit — any other 4xx. Retrying it unchanged will not help.
	exitRejected = 5
	// exitUnavailable reports that the operation did not happen for reasons outside the
	// caller's control: the API could not be reached, the request timed out, or the API
	// answered 5xx. Retrying may help.
	exitUnavailable = 6
	// exitInterrupted reports that the run was cancelled by a signal, following the shell
	// convention of 128 plus the signal number.
	exitInterrupted = 130
)

// usageError marks a failure caused by how the command was invoked rather than by the work
// it tried to do. Mistakes cobra catches before a command body runs — unknown commands,
// unknown flags, wrong argument counts, missing required flags — are recognised separately
// by exitCode and do not need this wrapper.
type usageError struct {
	err error
}

// errUsage marks err as a usage error.
func errUsage(err error) error {
	return &usageError{err: err}
}

// errUsagef returns a usage error with the given formatted message.
func errUsagef(format string, args ...any) error {
	return errUsage(fmt.Errorf(format, args...))
}

func (e *usageError) Error() string { return e.err.Error() }

func (e *usageError) Unwrap() error { return e.err }

// exitCode maps a finished run onto the documented exit codes. cmd is the command cobra
// actually reached, which is what separates a usage error from a runtime one: the root
// command's persistent pre-run sets SilenceUsage once flag and argument validation has
// passed, so a command that failed while SilenceUsage is still false never started work.
func exitCode(cmd *cobra.Command, err error) int {
	if err == nil {
		return exitOK
	}

	// A cancelled context is the signal handler in Execute, not an API outcome, so it is
	// checked before the transport errors it also produces.
	if errors.Is(err, context.Canceled) {
		return exitInterrupted
	}

	var usageErr *usageError
	if errors.As(err, &usageErr) || (cmd != nil && !cmd.SilenceUsage) {
		return exitUsage
	}

	var apiErr *sdk.Error
	if errors.As(err, &apiErr) {
		switch {
		case apiErr.StatusCode == http.StatusUnauthorized, apiErr.StatusCode == http.StatusForbidden:
			return exitAuth
		case apiErr.StatusCode == http.StatusNotFound:
			return exitNotFound
		case apiErr.StatusCode >= http.StatusInternalServerError:
			return exitUnavailable
		default:
			return exitRejected
		}
	}

	// ReportUsage drops the *sdk.Error it derives this sentinel from, so it has to be
	// matched on its own rather than through the 409 it stands for.
	if errors.Is(err, sdk.ErrThresholdExceeded) {
		return exitRejected
	}

	var netErr net.Error
	if errors.As(err, &netErr) || errors.Is(err, context.DeadlineExceeded) {
		return exitUnavailable
	}

	return exitError
}
