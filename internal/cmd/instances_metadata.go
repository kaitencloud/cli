package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

func newInstancesMetadataCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "metadata",
		Short: "Set or remove individual metadata keys of an instance",
		Long: "Set or remove individual metadata keys of an instance, without sending the whole\n" +
			"instance. Kaiten merges the keys into the stored metadata: each key sent is set, its\n" +
			"value replacing the stored one whole (lists and objects included), a removed key is\n" +
			"deleted, and every key the command does not name is kept. The merged metadata is\n" +
			"validated against the instance metadata fields the organization declares; a refusal\n" +
			"exits 5 and changes nothing.\n\n" +
			"These commands need a Kaiten release that accepts metadata on PATCH\n" +
			"/instances/{slug}; an older one refuses the request (exit code 5).",
		Args: cobra.NoArgs,
		RunE: runHelp,
	}
	cmd.AddCommand(newInstancesMetadataSetCommand())
	cmd.AddCommand(newInstancesMetadataUnsetCommand())
	return cmd
}

func newInstancesMetadataSetCommand() *cobra.Command {
	var file, payload string
	var stringPairs, jsonPairs []string
	cmd := &cobra.Command{
		Use:   "set <instance-slug>",
		Short: "Set metadata keys of an instance, keeping the others",
		Long: "Set metadata keys of an instance, keeping the others.\n\n" +
			"--string KEY=VALUE sets KEY to the text VALUE. --json KEY=JSON sets KEY to a JSON\n" +
			"value: a number, a boolean, a list or an object, or null to remove the key. Both\n" +
			"repeat, and the keys of one invocation go out in a single request.\n\n" +
			"--file and --payload take the metadata object to merge, in JSON or YAML, instead of\n" +
			"the flags: a key whose value is null is removed. They are sent as written.",
		Example: `  # Set one key
  kaiten instances metadata set acme-prod --string region=eu-west

  # Set several, of any JSON type, in one request
  kaiten instances metadata set acme-prod --string tier=gold \
    --json seats=40 --json 'tags=["beta","emea"]' --json 'sso={"provider":"okta"}'

  # Set and remove keys from a file; null removes
  kaiten instances metadata set acme-prod --file metadata.yaml`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			hasInline := anyFlagChanged(cmd, "string", "json")
			patch, err := resolveInput(file, payload, hasInline, func() (map[string]any, error) {
				return metadataPatchFromFlags(stringPairs, jsonPairs)
			})
			if err != nil {
				return err
			}
			if len(patch) == 0 {
				return errUsagef("name at least one metadata key to set")
			}
			return patchInstanceMetadata(cmd, args[0], patch)
		},
	}
	addInputSourceFlags(cmd, &file, &payload, "metadata")
	cmd.Flags().StringArrayVar(&stringPairs, "string", nil, "Set KEY to the text VALUE, as KEY=VALUE; repeat for more keys")
	cmd.Flags().StringArrayVar(&jsonPairs, "json", nil, "Set KEY to a JSON value, as KEY=JSON; null removes it; repeat for more keys")
	return cmd
}

func newInstancesMetadataUnsetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "unset <instance-slug> <key>...",
		Short: "Remove metadata keys of an instance, keeping the others",
		Long: "Remove metadata keys of an instance, keeping the others. A key the instance does\n" +
			"not hold is not an error: there is nothing to remove.",
		Example: `  kaiten instances metadata unset acme-prod legacy-plan
  kaiten instances metadata unset acme-prod legacy-plan trial-notes`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			patch := make(map[string]any, len(args)-1)
			for _, key := range args[1:] {
				if strings.TrimSpace(key) == "" {
					return errUsagef("a metadata key cannot be empty")
				}
				patch[key] = nil
			}
			return patchInstanceMetadata(cmd, args[0], patch)
		},
	}
}

func patchInstanceMetadata(cmd *cobra.Command, instanceSlug string, patch map[string]any) error {
	client, _, err := newClient(cmd)
	if err != nil {
		return err
	}
	ctx, cancel := commandContext(cmd)
	defer cancel()
	if err := client.Instances.PatchMetadata(ctx, instanceSlug, patch); err != nil {
		return err
	}
	return writeMessage(cmd, "Instance metadata updated")
}

// metadataPatchFromFlags builds the metadata patch of --string and --json. A key named twice
// is refused rather than resolved: either answer would drop a value the operator wrote.
func metadataPatchFromFlags(stringPairs, jsonPairs []string) (map[string]any, error) {
	patch := make(map[string]any, len(stringPairs)+len(jsonPairs))
	add := func(flag, pair string, parse func(string) (any, error)) error {
		key, raw, found := strings.Cut(pair, "=")
		if !found || strings.TrimSpace(key) == "" {
			return errUsagef("--%s takes KEY=VALUE, got %q", flag, pair)
		}
		if _, seen := patch[key]; seen {
			return errUsagef("metadata key %q is set more than once", key)
		}
		value, err := parse(raw)
		if err != nil {
			return errUsagef("parse --%s %s: %w", flag, key, err)
		}
		patch[key] = value
		return nil
	}

	for _, pair := range stringPairs {
		if err := add("string", pair, func(raw string) (any, error) { return raw, nil }); err != nil {
			return nil, err
		}
	}
	for _, pair := range jsonPairs {
		if err := add("json", pair, parseJSONValue); err != nil {
			return nil, err
		}
	}
	return patch, nil
}

// parseJSONValue reads exactly one JSON value. Numbers stay json.Number, so an integer
// too large for a float64 reaches the API as written.
func parseJSONValue(raw string) (any, error) {
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		if errors.Is(err, io.EOF) {
			err = errors.New("no JSON value")
		}
		return nil, fmt.Errorf("%w (text goes in --string, or quoted as a JSON string)", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, errors.New("more than one JSON value (text goes in --string, or quoted as a JSON string)")
	}
	return value, nil
}
