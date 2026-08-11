package account

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/uptrace/bun"
)

var (
	ErrEmailRequired = errors.New("email is required")
	ErrEmailInvalid  = errors.New("email is invalid")
	ErrEmailExists   = errors.New("email already exists")
	ErrNotFound      = errors.New("account not found")
)

type Accounts struct {
	db *bun.DB
}

func New(db *bun.DB) *Accounts {
	return &Accounts{db: db}
}

func (accounts *Accounts) List(ctx context.Context) ([]Account, error) {
	var result []Account
	if err := accounts.db.NewSelect().
		Model(&result).
		OrderExpr("created_at DESC").
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	return result, nil
}

func (accounts *Accounts) Create(ctx context.Context, email string) (*Account, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return nil, ErrEmailRequired
	}

	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email {
		return nil, ErrEmailInvalid
	}

	exists, err := accounts.db.NewSelect().
		Model((*Account)(nil)).
		Where("email = ?", email).
		Exists(ctx)
	if err != nil {
		return nil, fmt.Errorf("check account email: %w", err)
	}
	if exists {
		return nil, ErrEmailExists
	}

	account := &Account{
		Email:     email,
		CreatedAt: time.Now().UTC(),
	}
	if _, err := accounts.db.NewInsert().Model(account).Exec(ctx); err != nil {
		return nil, fmt.Errorf("create account: %w", err)
	}
	return account, nil
}

func (accounts *Accounts) Delete(ctx context.Context, id int64) error {
	result, err := accounts.db.NewDelete().
		Model((*Account)(nil)).
		Where("id = ?", id).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete account: %w", err)
	}

	deleted, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read deleted account count: %w", err)
	}
	if deleted == 0 {
		return ErrNotFound
	}
	return nil
}
