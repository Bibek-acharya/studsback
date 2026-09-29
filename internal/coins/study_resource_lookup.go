// internal/coins/study_resource_lookup.go
//
// The ResourceLookup seam, answered for the two classes that live in
// internal/studyresources.
//
// ── why the dependency points this way ───────────────────────────────────────
//
// This file is the only place internal/coins reaches into internal/studyresources,
// and that direction is chosen rather than accidental. The cycle is the whole
// problem: the download gate lives in studyresources (it is the only thing that
// can decide to hand over a file) and the ledger lives in coins (it is the only
// thing that can spend), so if the gate needed the ledger and the ledger needed
// the resource table, the two packages would import each other and Go would
// refuse to build.
//
// Two directions were available.
//
//  1. coins → studyresources, which is what this file does. The lookup is a
//     small adapter: one SELECT of one table, reached through a narrow interface
//     rather than the whole service. studyresources learns nothing about coins.
//
//  2. studyresources → coins, with the lookup declared in coins and implemented
//     in main.go the way profileCompletionAdapter already is. That is the right
//     pattern when the CONSUMER of an answer lives in another module — and it is
//     what the wallet's profile lookup does. It is wrong here for one specific
//     reason: the study-resource table is not a peer module's business, it is
//     data this package must read on the write path (the 404 check, the receipt
//     title, the history line). Hiding that read behind an adapter in main.go
//     would not remove the coupling, it would only move it to a file that has no
//     tests and no comment explaining it.
//
// So: one edge, coins → studyresources, through this file, and the reverse edge
// does not exist because the gate calls coins through an interface that
// studyresources declares itself (see its DownloadGate). Both directions are
// ports; neither is a cycle.
//
// ── what "unlockable" means here ─────────────────────────────────────────────
//
// A row is unlockable when it exists, is published, and is of a class this
// lookup owns. A draft is NOT: 03-api-contract.md §2.3 defines the 404 as
// "unknown id, or not published, or wrong type" precisely so that the unlock
// endpoint cannot be used to confirm the existence of something the public
// cannot see. GetPublishedResource already encodes that rule in one place, and
// this adapter reuses it rather than restating the is_published test — a second
// copy of "what counts as published" is a second thing to forget when the rule
// changes.
package coins

import (
	"context"
	"fmt"
	"strconv"

	"studsphere/backend/internal/studyresources"
)

// studyResourceLookup answers LookupUnlockable from the study-resources table.
//
// The field is the module's SERVICE rather than its repository for one reason:
// GetPublishedResource is the module's own definition of "a row the public may
// see", and going through it keeps the publication rule in one file.
type studyResourceLookup struct {
	resources *studyresources.Service
}

// NewStudyResourceLookup wires the lookup for the study_resource and video
// classes.
//
// studyResource is the *studyresources.Service. A nil service is a lookup that
// resolves nothing, which is the same answer as no lookup wired at all — it is
// not an error, because main.go wiring order is not something the wallet should
// be able to crash on.
func NewStudyResourceLookup(resources *studyresources.Service) ResourceLookup {
	return &studyResourceLookup{resources: resources}
}

// LookupUnlockable resolves a study resource, or returns an error matching
// ErrNotFound — which is what the unlock endpoint maps to 404.
//
// The three refusals are deliberately indistinguishable from each other, because
// §2.3 asks for one status: a soft-deleted row, a draft and a row that is a
// video when the caller asked for a document all answer "not found". A lookup
// that told them apart would let the endpoint confirm that id 812 exists and is
// merely a draft, which is the probing the single 404 exists to prevent.
func (l *studyResourceLookup) LookupUnlockable(ctx context.Context, resourceType string, resourceID uint64) (UnlockableResource, error) {
	// A class this adapter does not own is a 404 and not a lookup failure.
	// internal/mocktests, pressmedia and downloadcenter each need their own; that
	// is a later slice, and until one exists an unlock of a mock test must fail
	// rather than be validated against the wrong table.
	if resourceType != ResourceTypeStudyResource && resourceType != ResourceTypeVideo {
		return UnlockableResource{}, fmt.Errorf("%w: %s is not served by internal/studyresources", ErrNotFound, resourceType)
	}
	if l == nil || l.resources == nil {
		return UnlockableResource{}, fmt.Errorf("%w: the study resource lookup is not wired", ErrNotFound)
	}
	// A uint64 that does not fit the module's uint id is not a row that does not
	// exist yet; it is a value the module cannot address, and converting it would
	// wrap onto some other resource's id.
	if resourceID == 0 || resourceID > uint64(^uint(0)) {
		return UnlockableResource{}, fmt.Errorf("%w: study resource %s is not an id this module can hold",
			ErrNotFound, strconv.FormatUint(resourceID, 10))
	}

	resource, err := l.resources.GetPublishedResource(uint(resourceID))
	if err != nil {
		return UnlockableResource{}, fmt.Errorf("%w: study resource %s is missing or unpublished",
			ErrNotFound, strconv.FormatUint(resourceID, 10))
	}
	// The class is checked as well as the publication state. A caller asking for
	// a document and naming a video row is asking for the wrong price, and
	// charging the document price for a video would be a bug the ledger cannot
	// see: the entitlement it writes carries the class the CALLER named, not the
	// one the row actually is.
	if class := studyResourceClass(resource.ResourceType); class != resourceType {
		return UnlockableResource{}, fmt.Errorf("%w: study resource %s is a %s, not a %s",
			ErrNotFound, strconv.FormatUint(resourceID, 10), class, resourceType)
	}
	return UnlockableResource{Title: resource.Title}, nil
}

// studyResourceClass is the coin-economy class a stored resource_type falls into.
//
// A video lecture is priced as a video and draws on the video allowance, so the
// split is not cosmetic: it is the same split the gate and the price table make.
// The document classes are collapsed into study_resource because the economy
// prices all four of them identically, and a legacy row whose type does not
// normalize at all is a document — every type that has ever been storable in
// this table is either a video lecture or one of the four documents.
func studyResourceClass(resourceType string) string {
	if studyresources.IsVideoType(resourceType) {
		return ResourceTypeVideo
	}
	return ResourceTypeStudyResource
}
