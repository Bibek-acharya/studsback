// internal/coins/reason_description_test.go
//
// The wallet history's only explanation column.
//
// `describeReason` is what a student reads to understand money appearing in
// their balance, so its output is copy the product owns: a line that reads like
// a fragment ("Profile progress: your account") is a support question, and one
// that reads like jargon is worse. Pinned here as plain strings on purpose —
// these are sentences, and a sentence that changes silently has changed what
// every past and future transaction row tells a student.
//
// The cases without a ref matter most: they are the DEFAULT branch (`what =
// "your account"`), and the profile award is the one producer that always has
// no ref — which is exactly how the fragment reached the wallet.
package coins

import "testing"

func TestReasonDescriptionsReadAsEnglish(t *testing.T) {
	cases := []struct {
		name       string
		reasonCode string
		ref        *RefDTO
		want       string
	}{
		{
			// The reported line: a nil ref must not produce a fragment.
			name:       "profile award, no ref",
			reasonCode: ReasonProfileComplete,
			ref:        nil,
			want:       "Profile progress reward",
		},
		{
			name:       "unlock with a ref",
			reasonCode: ReasonResourceUnlock,
			ref:        &RefDTO{Type: ResourceTypeStudyResource, ID: 812},
			want:       "Unlocked: study_resource 812",
		},
		{
			name:       "referral qualified, no ref",
			reasonCode: ReasonReferralQualified,
			ref:        nil,
			want:       "Referral qualified: your account",
		},
		{
			// An unknown code renders as itself rather than as a guess: a wrong
			// reason on a balance line is the thing support cannot defend.
			name:       "unknown code passes through",
			reasonCode: "SOMETHING_NEW",
			ref:        nil,
			want:       "SOMETHING_NEW",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := describeReason(tc.reasonCode, tc.ref); got != tc.want {
				t.Errorf("describeReason(%q) = %q, want %q", tc.reasonCode, got, tc.want)
			}
		})
	}
}