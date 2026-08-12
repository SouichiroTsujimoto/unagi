package engagement

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/uptrace/bun"

	"github.com/SouichiroTsujimoto/unagi/internal/feature/article"
)

var (
	ErrNotFound      = errors.New("engagement not found")
	ErrInvalidInput  = errors.New("invalid input")
	ErrLimitExceeded = errors.New("limit exceeded")
	ErrLoginRequired = errors.New("login required")
)

const (
	KindEmoji  = "emoji"
	KindAvatar = "avatar"

	CommentStatusVisible = "visible"
	CommentStatusHidden  = "hidden"

	MaxStickersPerArticle        = 200
	MaxStickersPerVisitorArticle = 10
	MaxCommentLength             = 1000
	MaxCommentsPerArticle        = 500

	LoginPath = "/auth/x/login"
)

// AllowedEmoji are the stickers anonymous visitors may place.
var AllowedEmoji = []string{
	"👍", "🎉", "👀", "🎈",
	"❤️", "🍣", "🍵", "🍩",
	"🐟", "🧧", "👑",
}

var allowedEmojiSet = func() map[string]struct{} {
	out := make(map[string]struct{}, len(AllowedEmoji))
	for _, emoji := range AllowedEmoji {
		out[emoji] = struct{}{}
	}
	return out
}()

// Engagement is the public engagement store for stickers and comments.
type Engagement struct {
	db       *bun.DB
	articles *article.Articles
}

func New(db *bun.DB, articles *article.Articles) *Engagement {
	return &Engagement{db: db, articles: articles}
}

// Sticker is a placed sticker on an article board.
type Sticker struct {
	ID          int64     `json:"id"`
	Kind        string    `json:"kind"`
	Value       string    `json:"value"`
	X           float64   `json:"x"`
	Y           float64   `json:"y"`
	Username    string    `json:"username,omitempty"`
	DisplayName string    `json:"displayName,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

// Comment is a flat public comment.
type Comment struct {
	ID          int64     `json:"id"`
	Body        string    `json:"body"`
	Username    string    `json:"username"`
	DisplayName string    `json:"displayName"`
	AvatarURL   string    `json:"avatarUrl"`
	CreatedAt   time.Time `json:"createdAt"`
}

// Viewer is the signed-in X account shown in engagement UI.
type Viewer struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	AvatarURL   string `json:"avatarUrl"`
}

// Snapshot is the public engagement payload for an article.
type Snapshot struct {
	Stickers      []Sticker `json:"stickers"`
	Comments      []Comment `json:"comments"`
	AllowedEmoji  []string  `json:"allowedEmoji"`
	LoginPath     string    `json:"loginPath"`
	Authenticated bool      `json:"authenticated"`
	Viewer        *Viewer   `json:"viewer,omitempty"`
}

// AddEmojiInput places an anonymous emoji sticker.
type AddEmojiInput struct {
	Emoji       string
	X           float64
	Y           float64
	VisitorHash []byte
}

type dbSticker struct {
	bun.BaseModel `bun:"table:article_stickers,alias:s"`

	ID          int64     `bun:",pk,autoincrement"`
	ArticleID   int64     `bun:",notnull"`
	Kind        string    `bun:",notnull"`
	Value       string    `bun:",notnull"`
	X           float64   `bun:",notnull"`
	Y           float64   `bun:",notnull"`
	VisitorHash []byte    `bun:"visitor_hash"`
	XUserID     *string   `bun:"x_user_id"`
	Username    string    `bun:",notnull"`
	DisplayName string    `bun:",notnull"`
	CreatedAt   time.Time `bun:",notnull"`
}

type dbComment struct {
	bun.BaseModel `bun:"table:article_comments,alias:c"`

	ID          int64     `bun:",pk,autoincrement"`
	ArticleID   int64     `bun:",notnull"`
	Body        string    `bun:",notnull"`
	Status      string    `bun:",notnull"`
	XUserID     *string   `bun:"x_user_id"`
	Username    string    `bun:",notnull"`
	DisplayName string    `bun:",notnull"`
	AvatarURL   string    `bun:"avatar_url,notnull"`
	CreatedAt   time.Time `bun:",notnull"`
	UpdatedAt   time.Time `bun:",notnull"`
}

// HashVisitor returns a stable hash for a visitor cookie value.
func HashVisitor(raw string) []byte {
	sum := sha256.Sum256([]byte(raw))
	return sum[:]
}

func normalizeEmoji(emoji string) (string, error) {
	emoji = strings.TrimSpace(emoji)
	if emoji == "" {
		return "", fmt.Errorf("%w: emoji required", ErrInvalidInput)
	}
	if _, ok := allowedEmojiSet[emoji]; !ok {
		return "", fmt.Errorf("%w: emoji not allowed", ErrInvalidInput)
	}
	return emoji, nil
}

func normalizeCoord(v float64, name string) (float64, error) {
	if v < 0 || v > 1 || v != v {
		return 0, fmt.Errorf("%w: %s out of range", ErrInvalidInput, name)
	}
	return v, nil
}

func normalizeCommentBody(body string) (string, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return "", fmt.Errorf("%w: body required", ErrInvalidInput)
	}
	if utf8.RuneCountInString(body) > MaxCommentLength {
		return "", fmt.Errorf("%w: body too long", ErrInvalidInput)
	}
	return body, nil
}
