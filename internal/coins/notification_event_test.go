// internal/coins/notification_event_test.go
//
// The coins.debited contract, with no database.
//
// Two things are asserted here, and both are contracts rather than behaviour:
//
//   - that the event is in the CLOSED registry. ValidateRegistry is
//     logger.Fatal in cmd/server/main.go, so a constant without a row is a
//     server that does not boot, and a row whose template does not parse is the
//     same failure. This is the test that keeps the two halves together.
//   - that the templates obey 09-support-copy-cheat-sheet.md. "free", a currency
//     amount and the Income Tax Act windfall-gain vocabulary are a legal
//     exposure, and a template is just a string nobody re-reads, so the ban is
//     asserted here rather than left to review.
//
// The emission itself — that an unlock really writes a row, and that a failed
// write takes the purchase down with it — needs a real transaction and is in
// service_notification_test.go.
package coins

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"studsphere/backend/internal/notification"
)

// ── the registry row ─────────────────────────────────────────────────────────

// The event exists, the row exists, and the whole registry still validates.
func TestCoinsDebitedIsRegisteredAndTheRegistryValidates(t *testing.T) {
	if err := notification.ValidateRegistry(); err != nil {
		t.Fatalf("registry invalid: %v", err)
	}
	def, ok := notification.Registry[notification.EventCoinsDebited]
	if !ok {
		t.Fatalf("%s has no Registry row; ValidateRegistry would not catch it and boot would fail",
			notification.EventCoinsDebited)
	}
	if def.Key != notification.EventCoinsDebited {
		t.Errorf("row key = %q, want %q", def.Key, notification.EventCoinsDebited)
	}
	// 03-api-contract.md §5: explicit user, not transactional. Transactional
	// events are skipped by NotifyTx, so getting it wrong silently emits nothing.
	if def.RecipientKind != notification.RecipientExplicit {
		t.Errorf("recipient kind = %q, want %q", def.RecipientKind, notification.RecipientExplicit)
	}
	if def.Transactional {
		t.Error("coins.debited is Transactional; NotifyTx skips transactional events and a debit would never be announced")
	}
	// 09 "If the student says an email never arrived" lists this event as one that
	// emails. A missing receipt for a real debit is the question that list exists
	// to answer.
	if !def.EmailDefault {
		t.Error("email default is off; 09 lists coins.debited among the events that email a student")
	}
	// PriorityNormal, not critical: it confirms something the student just did on
	// purpose, and critical would force in_app on the way.
	if def.Priority != notification.PriorityNormal {
		t.Errorf("priority = %q, want %q", def.Priority, notification.PriorityNormal)
	}
}

// The emitted Data renders both templates. missingkey=error means a key the
// template names but the emit does not supply is a runtime failure in production
// and an empty body here, so it is asserted the way internal/forum does it.
func TestCoinsDebitedTemplatesRenderFromTheEmittedData(t *testing.T) {
	req := unlockNotificationRequest(4242, UnlockEvent{
		JournalID:    "11111111-1111-4111-8111-111111111111",
		ResourceType: ResourceTypeStudyResource,
		ResourceID:   812,
		CoinsPaid:    40,
		BalanceAfter: 40,
		Source:       UnlockSourceCoins,
	})
	if req.EventKey != notification.EventCoinsDebited {
		t.Fatalf("event key = %q, want %q", req.EventKey, notification.EventCoinsDebited)
	}
	if len(req.Recipients) != 1 || req.Recipients[0] != (notification.Ref{Type: "user", ID: 4242}) {
		t.Fatalf("recipients = %+v, want the unlocking student alone", req.Recipients)
	}
	// The occurrence key is the journal: one purchase, one announcement, and a
	// retried emit has an identity instead of writing a second row.
	journal, _ := req.Data["journal"].(string)
	if want := "coins.debited:" + journal; req.OccurrenceKey != want {
		t.Errorf("occurrence key = %q, want %q", req.OccurrenceKey, want)
	}
	if req.CorrelationID != journal {
		t.Errorf("correlation id = %q, want the journal id %q", req.CorrelationID, journal)
	}

	def := notification.Registry[notification.EventCoinsDebited]
	title, err := notification.ResolveTemplate(def.TitleTpl, req.Data)
	if err != nil {
		t.Fatalf("title template: %v", err)
	}
	body, err := notification.ResolveTemplate(def.BodyTpl, req.Data)
	if err != nil {
		t.Fatalf("body template: %v", err)
	}
	if title != "StudsTokens used" {
		t.Errorf("title = %q, want the copy deck's %q", title, "StudsTokens used")
	}
	// The coin figures, and only coin figures.
	if !strings.Contains(body, "40") || !strings.Contains(body, "study resource") {
		t.Errorf("body = %q, want the coins spent and the class unlocked", body)
	}
	if strings.Contains(body, "{{") || strings.Contains(body, "<no value>") {
		t.Errorf("body = %q; a template hole survived the render", body)
	}
}

// Every class renders, because the label is derived rather than carried and a
// class this function does not know would fall through to the document wording.
func TestCoinsDebitedNamesEveryUnlockableClass(t *testing.T) {
	want := map[string]string{
		ResourceTypeStudyResource: "study resource",
		ResourceTypeVideo:         "video lecture",
		ResourceTypeMockTest:      "mock test",
	}
	for _, class := range ResourceTypes {
		req := unlockNotificationRequest(1, UnlockEvent{
			JournalID:    "22222222-2222-4222-8222-222222222222",
			ResourceType: class,
			CoinsPaid:    10,
			BalanceAfter: 5,
		})
		def := notification.Registry[notification.EventCoinsDebited]
		if _, err := notification.ResolveTemplate(def.TitleTpl, req.Data); err != nil {
			t.Errorf("%s title: %v", class, err)
		}
		body, err := notification.ResolveTemplate(def.BodyTpl, req.Data)
		if err != nil {
			t.Errorf("%s body: %v", class, err)
			continue
		}
		if expected, ok := want[class]; !ok || !strings.Contains(body, expected) {
			t.Errorf("%s body = %q, want it to name %q", class, body, expected)
		}
	}
}

// 09-support-copy-cheat-sheet.md: three statutory bans and a set of house rules.
// A template is a string nobody re-reads, so the ban is a test.
func TestCoinsDebitedCopyObeysTheSupportCopyRules(t *testing.T) {
	def := notification.Registry[notification.EventCoinsDebited]
	for _, resourceType := range ResourceTypes {
		req := unlockNotificationRequest(1, UnlockEvent{
			JournalID:    "33333333-3333-4333-8333-333333333333",
			ResourceType: resourceType,
			CoinsPaid:    40,
			BalanceAfter: 0,
		})
		rendered, err := notification.ResolveTemplate(def.TitleTpl, req.Data)
		if err != nil {
			t.Fatalf("%s title: %v", resourceType, err)
		}
		body, err := notification.ResolveTemplate(def.BodyTpl, req.Data)
		if err != nil {
			t.Fatalf("%s body: %v", resourceType, err)
		}
		haystack := strings.ToLower(rendered + " " + body)
		// "free" is CPA 2075 s.16(2)(c)(3); a currency figure beside a coin
		// figure is a false value claim; and prize/award/win/raffle/draw are the
		// exact words Income Tax Act 2058 s.5/88A uses to define a windfall gain.
		// Substring matches, not words: "draw" inside "withdrawn" is the same trap.
		for _, banned := range []string{
			"free", "no cost", "prize", "award", "win", "winner", "raffle", "draw",
			"baksis", "jitauri", "cash out", "redeem for money", "refundable",
			"hurry", "limited time", "unlock more with", "npr", "inr", "₹", "rs.",
		} {
			if strings.Contains(haystack, banned) {
				t.Errorf("%s copy contains the banned %q: %q", resourceType, banned, rendered+" "+body)
			}
		}
		if strings.Contains(haystack, "you will lose") {
			t.Errorf("%s copy threatens the student: %q", resourceType, body)
		}
	}
}

// ── the typed insufficiency ──────────────────────────────────────────────────

// ErrInsufficient is a refinement of the sentinel, not a replacement: every
// errors.Is(err, ErrInsufficientCoins) in the status mapping has to keep
// matching, and the two figures have to arrive with it.
func TestErrInsufficientUnwrapsToTheSentinelAndCarriesTheFigures(t *testing.T) {
	err := ErrInsufficient(40, 25)
	if !errors.Is(err, ErrInsufficientCoins) {
		t.Fatalf("errors.Is(ErrInsufficient(...), ErrInsufficientCoins) = false; every 402 mapping depends on it")
	}
	// The message is unchanged, so a log line and a grep of the old string both
	// still work.
	if want := "insufficient coins: need 40, 25 available"; err.Error() != want {
		t.Errorf("message = %q, want %q", err.Error(), want)
	}
	required, available, ok := InsufficientFigures(err)
	if !ok {
		t.Fatal("InsufficientFigures reported no figures on an error built by ErrInsufficient")
	}
	if required != 40 || available != 25 {
		t.Errorf("figures = %d required / %d available, want 40 / 25", required, available)
	}
	// It survives being wrapped by a caller on the way up.
	wrapped := fmt.Errorf("purchase failed: %w", err)
	if _, _, ok := InsufficientFigures(wrapped); !ok {
		t.Error("a wrapped ErrInsufficient lost its figures")
	}
	// A bare sentinel reports no figures rather than inventing two.
	if _, _, ok := InsufficientFigures(ErrInsufficientCoins); ok {
		t.Error("the bare sentinel reported figures it does not have")
	}
	var typed *InsufficientError
	if !errors.As(err, &typed) {
		t.Fatal("errors.As did not reach the typed error")
	}
	if typed.Shortfall() != 15 {
		t.Errorf("shortfall = %d, want 15", typed.Shortfall())
	}
	// Floored, because a concurrent grant between the config read and the refusal
	// must not produce a negative gap in a body the UI renders.
	if got := (&InsufficientError{Required: 40, Available: 60}).Shortfall(); got != 0 {
		t.Errorf("shortfall with 60 available = %d, want 0", got)
	}
}

// ── the second read is gone ──────────────────────────────────────────────────

// panicBalanceWallet panics if anybody asks it for a balance. It is the whole
// proof: insufficientCoinsPayload used to open a WalletBalance read here, and a
// test that merely counts queries would have to keep counting.
type panicBalanceWallet struct{ *fakeWallet }

func (p panicBalanceWallet) WalletBalance(context.Context, uint, time.Time) (*WalletBalance, error) {
	panic("insufficientCoinsPayload read the wallet: the figures travel on the error")
}

func TestInsufficientCoinsPayloadNeverReadsTheWallet(t *testing.T) {
	soonest := time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)
	wallet := &fakeWallet{available: 25, soonest: &soonest}
	api := testAPI(t, &fakeEntitlements{}, wallet, nil)
	// A reader that refuses to answer, on the reader's own field.
	api.wallet = panicBalanceWallet{wallet}
	cfg, _ := api.config.Load()

	// required/available are the two figures the refusal was decided on.
	payload := api.insufficientCoinsPayload(context.Background(), 1, 40, 25, cfg)
	if payload.Required != 40 || payload.Available != 25 || payload.Shortfall != 15 {
		t.Errorf("payload = %d / %d / %d, want 40 / 25 / 15", payload.Required, payload.Available, payload.Shortfall)
	}
	// The rest of the body is still read, because it is about the wallet as it is
	// now: the two eligibility questions and the expiry are separate reads and
	// they stay.
	if payload.ExpiresInDays == nil {
		t.Error("expires_in_days is null; the soonest-expiry read is not the one that was removed")
	}
}

// The same claim at the HTTP boundary, counted rather than asserted: the whole
// 402 path — entitlement, allowance, the refused purchase and the payload — must
// not have asked for a balance at all.
func TestTheInsufficientPathReadsTheWalletNoTimes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	entitlements := &fakeEntitlements{allowanceErr: fmt.Errorf("%w: all used", ErrNoAllowanceRemaining)}
	wallet := &fakeWallet{available: 25}
	api := testAPI(t, entitlements, wallet, nil)
	RegisterWalletRoutes(router, stubAuth(4242), api)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/coins/unlock",
		strings.NewReader(`{"resource_type":"study_resource","resource_id":812}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "no-read-1")
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusPaymentRequired {
		t.Fatalf("status = %d, want 402 (%s)", recorder.Code, recorder.Body.String())
	}
	for _, call := range wallet.log.calls {
		if call == "wallet_balance" {
			t.Fatalf("the 402 path read the wallet; calls = %v", wallet.log.calls)
		}
	}
	// And the body still quotes the refusal, not a later reading of it.
	if !strings.Contains(recorder.Body.String(), `"available":25`) {
		t.Errorf("the 402 body lost the carried figure: %s", recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"shortfall":15`) {
		t.Errorf("the 402 body lost the shortfall: %s", recorder.Body.String())
	}
}
