package review

import (
	"errors"
	"strings"

	"github.com/go-playground/validator/v10"
)

func humanizeValidationError(err error) string {
	if err == nil {
		return ""
	}

	var validationErrors validator.ValidationErrors
	if !errors.As(err, &validationErrors) || len(validationErrors) == 0 {
		return err.Error()
	}

	fieldError := validationErrors[0]
	field := validationFieldLabel(fieldError.Field())
	switch fieldError.Tag() {
	case "required":
		return field + " is required"
	case "min":
		if fieldError.Field() == "Rating" {
			return "Rating must be between 1 and 5"
		}
		return field + " must be at least " + fieldError.Param() + " characters"
	case "max":
		if fieldError.Field() == "Rating" {
			return "Rating must be between 1 and 5"
		}
		return field + " must be at most " + fieldError.Param() + " characters"
	case "oneof":
		return field + " must be one of " + strings.ReplaceAll(fieldError.Param(), " ", ", ")
	default:
		return "Invalid value for " + field
	}
}

func validationFieldLabel(field string) string {
	switch field {
	case "UniversityID":
		return "University ID"
	default:
		return field
	}
}
