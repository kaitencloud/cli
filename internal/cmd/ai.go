package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"

	"github.com/kaitencloud/cli/internal/output"
	"github.com/kaitencloud/sdk-go"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// The pinned SDK predates the AI control-plane endpoints. Keep this bridge local
// until those operations are available in a released SDK. Reports record usage
// only: this command never holds provider keys or executes inference.
func newAICommand() *cobra.Command {
	models := aiCall{
		use: "list", short: "List one page of the model catalogue",
		path:     func([]string) string { return "/models" },
		filters:  map[string]string{"cursor": "cursor", "status": "status", "vendor": "vendor", "modality": "modality"},
		maxLimit: 200,
	}.command()
	modelPath := func(args []string) string {
		path := "/models"
		if len(args) > 0 {
			path += "/" + url.PathEscape(args[0])
		}
		return path
	}
	rulesPath := func(args []string) string {
		path := "/licenses/" + url.PathEscape(args[0]) + "/conversion-rules"
		if len(args) > 1 {
			path += "/" + url.PathEscape(args[1])
		}
		return path
	}
	inferences := aiCall{
		use: "list <instance-slug> <entitlement-slug>", short: "List one page of AI inference accounting records",
		path:     aiUsagePath("inferences"),
		filters:  map[string]string{"from": "from", "to": "to", "status": "status", "transaction-id": "transactionId", "cursor": "cursor"},
		maxLimit: 500,
	}.command()
	return groupCommand(
		"ai", "Manage AI credits and report or inspect accounting (no inference execution)",
		aiCall{
			use: "report <instance-slug> <entitlement-slug>", short: "Record an AI reserve or commit from an authored usage report",
			path: aiUsagePath(""), method: http.MethodPost, report: true,
		}.command(),
		groupCommand("models", "Manage the model catalogue (no provider credentials)", models,
			aiCall{use: "get <model-slug>", short: "Show a model", path: modelPath}.command(),
			aiCall{use: "create", short: "Create a catalogue model from JSON or YAML", path: modelPath, method: http.MethodPost, model: true}.command(),
			aiCall{use: "update <model-slug>", short: "Update a catalogue model without changing its slug", path: modelPath, method: http.MethodPut, model: true}.command(),
			aiCall{use: "delete <model-slug>", short: "Delete a catalogue model", path: modelPath, method: http.MethodDelete, model: true}.command()),
		groupCommand("conversion-rules", "Manage conversion rules of a specific license version (not a license family)",
			aiCall{
				use: "list <license-slug>", short: "List a license version's conversion rules", path: rulesPath,
				filters: map[string]string{"status": "status", "entitlement": "entitlementSlug", "model": "modelSlug"},
			}.command(),
			aiCall{use: "get <license-slug> <rule-id>", short: "Show a conversion rule", path: rulesPath}.command(),
			aiCall{use: "create <license-slug>", short: "Create a conversion rule from JSON or YAML", path: rulesPath, method: http.MethodPost}.command(),
			aiCall{use: "update <license-slug> <rule-id>", short: "Update a conversion rule from JSON or YAML", path: rulesPath, method: http.MethodPut}.command(),
			aiCall{use: "archive <license-slug> <rule-id>", short: "Archive a conversion rule", path: rulesPath, method: http.MethodDelete}.command()),
		groupCommand("inferences", "Read inference accounting records", inferences),
		groupCommand("reservations", "Read open AI credit reservations",
			aiCall{use: "list <instance-slug> <entitlement-slug>", short: "Show open reservations and reserved credits", path: aiUsagePath("reservations")}.command()),
	)
}

func aiUsagePath(resource string) func([]string) string {
	suffix := ""
	if resource != "" {
		suffix = "/" + resource
	}
	return func(args []string) string {
		return "/instances/" + url.PathEscape(args[0]) + "/entitlements/" + url.PathEscape(args[1]) + "/usage" + suffix
	}
}

type aiCall struct {
	use, short, method string
	path               func([]string) string
	filters            map[string]string
	maxLimit           int
	report, model      bool
}

// JSON numbers retain their original decimal text; YAML integer counters are
// decoded as integers. This avoids rounding usage quantities through float64.
type aiDocument map[string]any

func (d *aiDocument) UnmarshalJSON(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	return decoder.Decode((*map[string]any)(d))
}

func (d *aiDocument) UnmarshalYAML(node *yaml.Node) error {
	var value map[string]any
	if err := node.Decode(&value); err != nil {
		return err
	}
	*d = value
	return nil
}

func (a aiCall) command() *cobra.Command {
	var file, payload string
	var limit int
	cmd := &cobra.Command{
		Use: a.use, Short: a.short, Args: cobra.ExactArgs(positionalCount(a.use)),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := url.Values{}
			for flag, name := range a.filters {
				if !flagChanged(cmd, flag) {
					continue
				}
				value, _ := cmd.Flags().GetString(flag)
				if flag == "from" || flag == "to" {
					if _, err := parseTimeFlag(value, flag); err != nil {
						return err
					}
				}
				query.Set(name, value)
			}
			if flagChanged(cmd, "limit") {
				if limit < 1 || limit > a.maxLimit {
					return errUsagef("--limit must be between 1 and %d", a.maxLimit)
				}
				query.Set("limit", fmt.Sprint(limit))
			}
			var body []byte
			if a.method == http.MethodPost || a.method == http.MethodPut {
				in, err := resolveFileOrPayload[aiDocument](file, payload)
				if err != nil {
					return errUsage(err)
				}
				if a.report {
					if err := validateAIReport(in); err != nil {
						return err
					}
				} else if a.model {
					if err := validateAIModel(in, args); err != nil {
						return err
					}
				} else if err := validateAIRuleDecimals(in); err != nil {
					return err
				}
				if a.method == http.MethodPut {
					in, err = a.mergeUpdate(cmd, args, in)
					if err != nil {
						return err
					}
				}
				body, err = json.Marshal(in)
				if err != nil {
					return errUsage(err)
				}
			}
			if a.method == http.MethodDelete {
				action := "archive AI conversion rule"
				if a.model {
					action = "delete AI catalogue model"
				}
				if err := confirmDestructive(cmd, action, strings.Join(args, " ")); err != nil {
					return err
				}
			}
			return callAIControlPlane(cmd, a.method, a.path(args), query, body)
		},
	}
	for flag, name := range a.filters {
		cmd.Flags().String(flag, "", "Filter or page by "+name)
	}
	if a.maxLimit > 0 {
		cmd.Flags().IntVar(&limit, "limit", 0, fmt.Sprintf("Page size (1 to %d); use --cursor for the next page", a.maxLimit))
	}
	if a.method == http.MethodPost || a.method == http.MethodPut {
		noun := "conversion rule"
		if a.report {
			noun = "AI usage report"
		} else if a.model {
			noun = "catalogue model"
		}
		addInputSourceFlags(cmd, &file, &payload, noun)
	}
	if a.method == http.MethodDelete {
		addConfirmFlag(cmd)
	}
	return cmd
}

// Update commands expose patch semantics over replacement PUT endpoints. A fresh
// GET avoids resetting omitted fields to API defaults. There is no CAS contract:
// a concurrent writer between this GET and PUT can still be overwritten.
func (a aiCall) mergeUpdate(cmd *cobra.Command, args []string, patch aiDocument) (aiDocument, error) {
	fields := []string{"entitlementSlug", "modelSlug", "requestType", "rates", "minimumCreditsPerRequest", "maximumCreditsPerRequest", "chargeOnError"}
	identities := []string{"entitlementSlug", "modelSlug", "requestType"}
	if a.model {
		fields = []string{"slug", "name", "vendor", "modality", "status", "description", "externalIds", "metadata", "vendorCosts"}
		identities = []string{"slug", "modality"}
	}
	for key := range patch {
		allowed := false
		for _, field := range fields {
			if key == field {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil, errUsagef("%s is not a writable field for this update", key)
		}
	}
	data, _, err := requestAIControlPlane(cmd, http.MethodGet, a.path(args), nil, nil)
	if err != nil {
		return nil, err
	}
	var current aiDocument
	if err := json.Unmarshal(data, &current); err != nil || current == nil {
		return nil, fmt.Errorf("decode current AI resource: expected a JSON object")
	}
	for _, field := range identities {
		if value, present := patch[field]; present && !reflect.DeepEqual(value, current[field]) {
			return nil, errUsagef("%s is immutable; omit it or echo the current value", field)
		}
	}
	merged := aiDocument{}
	for _, field := range fields {
		if value, present := current[field]; present {
			merged[field] = value
		}
	}
	for field, value := range patch {
		if a.model && (field == "vendorCosts" || field == "metadata") {
			if changes, ok := value.(map[string]any); ok {
				combined := map[string]any{}
				if existing, ok := current[field].(map[string]any); ok {
					for key, value := range existing {
						combined[key] = value
					}
				}
				for key, value := range changes {
					combined[key] = value
				}
				merged[field] = combined
				continue
			}
		}
		// Rule rates intentionally replace the whole map. Explicit null for the
		// maximum cap is retained; identities have already been checked above.
		merged[field] = value
	}
	return merged, nil
}

func validateAIModel(in aiDocument, args []string) error {
	if in == nil {
		return errUsagef("model must be an object")
	}
	if len(args) > 0 {
		if slug, present := in["slug"]; present && slug != args[0] {
			return errUsagef("model slug is immutable: payload slug must match %s or be omitted", args[0])
		}
	}
	if value, present := in["vendorCosts"]; present {
		costs, ok := value.(map[string]any)
		if !ok {
			return errUsagef("vendorCosts must be an object with quoted decimal strings")
		}
		for name, cost := range costs {
			if _, ok := cost.(string); !ok {
				return errUsagef("vendorCosts.%s must be a quoted string", name)
			}
		}
	}
	return nil
}

func validateAIReport(in aiDocument) error {
	value, ok := in["value"].(map[string]any)
	if !ok || value["type"] != "ai_usage" {
		return errUsagef("AI report requires value.type ai_usage; numeric adjustments use instances usage report")
	}
	if value["phase"] != "reserve" && value["phase"] != "commit" {
		return errUsagef("AI report value.phase must be reserve or commit")
	}
	transactionID, _ := value["transactionId"].(string)
	if strings.TrimSpace(transactionID) == "" {
		return errUsagef("AI report requires value.transactionId; use the same identity for reserve and commit")
	}
	if behavior, present := in["behavior"]; present && behavior != "append" {
		return errUsagef("AI report behavior must be omitted or append")
	}
	if usage, ok := value["usage"].(map[string]any); ok {
		if units, present := usage["units"]; present {
			if _, ok := units.(string); !ok {
				return errUsagef("value.usage.units must be a quoted decimal string")
			}
		}
	}
	return nil
}

// Rates and credit bounds are decimal strings on the wire, never float64.
// Reject unquoted YAML/JSON numbers rather than silently rounding them.
func validateAIRuleDecimals(in map[string]any) error {
	if in == nil {
		return errUsagef("conversion rule must be an object")
	}
	rates, ok := in["rates"].(map[string]any)
	if !ok {
		return errUsagef("conversion rule requires a rates object with quoted decimal strings")
	}
	for name, value := range rates {
		if value == nil {
			continue
		}
		if _, ok := value.(string); !ok {
			return errUsagef("rates.%s must be a quoted decimal string", name)
		}
	}
	for _, name := range []string{"minimumCreditsPerRequest", "maximumCreditsPerRequest"} {
		if value, present := in[name]; present && (name != "maximumCreditsPerRequest" || value != nil) {
			if _, ok := value.(string); !ok {
				return errUsagef("%s must be a quoted decimal string", name)
			}
		}
	}
	return nil
}

func callAIControlPlane(cmd *cobra.Command, method, path string, query url.Values, body []byte) error {
	data, headers, err := requestAIControlPlane(cmd, method, path, query, body)
	if err != nil {
		return err
	}
	if method == http.MethodDelete && len(data) == 0 {
		message := "Conversion rule archived"
		if strings.HasPrefix(path, "/models/") {
			message = "Catalogue model deleted"
		}
		return writeMessage(cmd, message)
	}
	if headers.Get("Idempotent-Replayed") == "true" {
		fmt.Fprintln(cmd.ErrOrStderr(), "note: this AI transaction phase was already recorded; this is its original result")
	}
	cfg, err := runtimeConfig(cmd)
	if err != nil {
		return err
	}
	return writeAIResponse(cmd, cfg.Output, data)
}

func requestAIControlPlane(cmd *cobra.Command, method, path string, query url.Values, body []byte) ([]byte, http.Header, error) {
	_, cfg, err := newClient(cmd)
	if err != nil {
		return nil, nil, err
	}
	if method == "" {
		method = http.MethodGet
	}
	// Concatenating the escaped operation path preserves the configured /api
	// prefix; ResolveReference with an absolute operation path would drop it.
	endpoint := strings.TrimRight(cfg.BaseURL, "/") + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	ctx, cancel := commandContext(cmd)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, nil, errUsage(err)
	}
	if cfg.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.AuthToken)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	// Do not forward credentials or replay management writes through redirects.
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		apiErr := &sdk.Error{StatusCode: resp.StatusCode, Status: resp.Status, Body: data}
		var problem sdk.ErrorModel
		if json.Unmarshal(data, &problem) == nil {
			apiErr.Problem = &problem
		}
		return nil, nil, apiErr
	}
	return data, resp.Header, nil
}

func writeAIResponse(cmd *cobra.Command, format string, data []byte) error {
	format = strings.TrimSpace(format)
	if !json.Valid(data) {
		return fmt.Errorf("decode AI response: expected JSON")
	}
	if format == output.FormatJSON {
		return output.Write(cmd.OutOrStdout(), format, json.RawMessage(data), output.Table{})
	}
	if format == output.FormatNDJSON {
		// JSON/YAML preserve page cursors. NDJSON prints just one document per
		// item, matching the CLI's list-output convention.
		var page struct {
			Items json.RawMessage `json:"items"`
		}
		var items []json.RawMessage
		if json.Unmarshal(data, &page) == nil && page.Items != nil {
			if err := json.Unmarshal(page.Items, &items); err != nil {
				return fmt.Errorf("decode AI response items: %w", err)
			}
		} else if json.Unmarshal(data, &items) != nil {
			return output.Write(cmd.OutOrStdout(), format, json.RawMessage(data), output.Table{})
		}
		return output.Write(cmd.OutOrStdout(), format, items, output.Table{})
	}
	// Parsing JSON as a YAML node retains scalar types and exact numeric text,
	// unlike decoding through interface{} (float64) or json.Number (YAML string).
	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return fmt.Errorf("decode AI response: %w", err)
	}
	return output.Write(cmd.OutOrStdout(), output.FormatYAML, &node, output.Table{})
}
