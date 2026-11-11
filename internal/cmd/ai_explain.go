package cmd

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/kaitencloud/cli/internal/output"
	"github.com/kaitencloud/sdk-go"
	"github.com/spf13/cobra"
)

// What the explainable-charges contract adds to the AI commands: the vendor cost of a
// commit, the inference history's filters and its organization-wide listing, refunds,
// and table output for the records that explain a charge.

const inferencesListLong = `List one page of AI inference accounting records.

With an instance and a credit pool, the records of that pool; without them, the
records of every instance and pool of the organization, which --instance and
--entitlement narrow. Each record says what priced it (the rule snapshot and how it
matched), the licence whose rules were considered, its warnings, its vendor cost and
its refunds. A release that closed no reservation is listed only with
--status RELEASED or --transaction-id.`

var (
	vendorCostPattern  = regexp.MustCompile(`^[0-9]{1,12}(\.[0-9]{1,12})?$`)
	refundCredits      = regexp.MustCompile(`^[0-9]{1,12}(\.[0-9]{1,6})?$`)
	idempotencyKeyForm = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)
	aiRequestTypes     = []string{"COMPLETION", "EMBEDDING", "IMAGE_GENERATION", "AUDIO", "VIDEO"}
)

func inferencesPath(args []string) string {
	if len(args) == 0 {
		return "/usage/inferences"
	}
	return aiUsagePath("inferences")(args)
}

func inferencesListArgs(_ *cobra.Command, args []string) error {
	if len(args) != 0 && len(args) != 2 {
		return fmt.Errorf("accepts an instance slug and an entitlement slug, or neither for the organization's records; received %d", len(args))
	}
	return nil
}

func validateInferencesListFlags(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		for _, flag := range []string{"instance", "entitlement"} {
			if flagChanged(cmd, flag) {
				return errUsagef("--%s filters the organization's records: omit the instance and entitlement arguments to use it", flag)
			}
		}
	}
	if flagChanged(cmd, "authorized") {
		if value, _ := cmd.Flags().GetString("authorized"); value != "true" && value != "false" {
			return errUsagef("--authorized must be true or false")
		}
	}
	if flagChanged(cmd, "request-type") {
		value, _ := cmd.Flags().GetString("request-type")
		if !slices.Contains(aiRequestTypes, value) {
			return errUsagef("--request-type must be one of %s", strings.Join(aiRequestTypes, ", "))
		}
	}
	return nil
}

// setReportVendorCost writes --vendor-cost into the report's value, refusing a payload
// that already names another one.
func setReportVendorCost(in aiDocument, cost string) error {
	value, ok := in["value"].(map[string]any)
	if !ok {
		return errUsagef("AI report requires value.type ai_usage; numeric adjustments use instances usage report")
	}
	if existing, present := value["vendorCost"]; present && existing != cost {
		return errUsagef("--vendor-cost %s conflicts with value.vendorCost %v in the payload", cost, existing)
	}
	value["vendorCost"] = cost
	return nil
}

// validateReportVendorCost refuses what the API would: a vendor cost on a reserve or on
// a NOT_SENT commit, or one that is not a quoted decimal of the accepted precision.
func validateReportVendorCost(value map[string]any) error {
	cost, present := value["vendorCost"]
	if !present {
		return nil
	}
	if value["phase"] != "commit" {
		return errUsagef("value.vendorCost is commit only: a reserve has not reached the provider")
	}
	if value["requestStatus"] == "NOT_SENT" {
		return errUsagef("value.vendorCost cannot be sent on a NOT_SENT commit, which charges nothing")
	}
	text, ok := cost.(string)
	if !ok || !vendorCostPattern.MatchString(text) {
		return errUsagef("value.vendorCost must be a quoted decimal in USD with at most 12 integer and 12 fractional digits, e.g. \"0.0123\"")
	}
	return nil
}

func newAIRefundCommand() *cobra.Command {
	var credits, reason, key string
	cmd := &cobra.Command{
		Use:   "refund <instance-slug> <entitlement-slug> <transaction-id>",
		Short: "Give back credits charged for one committed AI inference",
		Long: `Give back credits charged for one COMMITTED AI inference, in full or in part.

The refund is a negative usage report linked to the inference: it lands in the
usage window open now and is listed on the inference's refunds. Without --credits,
everything the inference has left to refund is given back. It moves credits, so it
asks for confirmation unless --yes is given.

Every refund is sent under an idempotency key. Without --idempotency-key one is
generated and printed on stderr: run the command again with it to retry a refund
whose outcome you do not know, and the API applies it once.`,
		Example: `  kaiten ai inferences refund acme-prod ai-credits req-42 --credits 3.5 --reason "duplicate call"
  kaiten ai inferences refund acme-prod ai-credits req-42 --idempotency-key refund-req-42-1 --yes`,
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagChanged(cmd, "credits") && (!refundCredits.MatchString(credits) || strings.Trim(strings.ReplaceAll(credits, ".", ""), "0") == "") {
				return errUsagef("--credits must be a positive decimal with at most 12 integer and 6 fractional digits")
			}
			if flagChanged(cmd, "reason") && (reason == "" || utf8.RuneCountInString(reason) > 500) {
				return errUsagef("--reason must be 1 to 500 characters; omit it to give none")
			}
			generated := !flagChanged(cmd, "idempotency-key")
			if generated {
				key = "cli-refund-" + rand.Text()
			} else if !idempotencyKeyForm.MatchString(key) {
				return errUsagef("--idempotency-key must be 1 to 128 characters of letters, digits, '.', '_', ':' and '-'")
			}

			amount := "everything left"
			if credits != "" {
				amount = credits + " credits"
			}
			if err := confirmDestructive(cmd, "refund "+amount+" of AI inference", args[2]+" on "+args[0]+"/"+args[1]); err != nil {
				return err
			}

			body := map[string]string{}
			if credits != "" {
				body["credits"] = credits
			}
			if reason != "" {
				body["reason"] = reason
			}
			encoded, err := json.Marshal(body)
			if err != nil {
				return err
			}
			if generated {
				fmt.Fprintf(cmd.ErrOrStderr(), "idempotency key: %s (pass --idempotency-key %s to retry this refund)\n", key, key)
			}

			path := aiUsagePath("inferences")(args[:2]) + "/" + url.PathEscape(args[2]) + "/refund"
			data, headers, err := requestAIControlPlaneWithHeaders(cmd, http.MethodPost, path, nil, encoded, http.Header{"Idempotency-Key": {key}})
			if err != nil {
				return err
			}
			if headers.Get("Idempotent-Replayed") == "true" {
				fmt.Fprintln(cmd.ErrOrStderr(), "note: this refund was already recorded under this idempotency key; nothing was refunded again")
			}
			cfg, err := runtimeConfig(cmd)
			if err != nil {
				return err
			}
			if isTableOutput(cfg.Output) {
				return writeAITable(cmd, data, refundTable)
			}
			return writeAIResponse(cmd, cfg.Output, data)
		},
	}
	cmd.Flags().StringVar(&credits, "credits", "", "Credits to give back, a decimal; omit to refund everything left")
	cmd.Flags().StringVar(&reason, "reason", "", "Why, kept with the refund and shown to anyone who can read the organization's instances")
	cmd.Flags().StringVar(&key, "idempotency-key", "", "The refund's key, to retry it safely; generated and printed when omitted")
	addConfirmFlag(cmd)
	return cmd
}

// aiErrorHints says, per problem code, what the refusal means for the person at the
// terminal. The *sdk.Error stays wrapped, so the exit code does not change.
var aiErrorHints = map[string]string{
	"AIUsage.Disabled": "AI usage is disabled for this organization",
	"ReportEntitlementUsageMetric.TransactionIdReused": "this transactionId is closed (committed or released) or was sent with another body: " +
		"a retry must resend the identical payload, value.vendorCost byte for byte",
	"RefundAiInference.ContextNotFound":        "no such instance or AI credit pool",
	"RefundAiInference.InferenceNotFound":      "no inference with this transaction id on this instance and credit pool",
	"RefundAiInference.NotCommitted":           "only a COMMITTED inference can be refunded; this one is reserved, released or expired",
	"RefundAiInference.NothingToRefund":        "the inference was charged nothing, or its charge is already refunded in full",
	"RefundAiInference.ExceedsRefundable":      "the refund exceeds what the inference has left to refund",
	"RefundAiInference.IdempotencyKeyReused":   "this idempotency key already holds another refund or report of the pool: send this refund under a new key",
	"RefundAiInference.PoolNotGranted":         "the instance no longer holds this credit pool, and a refund is a report against it",
	"RefundAiInference.IdempotencyKeyRequired": "a refund needs an idempotency key",
	"RefundAiInference.InvalidIdempotencyKey":  "--idempotency-key must be 1 to 128 characters of letters, digits, '.', '_', ':' and '-'",
	"RefundAiInference.InvalidAmount":          "--credits must be a positive decimal with at most 12 integer and 6 fractional digits",
	"RefundAiInference.InvalidReason":          "--reason must be 1 to 500 characters",
}

type aiHintedError struct {
	hint string
	err  *sdk.Error
}

func (e *aiHintedError) Error() string { return e.hint + " (" + e.err.Error() + ")" }

func (e *aiHintedError) Unwrap() error { return e.err }

func explainAIError(apiErr *sdk.Error) error {
	hint, ok := aiErrorHints[apiErr.Code]
	if !ok {
		return apiErr
	}
	if apiErr.Code == "RefundAiInference.ExceedsRefundable" && apiErr.Problem != nil && apiErr.Problem.Errors != nil {
		for _, detail := range *apiErr.Problem.Errors {
			if left, ok := detail.Value.(string); ok && detail.Location != nil && *detail.Location == "body.credits" {
				hint += ": " + left + " left"
			}
		}
	}
	return &aiHintedError{hint: hint, err: apiErr}
}

func isTableOutput(format string) bool {
	format = strings.TrimSpace(format)
	return format == "" || format == output.FormatTable
}

func writeAITable(cmd *cobra.Command, data []byte, render func([]byte) (output.Table, error)) error {
	table, err := render(data)
	if err != nil {
		return err
	}
	if err := output.WriteTable(cmd.OutOrStdout(), table); err != nil {
		return err
	}
	var page struct {
		HasMore    bool   `json:"hasMore"`
		NextCursor string `json:"nextCursor"`
	}
	if json.Unmarshal(data, &page) == nil && page.HasMore && page.NextCursor != "" {
		fmt.Fprintf(cmd.ErrOrStderr(), "more records: pass --cursor %s for the next page\n", page.NextCursor)
	}
	return nil
}

// aiRecord reads a JSON object with its numbers kept as written.
type aiRecord map[string]any

func decodeAIRecords(data []byte) ([]aiRecord, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var page struct {
		Items []aiRecord `json:"items"`
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		var items []aiRecord
		if err := decoder.Decode(&items); err != nil {
			return nil, fmt.Errorf("decode AI response: %w", err)
		}
		return items, nil
	}
	if err := decoder.Decode(&page); err != nil {
		return nil, fmt.Errorf("decode AI response: %w", err)
	}
	return page.Items, nil
}

func (r aiRecord) text(path ...string) string {
	var value any = map[string]any(r)
	for _, key := range path {
		object, ok := value.(map[string]any)
		if !ok {
			return ""
		}
		value = object[key]
	}
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			parts = append(parts, fmt.Sprint(item))
		}
		return strings.Join(parts, ",")
	default:
		return fmt.Sprint(v)
	}
}

func orDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

func inferencesTable(data []byte) (output.Table, error) {
	records, err := decodeAIRecords(data)
	if err != nil {
		return output.Table{}, err
	}
	organization := false
	for _, record := range records {
		if _, ok := record["instanceSlug"]; ok {
			organization = true
		}
		if _, ok := record["entitlementSlug"]; ok {
			organization = true
		}
	}
	columns := []string{"TRANSACTION", "STATUS", "MODEL", "AUTHORIZED", "CHARGED", "REFUNDED", "RULE", "LICENSE", "VENDOR COST (REPORTED/COMPUTED)", "WARNINGS", "REPORTED AT"}
	if organization {
		columns = append([]string{"INSTANCE", "POOL"}, columns...)
	}
	table := output.Table{Columns: columns, Rows: [][]string{}}
	for _, r := range records {
		rule := r.text("pricing", "specificity")
		if id := r.text("pricing", "ruleId"); id != "" {
			rule = strings.TrimSpace(rule + " " + id)
		}
		vendor := "-"
		if reported, computed := r.text("vendorCost", "reported"), r.text("vendorCost", "computed"); reported != "" || computed != "" {
			vendor = orDash(reported) + " / " + orDash(computed) + " " + r.text("vendorCost", "currency")
		}
		model := r.text("model")
		if slug := r.text("modelSlug"); slug != "" && slug != model {
			model += " (" + slug + ")"
		}
		row := []string{
			r.text("transactionId"), r.text("status"), orDash(model), orDash(r.text("authorized")),
			orDash(r.text("credits", "charged")), orDash(r.text("credits", "refunded")), orDash(rule),
			orDash(r.text("licenseSlug")), vendor, orDash(r.text("warnings")), r.text("reportedAt"),
		}
		if organization {
			row = append([]string{orDash(r.text("instanceSlug")), orDash(r.text("entitlementSlug"))}, row...)
		}
		table.Rows = append(table.Rows, row)
	}
	return table, nil
}

func refundTable(data []byte) (output.Table, error) {
	var refund aiRecord
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&refund); err != nil || refund == nil {
		return output.Table{}, errors.New("decode AI refund: expected a JSON object")
	}
	return output.Table{
		Columns: []string{"ID", "CREDITS", "REASON", "REPORT SEQ", "CREATED BY", "CREATED AT"},
		Rows: [][]string{{
			refund.text("id"), refund.text("credits"), orDash(refund.text("reason")), refund.text("reportSeq"),
			orDash(refund.text("createdBy", "name")), refund.text("createdAt"),
		}},
	}, nil
}

func conversionRulesTable(data []byte) (output.Table, error) {
	records, err := decodeAIRecords(data)
	if err != nil {
		return output.Table{}, err
	}
	table := output.Table{
		Columns: []string{"ID", "ENTITLEMENT", "MODEL", "REQUEST TYPE", "STATUS", "GRANTED BY LICENSE", "GRANTED BY ADD-ONS", "CREATED BY", "UPDATED BY"},
		Rows:    [][]string{},
	}
	for _, r := range records {
		table.Rows = append(table.Rows, []string{
			r.text("id"), r.text("entitlementSlug"), orDash(r.text("modelSlug")), orDash(r.text("requestType")), r.text("status"),
			orDash(r.text("grantedByLicense")), orDash(r.text("grantedByAddons")), orDash(r.text("createdBy", "name")), orDash(r.text("updatedBy", "name")),
		})
	}
	return table, nil
}
