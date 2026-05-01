package scholarship

import (
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"
)

type scholarshipResolverRepoStub struct {
	localByProviderID   map[uint]*Scholarship
	providerByID        map[uint]*ProviderScholarship
	createdScholarships []*Scholarship
}

func (s *scholarshipResolverRepoStub) FindByProviderScholarshipID(providerScholarshipID uint) (*Scholarship, error) {
	if scholarship, ok := s.localByProviderID[providerScholarshipID]; ok {
		return scholarship, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (s *scholarshipResolverRepoStub) FindProviderScholarshipByID(id uint) (*ProviderScholarship, error) {
	if scholarship, ok := s.providerByID[id]; ok {
		return scholarship, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (s *scholarshipResolverRepoStub) FindByID(id uint) (*Scholarship, error) {
	return nil, gorm.ErrRecordNotFound
}

func (s *scholarshipResolverRepoStub) Create(scholarship *Scholarship) error {
	if scholarship.ID == 0 {
		scholarship.ID = uint(len(s.createdScholarships) + 1)
	}
	s.createdScholarships = append(s.createdScholarships, scholarship)
	s.localByProviderID[*scholarship.ProviderScholarshipID] = scholarship
	return nil
}

func TestResolveScholarshipForApplication_CreatesLocalScholarshipFromProviderRow(t *testing.T) {
	repo := &scholarshipResolverRepoStub{
		localByProviderID: map[uint]*Scholarship{},
		providerByID: map[uint]*ProviderScholarship{
			2: {
				ID:              2,
				Title:           "Provider scholarship",
				Description:     "Fallback source row",
				Location:        "Kathmandu",
				Value:           "Full tuition",
				Deadline:        time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
				DegreeLevel:     "Bachelor",
				FundingType:     "Full",
				ScholarshipType: "Merit",
				Status:          "published",
			},
		},
	}

	scholarship, err := resolveScholarshipForApplication(repo, 10002)
	if err != nil {
		t.Fatalf("resolveScholarshipForApplication() error = %v", err)
	}

	if got := len(repo.createdScholarships); got != 1 {
		t.Fatalf("createdScholarships = %d, want 1", got)
	}

	if scholarship.ID == 0 {
		t.Fatal("resolved scholarship should have a local ID")
	}

	if scholarship.ProviderScholarshipID == nil {
		t.Fatal("resolved scholarship should record provider scholarship ID")
	}

	if got, want := *scholarship.ProviderScholarshipID, uint(2); got != want {
		t.Fatalf("ProviderScholarshipID = %d, want %d", got, want)
	}

	if got, want := scholarship.Title, "Provider scholarship"; got != want {
		t.Fatalf("Title = %q, want %q", got, want)
	}
}

func TestResolveScholarshipForApplication_ReturnsLocalScholarshipWhenMapped(t *testing.T) {
	providerID := uint(7)
	repo := &scholarshipResolverRepoStub{
		localByProviderID: map[uint]*Scholarship{
			providerID: {
				ID:                    42,
				Title:                 "Local scholarship",
				ProviderScholarshipID: &providerID,
			},
		},
		providerByID: map[uint]*ProviderScholarship{},
	}

	scholarship, err := resolveScholarshipForApplication(repo, 10007)
	if err != nil {
		t.Fatalf("resolveScholarshipForApplication() error = %v", err)
	}

	if scholarship.ID != 42 {
		t.Fatalf("ID = %d, want 42", scholarship.ID)
	}

	if len(repo.createdScholarships) != 0 {
		t.Fatalf("createdScholarships = %d, want 0", len(repo.createdScholarships))
	}
}

func TestResolveScholarshipForApplication_ReturnsNotFoundForUnknownProviderScholarship(t *testing.T) {
	repo := &scholarshipResolverRepoStub{
		localByProviderID: map[uint]*Scholarship{},
		providerByID:      map[uint]*ProviderScholarship{},
	}

	_, err := resolveScholarshipForApplication(repo, 10002)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("error = %v, want record not found", err)
	}
}
