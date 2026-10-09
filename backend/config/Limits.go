package config

// Hard ceilings protect resources and match persistent column constraints.
const (
	DefaultMaxUploadFiles          = 10
	HardMaxUploadFiles             = 50
	DefaultTagMaxLength            = 10
	HardTagMaxLength               = 50
	DefaultRandomImageLimit        = 20
	HardRandomImageLimit           = 100
	DefaultPowVerifyTimeoutSeconds = 5
	HardPowVerifyTimeoutSeconds    = 15
	LegacyPowVerifyURL             = "https://cha.eta.im/api/validate"
)
