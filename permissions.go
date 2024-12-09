package main

import (
	"errors"
)

var (
	ErrUnauthorized     = errors.New("unauthorized")
	ErrInsufficientRole = errors.New("insufficient role")
)

// Permission represents a single permission
type Permission string

// Define our permissions
const (
	PermCreateOrg       Permission = "create:org"
	PermReadOrg        Permission = "read:org"
	PermUpdateOrg      Permission = "update:org"
	PermDeleteOrg      Permission = "delete:org"
	PermInviteUser     Permission = "invite:user"
	PermRemoveUser     Permission = "remove:user"
	PermUpdateUser     Permission = "update:user"
	PermManageSettings Permission = "manage:settings"
)

// RolePermissions defines what permissions each role has
var RolePermissions = map[string][]Permission{
	"owner": {
		PermCreateOrg,
		PermReadOrg,
		PermUpdateOrg,
		PermDeleteOrg,
		PermInviteUser,
		PermRemoveUser,
		PermUpdateUser,
		PermManageSettings,
	},
	"admin": {
		PermReadOrg,
		PermUpdateOrg,
		PermInviteUser,
		PermRemoveUser,
		PermUpdateUser,
		PermManageSettings,
	},
	"sub_account": {
		PermReadOrg,
	},
}

// HasPermission checks if a user has a specific permission
func (u *User) HasPermission(perm Permission) bool {
	// Check role-based permissions
	if perms, ok := RolePermissions[u.Role]; ok {
		for _, p := range perms {
			if p == perm {
				return true
			}
		}
	}

	// Check user-specific permissions
	return u.Permissions[string(perm)]
}

// HasAnyPermission checks if a user has any of the given permissions
func (u *User) HasAnyPermission(perms ...Permission) bool {
	for _, perm := range perms {
		if u.HasPermission(perm) {
			return true
		}
	}
	return false
}

// HasAllPermissions checks if a user has all of the given permissions
func (u *User) HasAllPermissions(perms ...Permission) bool {
	for _, perm := range perms {
		if !u.HasPermission(perm) {
			return false
		}
	}
	return true
}
