package api

import (
	"errors"
	"net/http"
	"net/mail"
	"strings"

	"github.com/codeassociates/lets-build-something/backend/internal/auth"
	"github.com/codeassociates/lets-build-something/backend/internal/httpx"
)

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type registration struct {
	credentials
	FullName     string `json:"full_name"`
	Phone        string `json:"phone"`
	Company      string `json:"company"`
	AddressLine1 string `json:"address_line1"`
	City         string `json:"city"`
	State        string `json:"state"`
	PostalCode   string `json:"postal_code"`
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) error {
	var in registration
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	if problems := validateRegistration(in); len(problems) > 0 {
		return httpx.Invalid(problems)
	}

	user, err := s.users.Create(r.Context(), auth.NewUser{
		Email: in.Email, Password: in.Password, Role: auth.RoleCustomer,
		FullName: strings.TrimSpace(in.FullName), Phone: in.Phone, Company: in.Company,
		AddressLine1: in.AddressLine1, City: in.City, State: in.State, PostalCode: in.PostalCode,
	})
	if errors.Is(err, auth.ErrEmailTaken) {
		return httpx.Invalid(map[string]string{
			"email": "An account with that email already exists. Try signing in.",
		})
	}
	if err != nil {
		return err
	}

	sess, err := s.users.StartSession(r.Context(), user.ID, s.cfg.SessionTTL)
	if err != nil {
		return err
	}
	auth.SetCookie(w, sess, s.secureCookies)
	httpx.JSON(w, http.StatusCreated, map[string]any{"user": user})
	return nil
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) error {
	var in credentials
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	if in.Email == "" || in.Password == "" {
		return httpx.Invalid(map[string]string{"email": "Enter your email and password."})
	}

	user, err := s.users.Authenticate(r.Context(), in.Email, in.Password)
	if err != nil {
		if errors.Is(err, auth.ErrBadCredentials) {
			return httpx.Unauthorized("That email and password do not match an account.")
		}
		return httpx.Unauthorized(err.Error())
	}

	sess, err := s.users.StartSession(r.Context(), user.ID, s.cfg.SessionTTL)
	if err != nil {
		return err
	}
	auth.SetCookie(w, sess, s.secureCookies)
	httpx.JSON(w, http.StatusOK, map[string]any{"user": user})
	return nil
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) error {
	if token := auth.TokenFromRequest(r); token != "" {
		if err := s.users.EndSession(r.Context(), token); err != nil {
			return err
		}
	}
	auth.ClearCookie(w, s.secureCookies)
	httpx.NoContent(w)
	return nil
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) error {
	user := auth.FromContext(r.Context())
	if user == nil {
		// Anonymous is a normal state for the shop, not an error the UI must handle.
		httpx.JSON(w, http.StatusOK, map[string]any{"user": nil})
		return nil
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"user": user})
	return nil
}

func (s *Server) updateMe(w http.ResponseWriter, r *http.Request) error {
	user, err := auth.Require(r.Context(), auth.RoleCustomer)
	if err != nil {
		return err
	}

	var in struct {
		FullName      *string `json:"full_name"`
		Phone         *string `json:"phone"`
		Company       *string `json:"company"`
		AddressLine1  *string `json:"address_line1"`
		AddressLine2  *string `json:"address_line2"`
		City          *string `json:"city"`
		State         *string `json:"state"`
		PostalCode    *string `json:"postal_code"`
		LicenseNumber *string `json:"license_number"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}

	// Deliberately no Role or Active here: a customer cannot promote themselves.
	updated, err := s.users.Update(r.Context(), user.ID, auth.UserUpdate{
		FullName: in.FullName, Phone: in.Phone, Company: in.Company,
		AddressLine1: in.AddressLine1, AddressLine2: in.AddressLine2, City: in.City,
		State: in.State, PostalCode: in.PostalCode, LicenseNumber: in.LicenseNumber,
	})
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"user": updated})
	return nil
}

func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) error {
	user, err := auth.Require(r.Context(), auth.RoleCustomer)
	if err != nil {
		return err
	}
	var in struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	if len(in.NewPassword) < 8 {
		return httpx.Invalid(map[string]string{
			"new_password": "Choose a password of at least 8 characters.",
		})
	}
	if _, err := s.users.Authenticate(r.Context(), user.Email, in.CurrentPassword); err != nil {
		return httpx.Invalid(map[string]string{
			"current_password": "That is not your current password.",
		})
	}
	if err := s.users.SetPassword(r.Context(), user.ID, in.NewPassword); err != nil {
		return err
	}

	// SetPassword clears every session, including this one; issue a fresh one so
	// the person who just changed it stays signed in.
	sess, err := s.users.StartSession(r.Context(), user.ID, s.cfg.SessionTTL)
	if err != nil {
		return err
	}
	auth.SetCookie(w, sess, s.secureCookies)
	httpx.NoContent(w)
	return nil
}

func validateRegistration(in registration) map[string]string {
	problems := map[string]string{}
	if _, err := mail.ParseAddress(strings.TrimSpace(in.Email)); err != nil {
		problems["email"] = "Enter a valid email address."
	}
	if len(in.Password) < 8 {
		problems["password"] = "Choose a password of at least 8 characters."
	}
	if strings.TrimSpace(in.FullName) == "" {
		problems["full_name"] = "Tell us your name."
	}
	return problems
}
