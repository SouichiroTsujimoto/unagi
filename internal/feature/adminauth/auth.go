package adminauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/uptrace/bun"
)

var (
	ErrUnauthorized     = errors.New("unauthorized")
	ErrInvalidToken     = errors.New("invalid token")
	ErrBootstrapClosed  = errors.New("bootstrap closed")
	ErrLastCredential   = errors.New("cannot delete last credential")
	ErrNotConfigured    = errors.New("webauthn not configured")
	ErrChallengeExpired = errors.New("challenge expired")
	ErrRecoveryInvalid  = errors.New("recovery code invalid")
)

const (
	PurposeRegistration = "registration"
	PurposeLogin        = "login"
	CookieName          = "unagi_admin_session"
	challengeTTL        = 5 * time.Minute
)

type Config struct {
	RPDisplayName      string
	RPID               string
	RPOrigins          []string
	BootstrapTokenHash []byte
	SessionTTL         time.Duration
	SecureCookies      bool
}

type Auth struct {
	db     *bun.DB
	wa     *webauthn.WebAuthn
	config Config
}

type adminUser struct {
	id          []byte
	name        string
	displayName string
	creds       []webauthn.Credential
}

func (u *adminUser) WebAuthnID() []byte                         { return u.id }
func (u *adminUser) WebAuthnName() string                        { return u.name }
func (u *adminUser) WebAuthnDisplayName() string                 { return u.displayName }
func (u *adminUser) WebAuthnCredentials() []webauthn.Credential { return u.creds }

type Credential struct {
	bun.BaseModel `bun:"table:admin_credentials,alias:c"`

	ID              int64      `bun:",pk,autoincrement" json:"id"`
	CredentialID    []byte     `bun:",notnull,unique" json:"-"`
	PublicKey       []byte     `bun:",notnull" json:"-"`
	AttestationType string     `bun:",notnull" json:"-"`
	Transport       string     `bun:",notnull" json:"-"`
	SignCount       uint32     `bun:",notnull" json:"-"`
	BackupEligible  bool       `bun:",notnull" json:"-"`
	BackupState     bool       `bun:",notnull" json:"-"`
	AAGUID          []byte     `json:"-"`
	DisplayName     string     `bun:",notnull" json:"displayName"`
	CreatedAt       time.Time  `bun:",notnull" json:"createdAt"`
	LastUsedAt      *time.Time `json:"lastUsedAt,omitempty"`
}

type challengeRow struct {
	bun.BaseModel `bun:"table:webauthn_challenges,alias:ch"`

	ID          int64     `bun:",pk,autoincrement"`
	Challenge   string    `bun:",notnull,unique"`
	SessionData string    `bun:",notnull"`
	Purpose     string    `bun:",notnull"`
	ExpiresAt   time.Time `bun:",notnull"`
	CreatedAt   time.Time `bun:",notnull"`
}

type Session struct {
	bun.BaseModel `bun:"table:admin_sessions,alias:s"`

	ID         int64     `bun:",pk,autoincrement"`
	TokenHash  []byte    `bun:",notnull,unique"`
	CSRFToken  string    `bun:",notnull"`
	ExpiresAt  time.Time `bun:",notnull"`
	CreatedAt  time.Time `bun:",notnull"`
	LastSeenAt time.Time `bun:",notnull"`
}

type recoveryRow struct {
	bun.BaseModel `bun:"table:recovery_codes,alias:rc"`

	ID        int64      `bun:",pk,autoincrement"`
	CodeHash  []byte     `bun:",notnull,unique"`
	UsedAt    *time.Time
	CreatedAt time.Time `bun:",notnull"`
}

func New(db *bun.DB, cfg Config) (*Auth, error) {
	if strings.TrimSpace(cfg.RPID) == "" || len(cfg.RPOrigins) == 0 {
		return nil, ErrNotConfigured
	}
	if cfg.RPDisplayName == "" {
		cfg.RPDisplayName = "unagi"
	}
	if cfg.SessionTTL <= 0 {
		cfg.SessionTTL = 7 * 24 * time.Hour
	}
	wa, err := webauthn.New(&webauthn.Config{
		RPDisplayName: cfg.RPDisplayName,
		RPID:          cfg.RPID,
		RPOrigins:     cfg.RPOrigins,
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			ResidentKey:        protocol.ResidentKeyRequirementRequired,
			RequireResidentKey: protocol.ResidentKeyRequired(),
			UserVerification:   protocol.VerificationRequired,
		},
		AttestationPreference: protocol.PreferNoAttestation,
	})
	if err != nil {
		return nil, fmt.Errorf("webauthn: %w", err)
	}
	return &Auth{db: db, wa: wa, config: cfg}, nil
}

func (a *Auth) SecureCookies() bool         { return a.config.SecureCookies }
func (a *Auth) SessionTTL() time.Duration   { return a.config.SessionTTL }
func (a *Auth) Origins() []string           { return append([]string(nil), a.config.RPOrigins...) }

func (a *Auth) CredentialCount(ctx context.Context) (int, error) {
	return a.db.NewSelect().Model((*Credential)(nil)).Count(ctx)
}

func (a *Auth) NeedsBootstrap(ctx context.Context) (bool, error) {
	n, err := a.CredentialCount(ctx)
	if err != nil {
		return false, err
	}
	return n == 0 && len(a.config.BootstrapTokenHash) > 0, nil
}

func (a *Auth) VerifyBootstrapToken(raw string) error {
	if len(a.config.BootstrapTokenHash) == 0 {
		return ErrBootstrapClosed
	}
	sum := sha256.Sum256([]byte(strings.TrimSpace(raw)))
	if subtleConstantTimeCompare(sum[:], a.config.BootstrapTokenHash) {
		return nil
	}
	return ErrInvalidToken
}

func (a *Auth) ListCredentials(ctx context.Context) ([]Credential, error) {
	var rows []Credential
	err := a.db.NewSelect().Model(&rows).OrderExpr("id ASC").Scan(ctx)
	return rows, err
}

func (a *Auth) DeleteCredential(ctx context.Context, id int64) error {
	n, err := a.CredentialCount(ctx)
	if err != nil {
		return err
	}
	if n <= 1 {
		return ErrLastCredential
	}
	res, err := a.db.NewDelete().Model((*Credential)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return ErrUnauthorized
	}
	return nil
}

func (a *Auth) loadUser(ctx context.Context) (*adminUser, error) {
	rows, err := a.ListCredentials(ctx)
	if err != nil {
		return nil, err
	}
	u := &adminUser{
		id:          []byte("unagi-admin"),
		name:        "admin",
		displayName: "Admin",
		creds:       make([]webauthn.Credential, 0, len(rows)),
	}
	for _, row := range rows {
		u.creds = append(u.creds, toWebAuthnCredential(row))
	}
	return u, nil
}

func toWebAuthnCredential(row Credential) webauthn.Credential {
	var transports []protocol.AuthenticatorTransport
	if row.Transport != "" {
		for _, t := range strings.Split(row.Transport, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				transports = append(transports, protocol.AuthenticatorTransport(t))
			}
		}
	}
	return webauthn.Credential{
		ID:              row.CredentialID,
		PublicKey:       row.PublicKey,
		AttestationType: row.AttestationType,
		Transport:       transports,
		Flags: webauthn.CredentialFlags{
			BackupEligible: row.BackupEligible,
			BackupState:    row.BackupState,
		},
		Authenticator: webauthn.Authenticator{
			AAGUID:    row.AAGUID,
			SignCount: row.SignCount,
		},
	}
}

func (a *Auth) storeChallenge(ctx context.Context, purpose string, session *webauthn.SessionData) error {
	raw, err := json.Marshal(session)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	row := &challengeRow{
		Challenge:   session.Challenge,
		SessionData: string(raw),
		Purpose:     purpose,
		ExpiresAt:   now.Add(challengeTTL),
		CreatedAt:   now,
	}
	_, err = a.db.NewInsert().Model(row).Exec(ctx)
	return err
}

func (a *Auth) takeChallenge(ctx context.Context, purpose, challenge string) (*webauthn.SessionData, error) {
	challenge = strings.TrimRight(challenge, "=")
	var row challengeRow
	err := a.db.NewSelect().
		Model(&row).
		Where("challenge = ? AND purpose = ?", challenge, purpose).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrChallengeExpired
	}
	if err != nil {
		return nil, err
	}
	_, _ = a.db.NewDelete().Model((*challengeRow)(nil)).Where("id = ?", row.ID).Exec(ctx)
	if time.Now().UTC().After(row.ExpiresAt) {
		return nil, ErrChallengeExpired
	}
	var session webauthn.SessionData
	if err := json.Unmarshal([]byte(row.SessionData), &session); err != nil {
		return nil, err
	}
	return &session, nil
}

func (a *Auth) BeginRegistration(ctx context.Context) (*protocol.CredentialCreation, error) {
	user, err := a.loadUser(ctx)
	if err != nil {
		return nil, err
	}
	exclude := make([]protocol.CredentialDescriptor, 0, len(user.creds))
	for _, c := range user.creds {
		exclude = append(exclude, c.Descriptor())
	}
	options, session, err := a.wa.BeginRegistration(user, webauthn.WithExclusions(exclude))
	if err != nil {
		return nil, err
	}
	if err := a.storeChallenge(ctx, PurposeRegistration, session); err != nil {
		return nil, err
	}
	return options, nil
}

func (a *Auth) FinishRegistration(ctx context.Context, body io.Reader, displayName string) (Credential, error) {
	user, err := a.loadUser(ctx)
	if err != nil {
		return Credential{}, err
	}
	parsed, err := protocol.ParseCredentialCreationResponseBody(body)
	if err != nil {
		return Credential{}, err
	}
	session, err := a.takeChallenge(ctx, PurposeRegistration, parsed.Response.CollectedClientData.Challenge)
	if err != nil {
		return Credential{}, err
	}
	cred, err := a.wa.CreateCredential(user, *session, parsed)
	if err != nil {
		return Credential{}, err
	}
	now := time.Now().UTC()
	row := Credential{
		CredentialID:    cred.ID,
		PublicKey:       cred.PublicKey,
		AttestationType: cred.AttestationType,
		Transport:       joinTransports(cred.Transport),
		SignCount:       cred.Authenticator.SignCount,
		BackupEligible:  cred.Flags.BackupEligible,
		BackupState:     cred.Flags.BackupState,
		AAGUID:          cred.Authenticator.AAGUID,
		DisplayName:     strings.TrimSpace(displayName),
		CreatedAt:       now,
	}
	if row.DisplayName == "" {
		row.DisplayName = "Passkey"
	}
	if _, err := a.db.NewInsert().Model(&row).Exec(ctx); err != nil {
		return Credential{}, err
	}
	return row, nil
}

func (a *Auth) BeginLogin(ctx context.Context) (*protocol.CredentialAssertion, error) {
	n, err := a.CredentialCount(ctx)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, ErrUnauthorized
	}
	options, session, err := a.wa.BeginDiscoverableLogin()
	if err != nil {
		return nil, err
	}
	if err := a.storeChallenge(ctx, PurposeLogin, session); err != nil {
		return nil, err
	}
	return options, nil
}

func (a *Auth) FinishLogin(ctx context.Context, body io.Reader) (Session, string, error) {
	parsed, err := protocol.ParseCredentialRequestResponseBody(body)
	if err != nil {
		return Session{}, "", err
	}
	session, err := a.takeChallenge(ctx, PurposeLogin, parsed.Response.CollectedClientData.Challenge)
	if err != nil {
		return Session{}, "", err
	}
	user, err := a.loadUser(ctx)
	if err != nil {
		return Session{}, "", err
	}
	handler := func(rawID, userHandle []byte) (webauthn.User, error) {
		return user, nil
	}
	cred, err := a.wa.ValidateDiscoverableLogin(handler, *session, parsed)
	if err != nil {
		return Session{}, "", err
	}
	now := time.Now().UTC()
	_, _ = a.db.NewUpdate().
		Model((*Credential)(nil)).
		Set("sign_count = ?", cred.Authenticator.SignCount).
		Set("last_used_at = ?", now).
		Set("backup_state = ?", cred.Flags.BackupState).
		Where("credential_id = ?", cred.ID).
		Exec(ctx)
	return a.CreateSession(ctx)
}

func (a *Auth) CreateSession(ctx context.Context) (Session, string, error) {
	return a.createSession(ctx)
}

func (a *Auth) createSession(ctx context.Context) (Session, string, error) {
	raw, err := GenerateToken(32)
	if err != nil {
		return Session{}, "", err
	}
	csrf, err := GenerateToken(24)
	if err != nil {
		return Session{}, "", err
	}
	now := time.Now().UTC()
	row := Session{
		TokenHash:  HashToken(raw),
		CSRFToken:  csrf,
		ExpiresAt:  now.Add(a.config.SessionTTL),
		CreatedAt:  now,
		LastSeenAt: now,
	}
	if _, err := a.db.NewInsert().Model(&row).Exec(ctx); err != nil {
		return Session{}, "", err
	}
	return row, raw, nil
}

func (a *Auth) LookupSession(ctx context.Context, rawToken string) (Session, error) {
	if rawToken == "" {
		return Session{}, ErrUnauthorized
	}
	var row Session
	err := a.db.NewSelect().
		Model(&row).
		Where("token_hash = ?", HashToken(rawToken)).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrUnauthorized
	}
	if err != nil {
		return Session{}, err
	}
	now := time.Now().UTC()
	if now.After(row.ExpiresAt) {
		_, _ = a.db.NewDelete().Model((*Session)(nil)).Where("id = ?", row.ID).Exec(ctx)
		return Session{}, ErrUnauthorized
	}
	row.LastSeenAt = now
	_, _ = a.db.NewUpdate().Model(&row).Column("last_seen_at").WherePK().Exec(ctx)
	return row, nil
}

func (a *Auth) DestroySession(ctx context.Context, rawToken string) error {
	_, err := a.db.NewDelete().Model((*Session)(nil)).Where("token_hash = ?", HashToken(rawToken)).Exec(ctx)
	return err
}

func (a *Auth) IssueRecoveryCodes(ctx context.Context, n int) ([]string, error) {
	if n <= 0 {
		n = 8
	}
	_, _ = a.db.NewDelete().Model((*recoveryRow)(nil)).Where("used_at IS NULL").Exec(ctx)
	codes := make([]string, 0, n)
	now := time.Now().UTC()
	for i := 0; i < n; i++ {
		code, err := GenerateToken(10)
		if err != nil {
			return nil, err
		}
		row := &recoveryRow{CodeHash: HashToken(code), CreatedAt: now}
		if _, err := a.db.NewInsert().Model(row).Exec(ctx); err != nil {
			return nil, err
		}
		codes = append(codes, code)
	}
	return codes, nil
}

func (a *Auth) ConsumeRecoveryCode(ctx context.Context, code string) error {
	var row recoveryRow
	err := a.db.NewSelect().
		Model(&row).
		Where("code_hash = ? AND used_at IS NULL", HashToken(strings.TrimSpace(code))).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrRecoveryInvalid
	}
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	row.UsedAt = &now
	_, err = a.db.NewUpdate().Model(&row).Column("used_at").WherePK().Exec(ctx)
	return err
}

func (a *Auth) ValidOrigin(origin string) bool {
	for _, o := range a.config.RPOrigins {
		if o == origin {
			return true
		}
	}
	return false
}

func joinTransports(ts []protocol.AuthenticatorTransport) string {
	parts := make([]string, 0, len(ts))
	for _, t := range ts {
		parts = append(parts, string(t))
	}
	return strings.Join(parts, ",")
}

func HashToken(raw string) []byte {
	sum := sha256.Sum256([]byte(raw))
	return sum[:]
}

func GenerateToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func subtleConstantTimeCompare(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := range a {
		v |= a[i] ^ b[i]
	}
	return v == 0
}
