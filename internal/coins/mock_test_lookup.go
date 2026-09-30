// internal/coins/mock_test_lookup.go
//
// The ResourceLookup seam, answered for the mock-test class.
//
// ── why the port is EXTENDED rather than special-cased ──────────────────────
//
// ResourceLookup is a two-method port and a single field on UnlockAPI. The
// obvious shortcut for a second domain would be to add a `mockTests` field and a
// class switch inside lookupUnlockable, so the wallet's one call site grows a
// branch that knows which module owns which table. That is the coupling this seam
// exists to avoid: unlock_api.go can read a paper's title and nothing about a
// paper, and the moment it knows that mock_test is special it will be one edit
// away from knowing that study_resource is too.
//
// So the extension is at the IMPLEMENTATION end. This file adds
//
//   - mockTestLookup — a ResourceLookup that owns exactly the mock_test class, in
//     the same shape as studyResourceLookup, and
//   - NewResourceLookups — a composite ResourceLookup that fans a question out to
//     several per-module lookups and returns the first that can answer it.
//
// WithResourceLookup is unchanged, main.go passes one composite where it used to
// pass one lookup, and internal/coins gains no knowledge of which module owns
// which table beyond the adapters it already contains. The next domain — press
// media, download centre — is one more adapter in this list and no other edit.
//
// ── why the import points this way, again ───────────────────────────────────
//
// Same reason as study_resource_lookup.go and for the same cycle: the gate lives in
// the module that owns the bytes and the ledger lives here, so one edge has to be
// coins -> the other module and the other direction is expressed as a port. The
// alternative — an adapter in main.go, the profileCompletionAdapter pattern — is
// right when the CONSUMER of the answer lives in another module, and wrong here for
// the same reason it was wrong for study resources: the mock-tests table is data
// this package must read on the write path (the 404 check, the receipt title, the
// history line), and hiding that read in a file with no tests and no comment
// explaining it does not remove the coupling, it only moves it somewhere less
// legible.
//
// ── what "unlockable" means for a paper ──────────────────────────────────────
//
// A row is unlockable when it exists and is published. There is no type to
// confuse it with — a mock test is a mock test — and no soft-delete escape
// hatch to normalise: FindTestByID goes through GORM's DeletedAt scoping, so a
// deleted row is already invisible here.
package coins

import (
	"context"
	"fmt"
	"strconv"

	"studsphere/backend/internal/mocktests"
)

// mockTestLookup answers LookupUnlockable from the mock-tests table.
type mockTestLookup struct {
	tests *mocktests.Service
}

// NewMockTestLookup wires the lookup for the mock_test class.
//
// A nil service resolves nothing, which is the same answer as no lookup wired at
// all — main.go's wiring order is not something the wallet should be able to crash
// on.
func NewMockTestLookup(tests *mocktests.Service) ResourceLookup {
	return &mockTestLookup{tests: tests}
}

// LookupUnlockable resolves a published mock test, or returns an error matching
// ErrNotFound — which is what the gate and the unlock endpoint map to 404.
//
// The three refusals are indistinguishable on purpose, for the reason
// study_resource_lookup.go gives: a soft-deleted row, a draft and an unknown id
// all answer "not found", because §2.3 asks for one status and a lookup that told
// them apart would let the endpoint confirm that a draft exists.
func (l *mockTestLookup) LookupUnlockable(_ context.Context, resourceType string, resourceID uint64) (UnlockableResource, error) {
	// A class this adapter does not own is a 404 and not a lookup failure. It is
	// checked BEFORE the nil service so an unwired composite is still a valid
	// answer for the classes its other adapters own.
	if resourceType != ResourceTypeMockTest {
		return UnlockableResource{}, fmt.Errorf("%w: %s is not served by internal/mocktests", ErrNotFound, resourceType)
	}
	if l == nil || l.tests == nil {
		return UnlockableResource{}, fmt.Errorf("%w: the mock test lookup is not wired", ErrNotFound)
	}
	// A uint64 that does not fit the module's uint id is not a row that does not
	// exist yet; it is a value the module cannot address, and converting it would
	// wrap onto some other paper's id.
	if resourceID == 0 || resourceID > uint64(^uint(0)) {
		return UnlockableResource{}, fmt.Errorf("%w: mock test %s is not an id this module can hold",
			ErrNotFound, strconv.FormatUint(resourceID, 10))
	}

	test, err := l.tests.GetPublishedTest(uint(resourceID))
	if err != nil {
		return UnlockableResource{}, fmt.Errorf("%w: mock test %s is missing or unpublished",
			ErrNotFound, strconv.FormatUint(resourceID, 10))
	}
	// The title is not decoration: it is what puts "Organic Chemistry Mock 2081"
	// in the debit receipt and the wallet history instead of "mock_test 812". A
	// lookup that cannot produce one may return an empty Title — every caller
	// falls back to the class words rather than failing an unlock over a label.
	return UnlockableResource{Title: test.Title}, nil
}

// ── the composite ────────────────────────────────────────────────────────────

// ClassLookup binds a ResourceLookup to the classes it answers for.
//
// The classes and the lookup are ONE fact, and that is why they are declared
// together: registering a lookup without saying what it answers is how a paper ends
// up validated against the study-resources table, which is the exact mistake this
// composite exists to make impossible. Split into two calls they would be back
// where a merge could separate them.
type ClassLookup struct {
	// Classes is the closed set of coin classes this lookup can resolve.
	Classes []string
	// Lookup is the per-module adapter.
	Lookup ResourceLookup
}

// resourceLookups is the composite: a class-indexed fan-out behind one
// ResourceLookup, so unlock_api.go keeps a single field and no knowledge of which
// module owns which table.
type resourceLookups struct {
	// byClass is class -> lookup, built once at wiring time so the hot path is a
	// map read rather than a scan.
	byClass map[string]ResourceLookup
}

// NewResourceLookups composes per-class lookups into the single ResourceLookup the
// wallet asks.
//
// It routes BY CLASS rather than trying every lookup in turn, and that is not an
// optimisation. A fan-out would make the answer depend on which module happened to
// be asked first, and would run a query against a table the lookup does not own
// before returning the same ErrNotFound it would have returned without asking.
//
// The first entry registered for a class wins. A duplicate is a wiring bug, and
// silently preferring either side of it would hide the bug until a resource had
// been priced off the wrong table. No entry for a class yields ErrNotFound, which
// is the same answer as an unwired lookup and keeps main.go's wiring order from
// being able to crash the wallet.
func NewResourceLookups(entries ...ClassLookup) ResourceLookup {
	composite := &resourceLookups{byClass: make(map[string]ResourceLookup, len(entries))}
	for _, entry := range entries {
		if entry.Lookup == nil {
			continue
		}
		for _, class := range entry.Classes {
			if _, claimed := composite.byClass[class]; claimed {
				continue
			}
			composite.byClass[class] = entry.Lookup
		}
	}
	return composite
}

// LookupUnlockable routes to the lookup registered for the class.
//
// A lookup that owns the class but cannot resolve the resource answers ErrNotFound
// and the composite stops there rather than asking another module about the same
// id: the owning module is the one that knows whether the row exists, and a second
// opinion from a table that does not hold this resource could only turn a
// not-found into a differently-worded not-found.
func (c *resourceLookups) LookupUnlockable(ctx context.Context, resourceType string, resourceID uint64) (UnlockableResource, error) {
	lookup, ok := c.byClass[resourceType]
	if !ok {
		return UnlockableResource{}, fmt.Errorf("%w: %s is not served by any wired lookup", ErrNotFound, resourceType)
	}
	return lookup.LookupUnlockable(ctx, resourceType, resourceID)
}
