package auth

import (
	"context"
	"net/http"

	"github.com/codeassociates/lets-build-something/backend/internal/httpx"
)

type ctxKey struct{}

// Middleware resolves the session cookie on every request and stashes the user
// in the context. It never rejects — that is the job of Require, so public and
// private routes can share one chain.
func (s *Store) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := TokenFromRequest(r)
		if token != "" {
			if user, err := s.LookupSession(r.Context(), token); err == nil && user != nil {
				r = r.WithContext(context.WithValue(r.Context(), ctxKey{}, user))
			}
		}
		next.ServeHTTP(w, r)
	})
}

// FromContext returns the signed-in user, or nil for an anonymous request.
func FromContext(ctx context.Context) *User {
	u, _ := ctx.Value(ctxKey{}).(*User)
	return u
}

// Require returns the signed-in user or an error suitable for returning
// straight out of a handler.
func Require(ctx context.Context, min Role) (*User, error) {
	u := FromContext(ctx)
	if u == nil {
		return nil, httpx.Unauthorized("You need to sign in to do that.")
	}
	if !u.Role.AtLeast(min) {
		return nil, httpx.Forbidden("Your account does not have access to that.")
	}
	return u, nil
}

// RequireSelfOrStaff allows a customer to reach their own records while letting
// staff reach anyone's — the check nearly every customer-facing route needs.
func RequireSelfOrStaff(ctx context.Context, customerID int64) (*User, error) {
	u, err := Require(ctx, RoleCustomer)
	if err != nil {
		return nil, err
	}
	if u.Role.AtLeast(RoleStaff) || u.ID == customerID {
		return u, nil
	}
	return nil, httpx.Forbidden("Your account does not have access to that.")
}
