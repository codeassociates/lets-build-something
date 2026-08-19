package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/codeassociates/lets-build-something/backend/internal/db"
	"github.com/jackc/pgx/v5"
)

// Role determines what a signed-in user may do. The three roles map to the
// three interfaces: the shop, the service desk, and administration.
type Role string

const (
	RoleCustomer Role = "customer"
	RoleStaff    Role = "staff"
	RoleAdmin    Role = "admin"
)

// rank orders roles so a check can be "staff or above" rather than an
// enumeration of every role that qualifies.
func (r Role) rank() int {
	switch r {
	case RoleCustomer:
		return 1
	case RoleStaff:
		return 2
	case RoleAdmin:
		return 3
	}
	return 0
}

// AtLeast reports whether this role satisfies a minimum. Admins can always do
// what staff can do, which is what a small rental office actually wants.
func (r Role) AtLeast(min Role) bool { return r.rank() >= min.rank() }

func (r Role) Valid() bool { return r.rank() > 0 }

type User struct {
	ID            int64     `json:"id"`
	Email         string    `json:"email"`
	Role          Role      `json:"role"`
	FullName      string    `json:"full_name"`
	Phone         string    `json:"phone"`
	Company       string    `json:"company"`
	AddressLine1  string    `json:"address_line1"`
	AddressLine2  string    `json:"address_line2"`
	City          string    `json:"city"`
	State         string    `json:"state"`
	PostalCode    string    `json:"postal_code"`
	LicenseNumber string    `json:"license_number"`
	Active        bool      `json:"active"`
	CreatedAt     time.Time `json:"created_at"`
}

type Store struct{ pool *db.DB }

func NewStore(pool *db.DB) *Store { return &Store{pool: pool} }

const userColumns = `id, email, role, full_name, phone, company, address_line1,
	address_line2, city, state, postal_code, license_number, active, created_at`

func scanUser(row pgx.Row) (*User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Email, &u.Role, &u.FullName, &u.Phone, &u.Company,
		&u.AddressLine1, &u.AddressLine2, &u.City, &u.State, &u.PostalCode,
		&u.LicenseNumber, &u.Active, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

var ErrEmailTaken = errors.New("an account with that email already exists")

type NewUser struct {
	Email         string
	Password      string
	Role          Role
	FullName      string
	Phone         string
	Company       string
	AddressLine1  string
	AddressLine2  string
	City          string
	State         string
	PostalCode    string
	LicenseNumber string
}

func (s *Store) Create(ctx context.Context, in NewUser) (*User, error) {
	hash, err := HashPassword(in.Password)
	if err != nil {
		return nil, err
	}
	row := s.pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, role, full_name, phone, company,
			address_line1, address_line2, city, state, postal_code, license_number)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		RETURNING `+userColumns,
		NormalizeEmail(in.Email), hash, in.Role, in.FullName, in.Phone, in.Company,
		in.AddressLine1, in.AddressLine2, in.City, in.State, in.PostalCode, in.LicenseNumber)

	u, err := scanUser(row)
	if err != nil {
		if isUniqueViolation(err, "users_email_key") {
			return nil, ErrEmailTaken
		}
		return nil, fmt.Errorf("creating user: %w", err)
	}
	return u, nil
}

func (s *Store) ByID(ctx context.Context, id int64) (*User, error) {
	u, err := scanUser(s.pool.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return u, err
}

func (s *Store) ByEmail(ctx context.Context, email string) (*User, error) {
	u, err := scanUser(s.pool.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE email = $1`, NormalizeEmail(email)))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return u, err
}

// Authenticate verifies a password. It hashes even when no user was found, so
// the response time does not reveal whether an email is registered.
func (s *Store) Authenticate(ctx context.Context, email, password string) (*User, error) {
	var id int64
	var hash string
	var active bool
	err := s.pool.QueryRow(ctx,
		`SELECT id, password_hash, active FROM users WHERE email = $1`,
		NormalizeEmail(email)).Scan(&id, &hash, &active)

	if errors.Is(err, pgx.ErrNoRows) {
		// Deliberately burn the same work as a real verification.
		_ = VerifyPassword("pbkdf2-sha256$600000$AAAAAAAAAAAAAAAAAAAAAA$"+
			"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", password)
		return nil, ErrBadCredentials
	}
	if err != nil {
		return nil, fmt.Errorf("looking up user: %w", err)
	}
	if err := VerifyPassword(hash, password); err != nil {
		return nil, ErrBadCredentials
	}
	if !active {
		return nil, errors.New("this account has been deactivated")
	}
	return s.ByID(ctx, id)
}

type UserFilter struct {
	Role   Role
	Search string
	Limit  int
	Offset int
}

func (s *Store) List(ctx context.Context, f UserFilter) ([]User, int, error) {
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}
	search := ""
	if f.Search != "" {
		search = "%" + strings.ToLower(f.Search) + "%"
	}

	rows, err := s.pool.Query(ctx, `
		SELECT `+userColumns+`, COUNT(*) OVER () AS total
		FROM users
		WHERE ($1 = '' OR role = $1)
		  AND ($2 = '' OR lower(full_name) LIKE $2 OR lower(email) LIKE $2
		       OR lower(company) LIKE $2)
		ORDER BY full_name
		LIMIT $3 OFFSET $4`,
		string(f.Role), search, f.Limit, f.Offset)
	if err != nil {
		return nil, 0, fmt.Errorf("listing users: %w", err)
	}
	defer rows.Close()

	users := []User{}
	total := 0
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Email, &u.Role, &u.FullName, &u.Phone, &u.Company,
			&u.AddressLine1, &u.AddressLine2, &u.City, &u.State, &u.PostalCode,
			&u.LicenseNumber, &u.Active, &u.CreatedAt, &total); err != nil {
			return nil, 0, err
		}
		users = append(users, u)
	}
	return users, total, rows.Err()
}

type UserUpdate struct {
	FullName      *string
	Phone         *string
	Company       *string
	AddressLine1  *string
	AddressLine2  *string
	City          *string
	State         *string
	PostalCode    *string
	LicenseNumber *string
	Role          *Role
	Active        *bool
}

// Update applies only the fields the caller supplied. COALESCE keeps this a
// single statement without assembling SQL by hand.
func (s *Store) Update(ctx context.Context, id int64, up UserUpdate) (*User, error) {
	var role *string
	if up.Role != nil {
		r := string(*up.Role)
		role = &r
	}
	row := s.pool.QueryRow(ctx, `
		UPDATE users SET
			full_name      = COALESCE($2, full_name),
			phone          = COALESCE($3, phone),
			company        = COALESCE($4, company),
			address_line1  = COALESCE($5, address_line1),
			address_line2  = COALESCE($6, address_line2),
			city           = COALESCE($7, city),
			state          = COALESCE($8, state),
			postal_code    = COALESCE($9, postal_code),
			license_number = COALESCE($10, license_number),
			role           = COALESCE($11, role),
			active         = COALESCE($12, active),
			updated_at     = now()
		WHERE id = $1
		RETURNING `+userColumns,
		id, up.FullName, up.Phone, up.Company, up.AddressLine1, up.AddressLine2,
		up.City, up.State, up.PostalCode, up.LicenseNumber, role, up.Active)

	u, err := scanUser(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return u, err
}

func (s *Store) SetPassword(ctx context.Context, id int64, plaintext string) error {
	hash, err := HashPassword(plaintext)
	if err != nil {
		return err
	}
	tag, err := s.pool.Exec(ctx,
		`UPDATE users SET password_hash = $2, updated_at = now() WHERE id = $1`, id, hash)
	if err != nil {
		return fmt.Errorf("setting password: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return errors.New("user not found")
	}
	// Force other devices to sign in again with the new password.
	_, err = s.pool.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1`, id)
	return err
}

func NormalizeEmail(e string) string { return strings.ToLower(strings.TrimSpace(e)) }

func isUniqueViolation(err error, constraint string) bool {
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) && pgErr.SQLState() == "23505" {
		return strings.Contains(err.Error(), constraint) || constraint == ""
	}
	return false
}
