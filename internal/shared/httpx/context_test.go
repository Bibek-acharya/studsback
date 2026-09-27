package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// contextWith runs fn with a gin context that has had key set to value, or no
// key at all when set is false, mirroring what middleware.Auth produces.
func contextWith(t *testing.T, set bool, value any) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	gc, _ := gin.CreateTestContext(w)
	gc.Request = req
	if set {
		gc.Set("user_id", value)
	}
	return gc
}

func TestCurrentUserIDMissingKey(t *testing.T) {
	userID, ok := CurrentUserID(contextWith(t, false, nil))
	if ok {
		t.Errorf("ok = true, want false when \"user_id\" was never set")
	}
	if userID != 0 {
		t.Errorf("userID = %d, want 0 alongside ok = false", userID)
	}
}

func TestCurrentUserIDTypes(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  uint
	}{
		// uint is the concrete type middleware.Auth stores today:
		// utils.Claims.UserID is a uint.
		{name: "uint", value: uint(11), want: 11},
		// The remaining cases are defensive; no middleware sets these today.
		{name: "uint64", value: uint64(11), want: 11},
		{name: "int", value: int(11), want: 11},
		{name: "int64", value: int64(11), want: 11},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userID, ok := CurrentUserID(contextWith(t, true, tt.value))
			if !ok {
				t.Fatalf("ok = false, want true for a %T value", tt.value)
			}
			if userID != tt.want {
				t.Errorf("userID = %d, want %d for a %T value", userID, tt.want, tt.value)
			}
		})
	}
}

func TestCurrentUserIDWrongTypeIsNotZero(t *testing.T) {
	tests := []struct {
		name  string
		value any
	}{
		{name: "string", value: "11"},
		{name: "float64", value: float64(11)},
		{name: "nil", value: nil},
		{name: "bool", value: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userID, ok := CurrentUserID(contextWith(t, true, tt.value))
			if ok {
				t.Errorf("ok = true for a %T value, want false", tt.value)
			}
			// The whole point of the strict form: a rejected value never
			// comes back as a plausible-looking id.
			if userID != 0 {
				t.Errorf("userID = %d, want 0 for a rejected %T value", userID, tt.value)
			}
		})
	}
}

// TestCurrentUserIDZeroIsResolvedNotRejected pins the distinction the unsafe
// assertion loses: a stored uint(0) is a *resolved* id, reported through ok,
// whereas an absent or mistyped value is not resolvable at all. Collapsing the
// two is what let a bad extraction write uploaded_by = 0 as if it were a user.
func TestCurrentUserIDZeroIsResolvedNotRejected(t *testing.T) {
	userID, ok := CurrentUserID(contextWith(t, true, uint(0)))
	if !ok {
		t.Errorf("ok = false, want true for a stored uint(0): the value is present and of the right type")
	}
	if userID != 0 {
		t.Errorf("userID = %d, want 0", userID)
	}
}
