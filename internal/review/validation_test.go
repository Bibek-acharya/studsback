package review

import (
	"testing"

	"github.com/go-playground/validator/v10"
)

func TestHumanizeValidationError(t *testing.T) {
	validate := validator.New()
	cases := []struct {
		name  string
		input CreateUniversityReviewRequest
		want  string
	}{
		{"pros minimum", CreateUniversityReviewRequest{UniversityID: 1, Rating: 5, Pros: "short", Cons: "valid cons text"}, "Pros must be at least 10 characters"},
		{"cons required", CreateUniversityReviewRequest{UniversityID: 1, Rating: 5, Pros: "valid pros text"}, "Cons is required"},
		{"rating range", CreateUniversityReviewRequest{UniversityID: 1, Rating: 6, Pros: "valid pros text", Cons: "valid cons text"}, "Rating must be between 1 and 5"},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.Struct(tt.input)
			if got := humanizeValidationError(err); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}
