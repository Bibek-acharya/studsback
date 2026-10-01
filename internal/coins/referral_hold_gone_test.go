package coins

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"strings"
	"testing"
)

// referralMechanicFiles are the three files that implement the referral mechanic, and
// the only three an assertion about "nothing reserves any more" needs to look at.
//
// NOT the whole package: Ledger.Reserve and Ledger.ReleaseReserved are general, tested
// primitives that a future mechanic may use, and they are deliberately untouched. An
// assertion over the whole package would fail for the wrong reason and would push
// somebody towards deleting working ledger code.
var referralMechanicFiles = []string{
	"referral.go",
	"referral_qualification.go",
	"referral_api.go",
}

// TheHoldModelIsGone is the drift guard for the model change.
//
// §3.3 said a referral award is RESERVED FROM THE REFERRER for seven days and then
// released. That model was rejected, because Reserve refuses a hold larger than the
// referrer's balance — so the mechanic refused to pay a student who had no coins, which
// is precisely the population an earn mechanic exists for. Awarding from the faucet is
// the change, and this test asserts nothing in the mechanic has quietly put it back.
//
// IT IS A SOURCE SCAN rather than a behavioural test, and the cost is worth naming: a
// scan cannot prove no code holds anything, only that nobody wrote the calls. What it can
// do is fail the moment somebody re-adds a Reserve to the referral path — which is
// exactly the edit this change is trying to prevent, because re-adding it looks like a
// bug fix ("this referral has no hold") rather than the regression it would be.
//
// The behavioural halves are elsewhere:
// TestSettlementPaysAReferralThatHasNoHoldAtAll pays a holdless, balance-less referrer,
// and TestAReferralPastItsWindowWithBothConditionsSettles asserts that zero
// REFERRAL_HOLD journals exist afterwards.
//
// The needles are matched against IDENTIFIERS ONLY, never against the source text. The
// file headers discuss the retired model at length — including naming the old functions —
// and a test that matched raw text would forbid the code from explaining itself.
func TestTheHoldModelIsGone(t *testing.T) {
	// (identifier, what its presence in the mechanic would mean)
	forbidden := []struct{ ident, why string }{
		{
			ident: "Reserve",
			why: "the referral path places a hold again. A hold requires the referrer to " +
				"already hold coins, so this reintroduces the bug that ended the model: a " +
				"student with a zero balance cannot be paid for referring anybody",
		},
		{
			ident: "ReleaseReserved",
			why:   "the referral path releases a hold again. Nothing is reserved, so there is nothing to release",
		},
		{
			ident: "FindOpenHold",
			why: "the referral path looks for an open hold. A settlement that depends on " +
				"finding one refuses when there is none, which is the old model",
		},
		{
			ident: "ApplyReserved",
			why:   "the referral path moves reserved balances. Nothing about a referral is reserved any more",
		},
		{
			ident: "holdMetadata",
			why:   "the referral path reads a hold's metadata. There are no holds to read",
		},
		{
			ident: "releaseAndGrant",
			why: "the release-and-grant pair is gone. grantInTx replaced it — still one " +
				"transaction, still on the caller's handle",
		},
		{
			ident: "hold_journal_id",
			why: "the settlement writes the hold's journal id. Nothing is held, so there " +
				"is no journal to record and the column stays NULL",
		},
	}

	for _, file := range referralMechanicFiles {
		idents := identifiersIn(t, file)
		for _, f := range forbidden {
			if idents[f.ident] {
				t.Errorf("%s references %s. %s", file, f.ident, f.why)
			}
		}
	}
}

// identifiersIn lists every IDENTIFIER the file's code mentions, and nothing else.
//
// ast.Inspect yields Ident nodes for identifiers in code and for nothing in comments — a
// comment is a token, not an expression — which is the whole reason this assertion can
// discuss the retired model in prose without tripping over itself.
//
// Parsed per file rather than once so a parse failure names the file, which a test that
// had silently degraded to a substring search would not.
func identifiersIn(t *testing.T, file string) map[string]bool {
	t.Helper()
	parsed, err := parser.ParseFile(token.NewFileSet(), file,
		mustRead(t, file), parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %s: %v", file, err)
	}
	out := map[string]bool{}
	ast.Inspect(parsed, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok {
			out[ident.Name] = true
		}
		return true
	})
	return out
}

// The other half of the same guard: SettleReferral must still be ONE transaction.
//
// Rewriting the model is exactly when this is most likely to be lost. A grant now looks
// like a single call, so calling Ledger.Grant directly would appear to be a
// simplification rather than the reopening of the two-transaction window ledger.go
// records and 2d6c2aa closed on purpose.
//
// Asserted structurally rather than left to review: exactly one InUserTx in the function,
// and the grant driven on that handle rather than through the exported Ledger.Grant.
func TestSettlementIsStillOneTransactionOnTheCallersHandle(t *testing.T) {
	parsed, err := parser.ParseFile(token.NewFileSet(), "referral.go",
		mustRead(t, "referral.go"), parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse referral.go: %v", err)
	}

	var settle *ast.FuncDecl
	for _, decl := range parsed.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "SettleReferral" {
			settle = fn
			break
		}
	}
	if settle == nil {
		t.Fatal("could not find SettleReferral in referral.go; this test cannot guard the " +
			"single-transaction property and must be updated rather than skipped")
	}

	transactions, grantsThroughLedger, grantsThroughHelper := 0, false, false
	ast.Inspect(settle, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		switch sel.Sel.Name {
		case "InUserTx":
			transactions++
		case "Grant":
			// Ledger.Grant opens its own transaction. Inside the outer one that is a
			// SAVEPOINT, so it would still commit atomically — which is exactly why the
			// test asserts the SHAPE rather than relying on behaviour: the code would be
			// doing something it does not look like it is doing, and the next reader
			// would trust the shape.
			grantsThroughLedger = true
		case "grantInTx":
			grantsThroughHelper = true
		}
		return true
	})

	if transactions != 1 {
		t.Errorf("SettleReferral opens %d transactions, want exactly 1. The claim, the cap "+
			"slot and the money must be one COMMIT, or a crash between them either pays "+
			"twice or loses the referral", transactions)
	}
	if grantsThroughLedger {
		t.Error("SettleReferral calls an exported Grant rather than driving the grant on " +
			"its own transaction handle. grantInTx is the body of both and exists so the " +
			"transaction structure is visible in this file rather than spread across two")
	}
	if !grantsThroughHelper {
		t.Error("SettleReferral does not call grantInTx; the grant must run on the caller's " +
			"handle so it commits with the claim and the state write")
	}
}

// hold_journal_id is permanently NULL, and the column is KEPT.
//
// A dead column is worth a test rather than a comment, because the comment ages and the
// test fails. Keeping it rather than dropping it is a decision, and this is what stops it
// being reversed by nobody noticing. Whether the write still happens is covered by
// TestTheHoldModelIsGone — a column can only be written by naming it in SQL.
func TestHoldJournalIDIsKeptAsAPermanentlyNullAuditColumn(t *testing.T) {
	field, ok := reflect.TypeOf(UserReferral{}).FieldByName("HoldJournalID")
	if !ok {
		t.Fatal("UserReferral has no HoldJournalID column. It is kept deliberately — a " +
			"nullable audit column costs nothing and dropping it destroys any history from " +
			"a deployment that ran the reserved-hold model. referral_model.go says so, and " +
			"this test is what stops the decision being reversed by accident")
	}
	if field.Type.Kind() != reflect.Ptr {
		t.Errorf("HoldJournalID is %v, want a pointer so the column can be NULL", field.Type)
	}
}

// ReasonReferralHold is now unreferenced by the mechanic, and that is a report to be made
// rather than a thing to delete.
//
// The constant stays. It is a ledger reason code, holdReasons still declares it, and
// Ledger.Reserve still refuses any other reason — removing it would change what the
// ledger accepts, for the sake of a note in a commit message, and Reserve and
// ReleaseReserved are general primitives a future mechanic may well use.
//
// What this test does is make the situation exact and keep it that way: the reason code
// is still a hold reason, and no file implementing the mechanic names it in code.
func TestReasonReferralHoldIsUnreferencedByTheMechanicAndStillDeclared(t *testing.T) {
	if !IsHoldReason(ReasonReferralHold) {
		t.Errorf("%s is no longer a hold reason. It is retained deliberately, so if it is "+
			"being removed then Reserve's reason vocabulary changes with it and that needs "+
			"its own decision", ReasonReferralHold)
	}
	for _, file := range referralMechanicFiles {
		if identifiersIn(t, file)[ReasonReferralHold] {
			t.Errorf("%s's code references %s. Nothing is reserved, so the constant is kept "+
				"for the ledger's own vocabulary and must not be used by the mechanic",
				file, ReasonReferralHold)
		}
	}
}

// The phone-verification port is one narrow question, and it has NO IMPLEMENTATION in
// this codebase — which is the finding, stated as an assertion rather than only in prose.
//
// main.go wires `nil` for coins.PhoneVerification. That is not an oversight: it is the
// correct posture given that auth.User records no verification state at all, and this
// test is what would fail if somebody "fixed" it with a proxy.
//
// A proxy is the specific failure worth guarding, because it looks like a fix and it
// removes the only thing standing between a reward and a typed phone number. Asserting
// the port has ONE method is what makes "just check they entered a phone" an edit
// somebody has to make deliberately rather than a helper they add.
func TestThePhoneVerificationPortIsOneNarrowQuestionWithNoImplementation(t *testing.T) {
	port := reflect.TypeOf((*PhoneVerification)(nil)).Elem()
	if port.Kind() != reflect.Interface {
		t.Fatalf("PhoneVerification is a %v, want an interface", port.Kind())
	}
	if port.NumMethod() != 1 {
		t.Errorf("PhoneVerification has %d methods, want 1. A wider port is an invitation "+
			"to add a 'has a phone number' variant, and that proxy is exactly what must "+
			"not exist: §5.2's rule is that the invitee VERIFIED a phone, not that they "+
			"typed one into a profile form", port.NumMethod())
	}
	method, ok := port.MethodByName("PhoneVerified")
	if !ok {
		t.Fatal("PhoneVerification has no PhoneVerified method; the test cannot guard the " +
			"port's shape and must be updated rather than skipped")
	}
	if method.Type.NumIn() != 2 ||
		!strings.Contains(method.Type.In(0).String(), "Context") ||
		method.Type.In(1).String() != "uint" {
		t.Errorf("PhoneVerified takes %v, want (context.Context, uint)", method.Type)
	}
	if method.Type.NumOut() != 2 ||
		method.Type.Out(0).Kind().String() != "bool" ||
		method.Type.Out(1).String() != "error" {
		t.Errorf("PhoneVerified returns (%s, %s), want (bool, error)",
			method.Type.Out(0), method.Type.Out(1))
	}
}

// mustRead reads a file from this package's directory.
//
// `go test` runs with the package's source directory as the working directory, so this is
// a same-directory read rather than a path relative to the repository root.
func mustRead(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return raw
}
