package cmd

import (
	"encoding/json"

	"github.com/kaitencloud/cli/internal/input"
	"github.com/kaitencloud/sdk-go"
	"github.com/spf13/cobra"
)

// licenseValueFlags is the entitlement value a license carries, in the six mutually
// exclusive shapes the CLI accepts for it: a whole document from a file or an inline
// payload, a bare number or boolean, or an object from a file or an inline payload.
type licenseValueFlags struct {
	file           string
	payload        string
	number         float64
	boolean        bool
	objectFile     string
	objectValue    string
	overagePercent int32
}

func (f *licenseValueFlags) register(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&f.file, "file", "f", "", "Path to the license entitlement JSON or YAML file")
	cmd.Flags().StringVar(&f.payload, "payload", "", "Inline license entitlement JSON or YAML payload")
	cmd.Flags().Float64Var(&f.number, "number", 0, "Number entitlement value")
	cmd.Flags().BoolVar(&f.boolean, "boolean", false, "Boolean entitlement value")
	cmd.Flags().StringVar(&f.objectFile, "object-file", "", "JSON or YAML object entitlement value file")
	cmd.Flags().StringVar(&f.objectValue, "object-payload", "", "Inline JSON or YAML object entitlement value")
	cmd.Flags().Int32Var(&f.overagePercent, "overage-percent", 0,
		"How far usage may exceed a numeric value before reports are refused: 0 is a hard limit, "+
			"20 accepts up to 20% more, -1 goes with the unlimited value -1")
}

// options returns the grant options the invocation set: the allowance, when --overage-percent
// was given, and nothing otherwise.
func (f *licenseValueFlags) options(cmd *cobra.Command) []sdk.LicenseEntitlementOption {
	if !flagChanged(cmd, "overage-percent") {
		return nil
	}
	return []sdk.LicenseEntitlementOption{sdk.WithOveragePercent(f.overagePercent)}
}

// keptOveragePercent is the allowance an update carries over from the grant it replaces. The
// API derives an omitted allowance from the value -- none, for a finite one -- so an update
// that only changes the value would turn a soft limit into a hard one. The current allowance
// is kept when it still fits the new value: a finite number. An unlimited value takes -1,
// which the API derives itself, and a grant that was unlimited has no allowance to keep.
func keptOveragePercent(current sdk.LicenseEntitlement, value sdk.LicenseEntitlementValue) (int32, bool) {
	percent := current.LimitCapExceededOveragePercent
	if percent == nil || *percent < 0 {
		return 0, false
	}
	number, ok := numberValue(value)
	if !ok || number == unlimitedValue {
		return 0, false
	}
	return *percent, true
}

// unlimitedValue is the numeric grant value the API reads as "no limit".
const unlimitedValue = -1

// numberValue returns the number a grant value holds, and false for a boolean or an object.
func numberValue(value sdk.LicenseEntitlementValue) (float64, bool) {
	raw, err := json.Marshal(value)
	if err != nil {
		return 0, false
	}
	var typed struct {
		Type  string `json:"type"`
		Value any    `json:"value"`
	}
	if err := json.Unmarshal(raw, &typed); err != nil || typed.Type != "number" {
		return 0, false
	}
	number, ok := typed.Value.(float64)
	return number, ok
}

// resolve turns the single value flag the invocation used into an entitlement value.
// --number 0 and --boolean=false are legitimate values, so the two are recognised by
// whether they were set rather than by what they hold.
func (f *licenseValueFlags) resolve(cmd *cobra.Command) (sdk.LicenseEntitlementValue, error) {
	hasNumber := flagChanged(cmd, "number")
	hasBoolean := flagChanged(cmd, "boolean")

	sources := 0
	for _, given := range []bool{
		f.file != "",
		f.payload != "",
		hasNumber,
		hasBoolean,
		f.objectFile != "",
		f.objectValue != "",
	} {
		if given {
			sources++
		}
	}
	if sources != 1 {
		return sdk.LicenseEntitlementValue{}, errUsagef(
			"set exactly one of --file, --payload, --number, --boolean, --object-file, or --object-payload")
	}

	switch {
	case f.file != "":
		return input.LoadFile[sdk.LicenseEntitlementValue](f.file)
	case f.payload != "":
		return input.LoadString[sdk.LicenseEntitlementValue](f.payload)
	case hasNumber:
		return sdk.NumberLicenseValue(f.number)
	case hasBoolean:
		return sdk.BooleanLicenseValue(f.boolean)
	}

	var (
		value map[string]any
		err   error
	)
	if f.objectFile != "" {
		value, err = input.LoadFile[map[string]any](f.objectFile)
	} else {
		value, err = input.LoadString[map[string]any](f.objectValue)
	}
	if err != nil {
		return sdk.LicenseEntitlementValue{}, err
	}
	return sdk.ConfigLicenseValue(value)
}
