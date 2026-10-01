// cmd/server/referral_wiring_test.go
//
// The one place the two referral subject vocabularies meet, asserted rather than
// assumed.
//
// internal/auth and internal/coins both declare the three account tables an account
// can be created in, and neither may import the other — auth must compile and run
// with no coin economy configured, so it names what it can create itself. That
// duplication is the price of the dependency direction and it has exactly one
// failure mode: the two lists drift, and a NEW account table attributes as the wrong
// subject. A wrong subject is a row whose referred_user_id points into a table the
// account is not in — and the uniqueness constraint on (referred_kind,
// referred_user_id) then refuses the legitimate attribution of the real account that
// does own that id. Silent, and it only shows up as an under-counted referral.
package main

import (
	"sort"
	"testing"

	"studsphere/backend/internal/auth"
	"studsphere/backend/internal/coins"
)

// The mapping must be bijective: every subject auth can produce has exactly one coin
// -system subject, and vice versa. A subject on one side with no mapping is a
// runtime error; a subject mapped twice is one of them wrong.
func TestReferralSubjectMappingCoversEveryAuthSubject(t *testing.T) {
	authSubjects := []string{
		auth.ReferralSubjectUser,
		auth.ReferralSubjectInstitution,
		auth.ReferralSubjectProvider,
	}
	coinsSubjects := coins.ReferredKinds

	// Every auth subject maps.
	for _, subject := range authSubjects {
		mapped, ok := referralSubjectKinds[subject]
		if !ok {
			t.Errorf("auth can create a %q account and the adapter has no coin-system "+
				"subject for it. An unmapped subject is refused at runtime, so every "+
				"signup of that kind silently records no referral", subject)
			continue
		}
		// And it maps onto something the coin system's CHECK constraint accepts —
		// which it would not if the two lists have drifted apart by a typo.
		found := false
		for _, allowed := range coinsSubjects {
			if mapped == allowed {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("auth's %q maps to %q, which is not in coins.ReferredKinds (%v); "+
				"chk_user_referral_kind would refuse every row naming it", subject, mapped, coinsSubjects)
		}
	}

	// Every coin-system subject is reachable.
	used := map[string]bool{}
	for _, mapped := range referralSubjectKinds {
		used[mapped] = true
	}
	for _, subject := range coinsSubjects {
		if !used[subject] {
			t.Errorf("coins.ReferredKinds contains %q and nothing in auth maps to it", subject)
		}
	}

	// And no two auth subjects share a mapping.
	byTarget := map[string]string{}
	for from, to := range referralSubjectKinds {
		if prev, dup := byTarget[to]; dup {
			t.Errorf("auth's %q and %q both map to %q, so one of them attributes as the "+
				"wrong account type", prev, from, to)
		}
		byTarget[to] = from
	}

	// Sizes equal, so a fourth account table on either side is caught here.
	if len(authSubjects) != len(coinsSubjects) {
		sort.Strings(authSubjects)
		sort.Strings(coinsSubjects)
		t.Errorf("auth knows %d account subjects (%v) and coins knows %d (%v). A new "+
			"account table has to be added to BOTH, and the enumeration in "+
			"internal/auth/referral.go with it",
			len(authSubjects), authSubjects, len(coinsSubjects), coinsSubjects)
	}
}

// An unmapped subject must be an ERROR rather than a plausible-looking default.
//
// The tempting implementation maps anything unrecognised to "user", which is wrong
// for an institution and wrong in the dangerous direction: a provider signup would
// record an attribution against a student id, and the uniqueness constraint would
// then refuse the real student that owns it.
func TestReferralAdapterRefusesAnUnmappedSubject(t *testing.T) {
	adapter := &referralAttributorAdapter{}
	if _, err := adapter.ApplyReferral(t.Context(), auth.ReferralSubject{
		Kind: "not_a_table",
		ID:   1,
		Code: "K7M2QX9RT4",
	}); err == nil {
		t.Fatal("an unmapped subject was accepted; a plausible default would record a " +
			"referral against the wrong table's id space")
	}

	// And a nil service is an error too, so a missing coin economy is loud rather
	// than a silent "no referral".
	if _, err := adapter.ApplyReferral(t.Context(), auth.ReferralSubject{
		Kind: auth.ReferralSubjectUser,
		ID:   1,
		Code: "K7M2QX9RT4",
	}); err == nil {
		t.Fatal("a nil referral service was accepted; an unwired attributor must not look " +
			"like a successful attribution")
	}
}
