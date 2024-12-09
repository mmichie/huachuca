package main

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestPermissions(t *testing.T) {
	orgID := uuid.New()

	tests := []struct {
		name        string
		user        User
		permission  Permission
		shouldHave  bool
	}{
		{
			name: "Owner has create org permission",
			user: User{
				Role:        "owner",
				Permissions: Permissions{},
			},
			permission: PermCreateOrg,
			shouldHave: true,
		},
		{
			name: "Sub account does not have create org permission",
			user: User{
				Role:        "sub_account",
				Permissions: Permissions{},
			},
			permission: PermCreateOrg,
			shouldHave: false,
		},
		{
			name: "Admin has invite user permission",
			user: User{
				Role:        "admin",
				Permissions: Permissions{},
			},
			permission: PermInviteUser,
			shouldHave: true,
		},
		{
			name: "User with specific permission override",
			user: User{
				Role:        "sub_account",
				Permissions: Permissions{"create:org": true},
			},
			permission: PermCreateOrg,
			shouldHave: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.user.ID = uuid.New()
			tc.user.OrganizationID = orgID

			hasPermission := tc.user.HasPermission(tc.permission)
			require.Equal(t, tc.shouldHave, hasPermission)
		})
	}
}
