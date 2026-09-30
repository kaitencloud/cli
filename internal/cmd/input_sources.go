package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/kaitencloud/cli/internal/input"
	"github.com/spf13/cobra"
)

func addInputSourceFlags(cmd *cobra.Command, file, payload *string, noun string) {
	cmd.Flags().StringVarP(file, "file", "f", "", fmt.Sprintf("Path to the %s JSON or YAML file", noun))
	cmd.Flags().StringVar(payload, "payload", "", fmt.Sprintf("Inline %s JSON or YAML payload", noun))
}

// resolveInput picks the single input source the invocation used and turns it into T.
// The three sources are mutually exclusive on purpose: a --file that silently loses to an
// inline flag, or the other way round, would send a payload the operator never wrote.
func resolveInput[T any](file, payload string, hasInline bool, buildInline func() (T, error)) (T, error) {
	var zero T

	sources := 0
	if strings.TrimSpace(file) != "" {
		sources++
	}
	if strings.TrimSpace(payload) != "" {
		sources++
	}
	if hasInline {
		sources++
	}

	switch {
	case sources == 0:
		return zero, errUsagef("provide one input source: --file, --payload, or inline flags")
	case sources > 1:
		return zero, errUsagef("use exactly one input source: --file, --payload, or inline flags")
	}

	if strings.TrimSpace(file) != "" {
		return input.LoadFile[T](file)
	}
	if strings.TrimSpace(payload) != "" {
		return input.LoadString[T](payload)
	}

	return buildInline()
}

// resolveFileOrPayload is resolveInput for a command whose payload has no inline flags.
func resolveFileOrPayload[T any](file, payload string) (T, error) {
	return resolveInput(file, payload, false, func() (T, error) {
		var zero T
		return zero, nil
	})
}

func flagChanged(cmd *cobra.Command, name string) bool {
	flag := cmd.Flags().Lookup(name)
	return flag != nil && flag.Changed
}

func anyFlagChanged(cmd *cobra.Command, names ...string) bool {
	for _, name := range names {
		if flagChanged(cmd, name) {
			return true
		}
	}
	return false
}

// optionalString returns a pointer to value only when the invocation set the named flag.
// An optional field the operator never mentioned has to stay absent from the request body:
// sending it as an empty string would overwrite whatever the API already holds.
func optionalString(cmd *cobra.Command, name, value string) *string {
	if !flagChanged(cmd, name) {
		return nil
	}
	return &value
}

// optionalEnum is optionalString for a string-backed SDK enum. The CLI does not validate
// the member here; the API rejects an unknown one with a message naming the valid set.
func optionalEnum[T ~string](cmd *cobra.Command, name, value string) *T {
	if !flagChanged(cmd, name) {
		return nil
	}
	member := T(value)
	return &member
}

// parseRequiredMapFlag is parseMapFlag for an object the API requires: an omitted flag
// becomes an empty object, because the field cannot be sent as null.
func parseRequiredMapFlag(raw, flagName string) (map[string]any, error) {
	value, err := parseMapFlag(raw, flagName)
	if err != nil {
		return nil, err
	}
	if value == nil {
		return map[string]any{}, nil
	}
	return value, nil
}

func parseMapFlag(raw, flagName string) (map[string]any, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}

	value, err := input.LoadString[map[string]any](raw)
	if err != nil {
		return nil, errUsagef("parse --%s: %w", flagName, err)
	}

	return value, nil
}

func parseTimeFlag(raw, flagName string) (*time.Time, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}

	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, errUsagef("parse --%s: %w", flagName, err)
	}

	return &parsed, nil
}

func requiredInlineString(value, flagName string) error {
	if strings.TrimSpace(value) == "" {
		return errUsagef("--%s is required when using inline flags", flagName)
	}
	return nil
}

// requiredInlineTime parses an RFC3339 flag that inline input cannot omit.
func requiredInlineTime(raw, flagName string) (time.Time, error) {
	parsed, err := parseTimeFlag(raw, flagName)
	if err != nil {
		return time.Time{}, err
	}
	if parsed == nil {
		return time.Time{}, errUsagef("--%s is required when using inline flags", flagName)
	}
	return *parsed, nil
}

// requiredInlineFlag checks a flag that inline input cannot omit and whose zero value is a
// legitimate answer -- false, or 0. Those cannot be told apart from "absent" by value, so
// whether the flag was set is the only usable signal.
func requiredInlineFlag(cmd *cobra.Command, flagName string) error {
	if !flagChanged(cmd, flagName) {
		return errUsagef("--%s is required when using inline flags", flagName)
	}
	return nil
}
