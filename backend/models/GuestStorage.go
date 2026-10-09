package models

// EffectiveGuestStorageID preserves the legacy default when no guest override
// is configured. It does not enable guest authentication or grant user access.
func (s Settings) EffectiveGuestStorageID() int {
	if s.GuestStorage > 0 {
		return s.GuestStorage
	}
	return s.DefaultStorage
}
