package version

const defaultValue = "dev"

// Value is overridden at build time with -ldflags "-X .../internal/version.Value=<version>".
var Value = defaultValue

func String() string {
	if Value == "" {
		return defaultValue
	}
	return Value
}
