package account

import (
	"time"

	"github.com/uptrace/bun"
)

type Account struct {
	bun.BaseModel `bun:"table:accounts,alias:a" json:"-"`

	ID        int64     `bun:",pk,autoincrement" json:"id"`
	Email     string    `bun:",notnull,unique" json:"email"`
	CreatedAt time.Time `bun:",notnull" json:"createdAt"`
}
