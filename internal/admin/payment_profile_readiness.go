package admin

import "context"

func (s *SettingsService) paymentProfileReadiness(ctx context.Context, provider string) ([]string, bool) {
	if s.profiles == nil {
		return nil, false
	}
	profiles, err := s.PaymentProfiles(ctx)
	if err != nil {
		// Profile storage owns migrated accounts. An unavailable read cannot
		// make the provider look disabled through legacy setting defaults.
		return []string{"unavailable:payment_profile." + provider}, true
	}
	found, missingProfile, missingMerchant := false, false, false
	for _, profile := range profiles {
		if profile.Provider != provider {
			continue
		}
		found = true
		if !profile.Enabled {
			continue
		}
		missingProfile = missingProfile || !profile.Configured || len(profile.EnabledChannels) == 0
		missingMerchant = missingMerchant || provider == "ezpay" && profile.MerchantID == ""
	}
	var issues []string
	if missingProfile {
		issues = append(issues, "missing:payment_profile."+provider)
	}
	if missingMerchant {
		issues = append(issues, "missing:payment_profile."+provider+".merchant_id")
	}
	return issues, found
}
