package cmd

import (
	"github.com/kaitencloud/cli/internal/input"
	"github.com/kaitencloud/sdk-go"
	"github.com/spf13/cobra"
)

// licenseValueFlags is the entitlement value a license carries, in the six mutually
// exclusive shapes the CLI accepts for it: a whole document from a file or an inline
// payload, a bare number or boolean, or an object from a file or an inline payload.
type licenseValueFlags struct {
	file        string
	payload     string
	number      float64
	boolean     bool
	objectFile  string
	objectValue string
}

func (f *licenseValueFlags) register(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&f.file, "file", "f", "", "Path to the license entitlement JSON or YAML file")
	cmd.Flags().StringVar(&f.payload, "payload", "", "Inline license entitlement JSON or YAML payload")
	cmd.Flags().Float64Var(&f.number, "number", 0, "Number entitlement value")
	cmd.Flags().BoolVar(&f.boolean, "boolean", false, "Boolean entitlement value")
	cmd.Flags().StringVar(&f.objectFile, "object-file", "", "JSON or YAML object entitlement value file")
	cmd.Flags().StringVar(&f.objectValue, "object-payload", "", "Inline JSON or YAML object entitlement value")
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
