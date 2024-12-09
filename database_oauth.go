package main

import (
	"context"
	"database/sql"
)

func (db *DB) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	user := &User{}
	err := db.GetContext(ctx, user, `
		SELECT id, email, name, organization_id, role, permissions, created_at
		FROM users WHERE email = $1
	`, email)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (db *DB) CreateOrganizationWithOwner(ctx context.Context, org *Organization, owner *User) error {
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Create organization
	_, err = tx.ExecContext(ctx, `
		INSERT INTO organizations (id, name, owner_id, subscription_tier, max_sub_accounts)
		VALUES ($1, $2, $3, $4, $5)
	`, org.ID, org.Name, org.OwnerID, org.SubscriptionTier, org.MaxSubAccounts)
	if err != nil {
		return err
	}

	// Create owner
	_, err = tx.ExecContext(ctx, `
		INSERT INTO users (id, email, name, organization_id, role, permissions)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, owner.ID, owner.Email, owner.Name, owner.OrganizationID, owner.Role, owner.Permissions)
	if err != nil {
		return err
	}

	return tx.Commit()
}
