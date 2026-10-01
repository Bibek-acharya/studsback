package institution

import (
	"strconv"
	"testing"

	"studsphere/backend/internal/system"

	"gorm.io/gorm"
)

// Shared by inquiry_scope_test.go. Kept apart so the test file above reads as the
// table of cases and this reads as the fixtures.

func u64(v uint) string { return strconv.FormatUint(uint64(v), 10) }

// The reads go back through the system repository over the SAME gorm handle the
// handler used, not through a second one — a second handle to the same in-memory
// database would not see the handler's writes at all, and every assertion here
// would be checking its own seed.
func inquiryStatus(t *testing.T, db *gorm.DB, id uint) string {
	t.Helper()
	row, err := system.NewRepository(db).FindContactInquiryByID(id)
	if err != nil {
		t.Fatalf("read inquiry %d: %v", id, err)
	}
	return row.Status
}

func inquiryExists(t *testing.T, db *gorm.DB, id uint) bool {
	t.Helper()
	_, err := system.NewRepository(db).FindContactInquiryByID(id)
	return err == nil
}
