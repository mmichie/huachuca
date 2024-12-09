package main

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Organization struct {
	ID              uuid.UUID          `db:"id" json:"id"`
	Name            string             `db:"name" json:"name"`
	OwnerID         uuid.UUID          `db:"owner_id" json:"owner_id"`
	SubscriptionTier string            `db:"subscription_tier" json:"subscription_tier"`
	MaxSubAccounts  int               `db:"max_sub_accounts" json:"max_sub_accounts"`
	CreatedAt       time.Time          `db:"created_at" json:"created_at"`
}

type User struct {
	ID             uuid.UUID          `db:"id" json:"id"`
	Email          string             `db:"email" json:"email"`
	Name           string             `db:"name" json:"name"`
	OrganizationID uuid.UUID          `db:"organization_id" json:"organization_id"`
	Role           string             `db:"role" json:"role"`
	Permissions    Permissions        `db:"permissions" json:"permissions"`
	CreatedAt      time.Time          `db:"created_at" json:"created_at"`
}

type Permissions map[string]bool

// Value implements the driver.Valuer interface for Permissions
func (p Permissions) Value() (driver.Value, error) {
	return json.Marshal(p)
}

// Scan implements the sql.Scanner interface for Permissions
func (p *Permissions) Scan(value interface{}) error {
	if value == nil {
		*p = make(Permissions)
		return nil
	}
	return json.Unmarshal(value.([]byte), p)
}
