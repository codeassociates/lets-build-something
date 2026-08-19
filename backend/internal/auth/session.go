package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const CookieName = "rental_session"

type Session struct {
	Token     string
	ExpiresAt time.Time
	User      *User
}

func (s *Store) StartSession(ctx context.Context, userID int64, ttl time.Duration) (*Session, error) {
	token, digest, err := newSessionToken()
	if err != nil {
		return nil, err
	}
	expires := time.Now().Add(ttl)
	if _, err := s.pool.Exec(ctx,
		`INSERT INTO sessions (token_hash, user_id, expires_at) VALUES ($1, $2, $3)`,
		digest, userID, expires); err != nil {
		return nil, fmt.Errorf("creating session: %w", err)
	}
	user, err := s.ByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &Session{Token: token, ExpiresAt: expires, User: user}, nil
}

// LookupSession resolves a cookie value to its user, treating expired and
// unknown tokens identically.
func (s *Store) LookupSession(ctx context.Context, token string) (*User, error) {
	if token == "" {
		return nil, nil
	}
	u, err := scanUser(s.pool.QueryRow(ctx, `
		SELECT `+prefixed(userColumns, "u.")+`
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = $1 AND s.expires_at > now() AND u.active`,
		hashToken(token)))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("looking up session: %w", err)
	}
	return u, nil
}

func (s *Store) EndSession(ctx context.Context, token string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, hashToken(token))
	return err
}

// PurgeExpiredSessions is called periodically by the worker.
func (s *Store) PurgeExpiredSessions(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE expires_at < now()`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// SetCookie issues the session cookie. It is HttpOnly so no script can read the
// token, and Lax so ordinary navigation to the site keeps the user signed in.
func SetCookie(w http.ResponseWriter, sess *Session, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    sess.Token,
		Path:     "/",
		Expires:  sess.ExpiresAt,
		MaxAge:   int(time.Until(sess.ExpiresAt).Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func ClearCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func TokenFromRequest(r *http.Request) string {
	if c, err := r.Cookie(CookieName); err == nil {
		return c.Value
	}
	return ""
}

// prefixed qualifies a column list with a table alias for use in joins.
func prefixed(columns, prefix string) string {
	parts := strings.Split(columns, ",")
	for i, col := range parts {
		parts[i] = prefix + strings.TrimSpace(col)
	}
	return strings.Join(parts, ", ")
}
