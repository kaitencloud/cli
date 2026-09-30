package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// confirmFlagName is the flag that skips the confirmation prompt of a destructive command.
const confirmFlagName = "yes"

// isTerminal reports whether reader is an interactive terminal. It is a variable so tests
// can exercise the interactive prompt without allocating a pseudo-terminal.
var isTerminal = func(reader io.Reader) bool {
	file, ok := reader.(*os.File)
	if !ok {
		return false
	}

	info, err := file.Stat()
	if err != nil {
		return false
	}

	return info.Mode()&os.ModeCharDevice != 0
}

// addConfirmFlag registers --yes on a destructive command.
func addConfirmFlag(cmd *cobra.Command) {
	cmd.Flags().BoolP(confirmFlagName, "y", false, "Skip the confirmation prompt")
}

// confirmDestructive echoes the target of an irreversible operation and waits for the
// operator to confirm it. It returns an error when the operator declines, and when stdin
// is not a terminal and --yes was not given, so that non-interactive callers fail loudly
// instead of hanging on a prompt nobody can answer.
func confirmDestructive(cmd *cobra.Command, action, target string) error {
	skip, err := cmd.Flags().GetBool(confirmFlagName)
	if err != nil {
		return err
	}
	if skip {
		return nil
	}

	stdin := cmd.InOrStdin()
	if !isTerminal(stdin) {
		return errNotInteractive(action, target)
	}

	prompt := fmt.Sprintf("About to %s %s. This cannot be undone.\nContinue? [y/N]: ", action, target)
	if _, err := io.WriteString(cmd.ErrOrStderr(), prompt); err != nil {
		return err
	}

	answer, err := bufio.NewReader(stdin).ReadString('\n')
	switch {
	case errors.Is(err, io.EOF) && strings.TrimSpace(answer) == "":
		// Stdin looked like a terminal but nothing can be read from it, which is what
		// /dev/null and a closed stdin look like on a build agent.
		return errNotInteractive(action, target)
	case err != nil && !errors.Is(err, io.EOF):
		return fmt.Errorf("read confirmation: %w", err)
	}

	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "y", "yes":
		return nil
	default:
		return fmt.Errorf("%s %s cancelled", action, target)
	}
}

func errNotInteractive(action, target string) error {
	return errUsagef("refusing to %s %s without confirmation: stdin is not interactive, pass --%s", action, target, confirmFlagName)
}
