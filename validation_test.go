package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidation(t *testing.T) {
	t.Run("Email Validation", func(t *testing.T) {
		tests := []struct {
			name    string
			email   string
			wantErr bool
		}{
			{
				name:    "Valid email",
				email:   "test@example.com",
				wantErr: false,
			},
			{
				name:    "Empty email",
				email:   "",
				wantErr: true,
			},
			{
				name:    "Invalid email format",
				email:   "not-an-email",
				wantErr: true,
			},
			{
				name:    "Email too long",
				email:   strings.Repeat("a", MaxEmailLength+1) + "@example.com",
				wantErr: true,
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				err := ValidateEmail(tc.email)
				if tc.wantErr {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}
			})
		}
	})

	t.Run("Name Validation", func(t *testing.T) {
		tests := []struct {
			name    string
			input   string
			wantErr bool
		}{
			{
				name:    "Valid name",
				input:   "John Doe",
				wantErr: false,
			},
			{
				name:    "Empty name",
				input:   "",
				wantErr: true,
			},
			{
				name:    "Name too long",
				input:   strings.Repeat("a", MaxNameLength+1),
				wantErr: true,
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				err := ValidateName(tc.input)
				if tc.wantErr {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}
			})
		}
	})

	t.Run("UUID Validation", func(t *testing.T) {
		tests := []struct {
			name    string
			uuid    string
			wantErr bool
		}{
			{
				name:    "Valid UUID",
				uuid:    "123e4567-e89b-12d3-a456-426614174000",
				wantErr: false,
			},
			{
				name:    "Empty UUID",
				uuid:    "",
				wantErr: true,
			},
			{
				name:    "Invalid UUID format",
				uuid:    "not-a-uuid",
				wantErr: true,
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				err := ValidateUUID(tc.uuid)
				if tc.wantErr {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}
			})
		}
	})
}
