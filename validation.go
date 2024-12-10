package main

import (
	"errors"
	"fmt"
	"net/mail"
	"unicode/utf8"

	"github.com/google/uuid"
)

var (
	ErrInvalidEmail      = errors.New("invalid email format")
	ErrInvalidUUID       = errors.New("invalid UUID format")
	ErrEmptyField        = errors.New("required field is empty")
	ErrFieldTooLong      = errors.New("field exceeds maximum length")
	ErrRequestBodyTooBig = errors.New("request body too large")
)

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

const (
	MaxNameLength        = 255
	MaxEmailLength      = 255
	MaxRequestBodyBytes = 1 * 1024 * 1024 // 1MB
)

// ValidateEmail checks if an email address is valid
func ValidateEmail(email string) error {
	if email == "" {
		return &ValidationError{Field: "email", Message: ErrEmptyField.Error()}
	}

	if len(email) > MaxEmailLength {
		return &ValidationError{Field: "email", Message: ErrFieldTooLong.Error()}
	}

	if _, err := mail.ParseAddress(email); err != nil {
		return &ValidationError{Field: "email", Message: ErrInvalidEmail.Error()}
	}

	return nil
}

// ValidateUUID checks if a string is a valid UUID
func ValidateUUID(id string) error {
	if id == "" {
		return &ValidationError{Field: "id", Message: ErrEmptyField.Error()}
	}

	if _, err := uuid.Parse(id); err != nil {
		return &ValidationError{Field: "id", Message: ErrInvalidUUID.Error()}
	}

	return nil
}

// ValidateName checks if a name field is valid
func ValidateName(name string) error {
	if name == "" {
		return &ValidationError{Field: "name", Message: ErrEmptyField.Error()}
	}

	if utf8.RuneCountInString(name) > MaxNameLength {
		return &ValidationError{Field: "name", Message: ErrFieldTooLong.Error()}
	}

	return nil
}

// ValidateCreateOrganizationRequest validates the create organization request
func ValidateCreateOrganizationRequest(req *CreateOrganizationRequest) error {
	if err := ValidateName(req.Name); err != nil {
		return err
	}

	if err := ValidateEmail(req.OwnerEmail); err != nil {
		return err
	}

	if err := ValidateName(req.OwnerName); err != nil {
		return err
	}

	return nil
}

// ValidateAddUserRequest validates the add user request
func ValidateAddUserRequest(req *AddUserRequest) error {
	if err := ValidateEmail(req.Email); err != nil {
		return err
	}

	if err := ValidateName(req.Name); err != nil {
		return err
	}

	return nil
}
