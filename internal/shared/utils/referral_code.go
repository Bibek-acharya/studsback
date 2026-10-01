// internal/shared/utils/referral_code.go
//
// The StudsToken referral code: how it is generated, and how it is normalised.
//
// ── why the generator lives here and not in internal/coins ────────────────────
//
// The code is a COLUMN on the users table, and internal/auth owns that table. The
// generator needs no coin-system knowledge — it is crypto/rand plus an alphabet —
// so putting it in internal/coins would mean internal/auth importing internal/coins
// to mint a string, which is the dependency inversion profile_award.go refuses.
// utils already holds GenerateOTP and GenerateRandomPassword, both of which use
// crypto/rand the same way, so this is the house location rather than a new one.
//
// The ECONOMY of the code — the caps, the uniqueness, the attribution table —
// lives in internal/coins. What lives here is only "what does a code look like".
//
// ── why a random code at all, and why this length ─────────────────────────────
//
// A referral code is a bearer token for a relationship: whoever holds it can
// credit somebody's account. Three properties follow, and only the first is about
// fraud.
//
//  1. It must not be GUESSABLE. Online brute force is bounded by rate limiting
//     and by the fact that a wrong code simply attributes nothing — there is no
//     "is this code valid" endpoint to enumerate against, which is deliberate and
//     is itself part of the design. An offline attacker gets the space below.
//
//  2. It must not be ENUMERABLE, which is a different and easier-to-forget
//     failure. A sequential or timestamp-derived code ("user000137", "1712004-a")
//     leaks two things at once: the total number of accounts, and the order in
//     which people joined. Referral relationships are personal — "who invited
//     whom" is a social graph, and a social graph that can be reconstructed by
//     incrementing a counter is a privacy leak (Privacy Act 2075, and the
//     position in docs/coin-system/07-compliance-nepal.md) as well as a fraud
//     route. So: no counter, no timestamp, no user id, nothing derivable. The
//     code is uniform over its whole space.
//
//  3. It must be TYPEABLE. It travels by chat message, by a printed leaflet, and
//     sometimes by being read aloud across a room.
//
// The alphabet is Crockford base32 minus I, L, O and U — 32 symbols, no character
// pair a human confuses (1/I/L, 0/O, U/V). 5 bits per character.
//
// The length is 10 characters, so 32^10 = 2^50 ≈ 1.1 × 10^15 distinct codes:
//
//   - Enumeration: even at 10^9 attempts a second — far beyond any HTTP endpoint
//     that exists — a full sweep is ~18 days. Realistically the cost is a request
//     per attempt, which is the thing rate limiting is for.
//   - Collisions: birthday probability ≈ n^2 / 2^51. At 1,000,000 users that is
//     about 0.04%, i.e. roughly one collision per 2,500 codes generated. That is
//     NOT acceptable to ignore, which is why generation is retried on conflict
//     rather than assumed collision-free — see internal/auth/referral_code.go.
//     At 10 characters the birthday bound sits at ~33M users, comfortably past
//     any launch projection, so the retry loop stays a rare path rather than a
//     hot one.
//   - Typing: 10 characters is short enough to read aloud and paste from a URL
//     bar, and long enough that nobody types it from memory.
//
// Eight characters (2^40) would still resist online guessing, but its birthday
// bound is ~1M users — the retry loop would fire constantly and, more importantly,
// the code would carry less margin against a future offline attack over a leaked
// table. Twelve (2^60) buys margin nobody needs at launch and costs real typing
// errors, which cost real referrals. Ten is the balance point, and the argument
// is recorded here so the next person to "just make it 6 for convenience" can see
// what they would be giving up.
package utils

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
)

// ReferralCodeLength is the number of characters in a referral code.
//
// Ten characters of Crockford base32 is 50 bits. See the file header for the
// enumeration, collision and typing arithmetic behind that number.
const ReferralCodeLength = 10

// referralCodeAlphabet is the 32 symbols a code is drawn from: the digits and the
// uppercase letters, minus I, L, O and U.
//
// The omissions are not cosmetic. 1/I/L and 0/O are the two pairs people
// mistype when reading a code off a screen or transcribing it by hand, and U is
// dropped alongside V for the same reason. Removing four of 36 symbols keeps the
// per-character entropy at exactly 5 bits, so the length arithmetic above stays a
// round number.
//
// It is NOT the full Crockford alphabet: Crockford also defines DECODE aliases
// (O reads as 0, I and L read as 1). NormalizeReferralCode applies those aliases
// on input, so a student who transcribes "O" for "0" still lands on the right
// code — but a code we generate never contains an aliasable character in the first
// place.
const referralCodeAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// referralAlphabetSize is len(referralCodeAlphabet). Asserted against the constant
// in GenerateReferralCode's own test rather than derived with len() at each use,
// because a silent edit to the alphabet string would otherwise just make the
// modulus wrong.
const referralAlphabetSize = 32

// GenerateReferralCode returns a fresh referral code.
//
// crypto/rand, matching GenerateOTP and GenerateRandomPassword in this package.
// math/rand is not acceptable here: it is seeded from the clock, so a code
// sequence is reproducible by anyone who knows roughly when the process started,
// which is the "timestamp-derived" failure the file header rules out.
//
// The alphabet-index draw uses rand.Int over a 32-element modulus rather than
// taking a random byte modulo 32, because modulo on a byte discards the low bits
// of the entropy source and biases the distribution towards the first 8 symbols
// (256 = 8 × 32 exactly, so every byte maps to 8 valid and 8 invalid values — the
// bias is small but it is real, and rand.Int is not more code).
//
// A failure of the entropy source is returned as an error rather than papered
// over with a weak fallback. The caller retries; see
// internal/auth/referral_code.go for why a collision must never fail a signup.
func GenerateReferralCode() (string, error) {
	if len(referralCodeAlphabet) != referralAlphabetSize {
		// Unreachable: a compile-time-sized constant string. Present because the
		// modulus below is wrong if the alphabet changes length, and a wrong
		// modulus produces a subtly non-uniform code rather than a crash.
		return "", fmt.Errorf("referral alphabet is %d symbols, expected %d", len(referralCodeAlphabet), referralAlphabetSize)
	}
	out := make([]byte, ReferralCodeLength)
	for i := range out {
		n, err := rand.Int(rand.Reader, big.NewInt(referralAlphabetSize))
		if err != nil {
			return "", err
		}
		out[i] = referralCodeAlphabet[n.Int64()]
	}
	return string(out), nil
}

// NormalizeReferralCode puts whatever the client sent into the canonical form the
// stored codes are in: uppercase, and with the characters a human introduces by
// accident removed.
//
// This exists because the code is a HUMAN-INPUT field, and a mismatch between
// "the code as stored" and "the code as typed" is a lost referral rather than a
// validation error — the student is told nothing, because from the server's point
// of view they simply never arrived with a code. Three classes of noise:
//
//   - case. "Ab3dEf9" and "AB3DEF9" are the same code. The unique index is on the
//     canonical form, so storing what the client sent would let one code exist
//     twice in two spellings and neither would collide.
//   - separators. A space or a hyphen, from a chat client auto-formatting or from
//     someone reading "ABCDE FGHIJ" off a printed leaflet. Codes are generated
//     without separators, so any separator in the input is noise.
//   - the Crockford decode aliases. A student reading a code aloud writes O for
//     0, and I or L for 1. Since those letters are never generated, mapping them
//     onto the digits they were meant to be can only ever turn a near-miss into a
//     hit.
//
// Every other character — including a completely wrong code — is dropped. The
// result may be empty or shorter than ReferralCodeLength, and the caller's lookup
// will simply not match. Silently dropping is correct here: a code that survives
// normalisation but does not match any row is the same outcome as one that was
// dropped, and a rejected-code error message would leak whether a code exists.
func NormalizeReferralCode(raw string) string {
	var b strings.Builder
	b.Grow(len(raw))
	for _, r := range strings.ToUpper(raw) {
		switch r {
		case 'O':
			// Crockford decode alias: O reads as zero.
			r = '0'
		case 'I', 'L':
			// Crockford decode alias: I and L read as one.
			r = '1'
		}
		if strings.IndexRune(referralCodeAlphabet, r) < 0 {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// ReferralCodeHasCanonicalLength reports whether a normalised code is the right
// length. It is a cheap pre-filter, NOT a validation: a code of the wrong length
// cannot match any stored code, so there is no point going to the database, but
// saying so is not the same as saying the code is unknown, and the caller must not
// turn it into an error the client can read.
func ReferralCodeHasCanonicalLength(code string) bool {
	return len(code) == ReferralCodeLength
}
