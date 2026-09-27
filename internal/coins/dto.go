package coins

// The request DTOs below are the partial-update shape of
// PUT /api/v1/admin/coins/economy (03-api-contract.md §3.1).
//
// Every leaf is a pointer, including the group structs, so the merge can
// distinguish three states that a value type cannot: "not mentioned" (keep the
// current value), "mentioned" (apply it), and "mentioned as an explicit
// zero/null" (apply the zero). An admin changing prices.study_resource from 40
// to 60 sends one number; everything else, including the whole referral block,
// keeps the value already stored.
//
// The response type is EconomyConfig itself rather than a separate DTO. The
// same object is the system_settings payload, the GET response and the PUT
// response, so its json tags are the single wire contract; a second struct
// mirroring those fifteen fields would be a second thing to forget to update
// the next time a price is added.

// UpdateEconomyConfigRequest is a partial update. Omitted keys retain their
// current stored value.
type UpdateEconomyConfigRequest struct {
	Prices             *UpdatePricesRequest    `json:"prices"`
	Awards             *UpdateAwardsRequest    `json:"awards"`
	Allowance          *UpdateAllowanceRequest `json:"allowance"`
	Expiry             *UpdateExpiryRequest    `json:"expiry"`
	Referral           *UpdateReferralRequest  `json:"referral"`
	ClawbackWindowDays *int64                  `json:"clawback_window_days"`
}

type UpdatePricesRequest struct {
	StudyResource *int64 `json:"study_resource"`
	Video         *int64 `json:"video"`
	MockTest      *int64 `json:"mock_test"`
}

type UpdateAwardsRequest struct {
	ProfileComplete    *int64 `json:"profile_complete"`
	ProfileInstalment  *int64 `json:"profile_instalment"`
	ProfileInstalments *int64 `json:"profile_instalments"`
	ReferralReferrer   *int64 `json:"referral_referrer"`
	ReferralReferred   *int64 `json:"referral_referred"`
	ResourceApproved   *int64 `json:"resource_approved"`
}

type UpdateAllowanceRequest struct {
	DocumentUnlocks *int64 `json:"document_unlocks"`
	VideoUnlocks    *int64 `json:"video_unlocks"`
	MockTestUnlocks *int64 `json:"mock_test_unlocks"`
	ExpiresInDays   *int64 `json:"expires_in_days"`
}

type UpdateExpiryRequest struct {
	FreeDays           *int64 `json:"free_days"`
	EarnedDays         *int64 `json:"earned_days"`
	ActivityExtendDays *int64 `json:"activity_extend_days"`
}

type UpdateReferralRequest struct {
	MonthlyCap      *int64 `json:"monthly_cap"`
	LifetimeCoinCap *int64 `json:"lifetime_coin_cap"`
	HoldDays        *int64 `json:"hold_days"`
}

// applyEconomyConfigUpdate merges req onto base and returns the result. It is
// pure: nothing is written and nothing is validated here, so the merge can be
// tested on its own and validation always sees a whole config.
//
// A group that is present but empty changes nothing, which is the same
// outcome as omitting it. Validation still runs over the merged result, so a
// stored value that was somehow already invalid is surfaced by the write that
// touches anything rather than lingering.
func applyEconomyConfigUpdate(base EconomyConfig, req UpdateEconomyConfigRequest) EconomyConfig {
	out := base

	if p := req.Prices; p != nil {
		assignInt64(&out.Prices.StudyResource, p.StudyResource)
		assignInt64(&out.Prices.Video, p.Video)
		assignInt64(&out.Prices.MockTest, p.MockTest)
	}
	if a := req.Awards; a != nil {
		assignInt64(&out.Awards.ProfileComplete, a.ProfileComplete)
		assignInt64(&out.Awards.ProfileInstalment, a.ProfileInstalment)
		assignInt64(&out.Awards.ProfileInstalments, a.ProfileInstalments)
		assignInt64(&out.Awards.ReferralReferrer, a.ReferralReferrer)
		assignInt64(&out.Awards.ReferralReferred, a.ReferralReferred)
		assignInt64(&out.Awards.ResourceApproved, a.ResourceApproved)
	}
	if al := req.Allowance; al != nil {
		assignInt64(&out.Allowance.DocumentUnlocks, al.DocumentUnlocks)
		assignInt64(&out.Allowance.VideoUnlocks, al.VideoUnlocks)
		assignInt64(&out.Allowance.MockTestUnlocks, al.MockTestUnlocks)
		assignInt64(&out.Allowance.ExpiresInDays, al.ExpiresInDays)
	}
	if e := req.Expiry; e != nil {
		assignInt64(&out.Expiry.FreeDays, e.FreeDays)
		assignInt64(&out.Expiry.EarnedDays, e.EarnedDays)
		assignInt64(&out.Expiry.ActivityExtendDays, e.ActivityExtendDays)
	}
	if r := req.Referral; r != nil {
		assignInt64(&out.Referral.MonthlyCap, r.MonthlyCap)
		assignInt64(&out.Referral.LifetimeCoinCap, r.LifetimeCoinCap)
		assignInt64(&out.Referral.HoldDays, r.HoldDays)
	}
	assignInt64(&out.ClawbackWindowDays, req.ClawbackWindowDays)

	return out
}

// assignInt64 applies a partial-update leaf: a nil pointer means the key was
// not mentioned and the current value stands.
func assignInt64(dst *int64, src *int64) {
	if src != nil {
		*dst = *src
	}
}
