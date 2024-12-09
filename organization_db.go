package main

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

var (
	ErrOrganizationNotFound = errors.New("organization not found")
	ErrUserNotFound        = errors.New("user not found")
	ErrEmailTaken         = errors.New("email already taken")
	ErrMaxSubAccounts     = errors.New("maximum sub-accounts reached")
)

// CreateOrganization creates a new organization and its owner
func (db *DB) CreateOrganization(ctx context.Context, name, ownerEmail, ownerName string) (*Organization, error) {
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Check if email is already taken
	var count int
	err = tx.GetContext(ctx, &count, "SELECT COUNT(*) FROM users WHERE email = $1", ownerEmail)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, ErrEmailTaken
	}

	org := &Organization{
		ID:              uuid.New(),
		Name:            name,
		SubscriptionTier: "free",
		MaxSubAccounts:  5,
	}

	// Create organization
	_, err = tx.ExecContext(ctx, `
		INSERT INTO organizations (id, name, owner_id, subscription_tier, max_sub_accounts)
		VALUES ($1, $2, $3, $4, $5)
	`, org.ID, org.Name, org.OwnerID, org.SubscriptionTier, org.MaxSubAccounts)
	if err != nil {
		return nil, err
	}

	// Create owner user
	owner := &User{
		ID:             uuid.New(),
		Email:          ownerEmail,
		Name:           ownerName,
		OrganizationID: org.ID,
		Role:           "owner",
		Permissions:    Permissions{"admin": true},
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO users (id, email, name, organization_id, role, permissions)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, owner.ID, owner.Email, owner.Name, owner.OrganizationID, owner.Role, owner.Permissions)
	if err != nil {
		return nil, err
	}

	// Update organization with owner ID
	_, err = tx.ExecContext(ctx, `
		UPDATE organizations SET owner_id = $1 WHERE id = $2
	`, owner.ID, org.ID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return org, nil
}

// GetOrganization retrieves an organization by ID
func (db *DB) GetOrganization(ctx context.Context, id uuid.UUID) (*Organization, error) {
	org := &Organization{}
	err := db.GetContext(ctx, org, `
		SELECT id, name, owner_id, subscription_tier, max_sub_accounts, created_at
		FROM organizations WHERE id = $1
	`, id)
	if err != nil {
		return nil, err
	}
	return org, nil
}

// GetOrganizationUsers retrieves all users in an organization
func (db *DB) GetOrganizationUsers(ctx context.Context, orgID uuid.UUID) ([]User, error) {
	var users []User
	err := db.SelectContext(ctx, &users, `
		SELECT id, email, name, organization_id, role, permissions, created_at
		FROM users WHERE organization_id = $1
	`, orgID)
	if err != nil {
		return nil, err
	}
	return users, nil
}

// AddUserToOrganization adds a new user to an organization
func (db *DB) AddUserToOrganization(ctx context.Context, orgID uuid.UUID, email, name string) (*User, error) {
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Check if email is already taken
	var count int
	err = tx.GetContext(ctx, &count, "SELECT COUNT(*) FROM users WHERE email = $1", email)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, ErrEmailTaken
	}

	// Check number of existing sub-accounts
	err = tx.GetContext(ctx, &count, `
		SELECT COUNT(*) FROM users
		WHERE organization_id = $1 AND role = 'sub_account'
	`, orgID)
	if err != nil {
		return nil, err
	}

	var maxSubAccounts int
	err = tx.GetContext(ctx, &maxSubAccounts, `
		SELECT max_sub_accounts FROM organizations WHERE id = $1
	`, orgID)
	if err != nil {
		return nil, err
	}

	if count >= maxSubAccounts {
		return nil, ErrMaxSubAccounts
	}

	user := &User{
		ID:             uuid.New(),
		Email:          email,
		Name:           name,
		OrganizationID: orgID,
		Role:           "sub_account",
		Permissions:    Permissions{},
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO users (id, email, name, organization_id, role, permissions)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, user.ID, user.Email, user.Name, user.OrganizationID, user.Role, user.Permissions)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return user, nil
}
