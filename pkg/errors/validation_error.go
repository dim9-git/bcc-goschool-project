package pkgerrors

import (
	"errors"
	"fmt"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation"
)

type ErrorEntity struct {
	Name   string
	Reason string
}

type ValidationError struct {
	Errors []ErrorEntity
}

func (e *ValidationError) Error() string {
	if len(e.Errors) == 0 {
		return "Validation error"
	}

	errors := make([]string, 0, len(e.Errors))
	for _, err := range e.Errors {
		errors = append(errors, fmt.Sprintf("%s: %s", err.Name, err.Reason))
	}
	return fmt.Sprintf("Validation error: %s", strings.Join(errors, ", "))
}

func (e *ValidationError) Unwrap() []error {
	errors := make([]error, 0, len(e.Errors))

	for _, err := range e.Errors {
		errors = append(errors, fmt.Errorf("%s: %s", err.Name, err.Reason))
	}

	return errors
}

func NewValidationErrorFromOzzo(errs validation.Errors) *ValidationError {
	ve := &ValidationError{
		Errors: make([]ErrorEntity, 0, len(errs)),
	}

	if errs == nil {
		return ve
	}

	ve.parseValidationErrors(errs)
	return ve
}

func (ve *ValidationError) parseValidationErrors(errs validation.Errors) {
	for field, fieldErr := range errs {

		var validationErrs validation.Errors
		switch {
		case errors.As(fieldErr, &validationErrs):
			ve.parseValidationErrors(validationErrs)
		default:
			ve.Errors = append(ve.Errors, ErrorEntity{
				Name:   field,
				Reason: fieldErr.Error(),
			})
		}
	}
}
