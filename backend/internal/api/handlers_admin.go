package api

import (
	"errors"
	"net/http"
	"net/mail"
	"strings"

	"github.com/codeassociates/lets-build-something/backend/internal/auth"
	"github.com/codeassociates/lets-build-something/backend/internal/billing"
	"github.com/codeassociates/lets-build-something/backend/internal/httpx"
	"github.com/codeassociates/lets-build-something/backend/internal/jobs"
	"github.com/codeassociates/lets-build-something/backend/internal/money"
	"github.com/codeassociates/lets-build-something/backend/internal/notify"
)

// --- invoices and payments ---

func (s *Server) listInvoices(w http.ResponseWriter, r *http.Request) error {
	user, err := auth.Require(r.Context(), auth.RoleCustomer)
	if err != nil {
		return err
	}
	f := billing.InvoiceFilter{
		Status:     httpx.Query(r, "status", ""),
		UnpaidOnly: httpx.QueryBool(r, "unpaid"),
		Limit:      httpx.QueryInt(r, "limit", 50),
		Offset:     httpx.QueryInt(r, "offset", 0),
	}
	if user.Role.AtLeast(auth.RoleStaff) {
		f.CustomerID = int64(httpx.QueryInt(r, "customer_id", 0))
	} else {
		f.CustomerID = user.ID
	}

	invoices, total, err := s.billing.ListInvoices(r.Context(), f)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"invoices": invoices, "total": total})
	return nil
}

func (s *Server) loadInvoiceFor(r *http.Request) (*billing.Invoice, error) {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return nil, err
	}
	inv, err := s.billing.GetInvoice(r.Context(), id)
	if err != nil {
		return nil, err
	}
	if inv == nil {
		return nil, httpx.NotFound("That invoice")
	}
	if _, err := auth.RequireSelfOrStaff(r.Context(), inv.CustomerID); err != nil {
		return nil, err
	}
	return inv, nil
}

func (s *Server) getInvoice(w http.ResponseWriter, r *http.Request) error {
	inv, err := s.loadInvoiceFor(r)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"invoice": inv})
	return nil
}

func (s *Server) payInvoice(w http.ResponseWriter, r *http.Request) error {
	inv, err := s.loadInvoiceFor(r)
	if err != nil {
		return err
	}
	var in struct {
		Card billing.Card `json:"card"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	if in.Card.Number == "" {
		return httpx.Invalid(map[string]string{"card": "Enter your card details."})
	}

	paid, err := s.billing.PayInvoice(r.Context(), inv.ID, in.Card)
	switch {
	case errors.Is(err, billing.ErrAlreadySettled):
		return httpx.Conflict("That invoice has already been paid.")
	case errors.Is(err, billing.ErrDeclined):
		return &httpx.Error{Status: http.StatusPaymentRequired, Code: "card_declined",
			Message: err.Error()}
	case err != nil:
		return err
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"invoice": paid})
	return nil
}

func (s *Server) refundInvoice(w http.ResponseWriter, r *http.Request) error {
	if _, err := auth.Require(r.Context(), auth.RoleStaff); err != nil {
		return err
	}
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		AmountCents int64 `json:"amount_cents"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}

	inv, err := s.billing.RefundInvoice(r.Context(), id, money.Cents(in.AmountCents))
	if err != nil {
		return httpx.BadRequest(err.Error())
	}
	if inv == nil {
		return httpx.NotFound("That invoice")
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"invoice": inv})
	return nil
}

func (s *Server) listPayments(w http.ResponseWriter, r *http.Request) error {
	user, err := auth.Require(r.Context(), auth.RoleCustomer)
	if err != nil {
		return err
	}
	f := billing.PaymentFilter{
		ReservationID: int64(httpx.QueryInt(r, "reservation_id", 0)),
		Limit:         httpx.QueryInt(r, "limit", 100),
	}
	if user.Role.AtLeast(auth.RoleStaff) {
		f.CustomerID = int64(httpx.QueryInt(r, "customer_id", 0))
	} else {
		f.CustomerID = user.ID
	}

	payments, err := s.billing.ListPayments(r.Context(), f)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"payments": payments})
	return nil
}

// --- customers, as seen by staff ---

func (s *Server) listCustomers(w http.ResponseWriter, r *http.Request) error {
	if _, err := auth.Require(r.Context(), auth.RoleStaff); err != nil {
		return err
	}
	users, total, err := s.users.List(r.Context(), auth.UserFilter{
		Role:   auth.RoleCustomer,
		Search: httpx.Query(r, "q", ""),
		Limit:  httpx.QueryInt(r, "limit", 50),
		Offset: httpx.QueryInt(r, "offset", 0),
	})
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"customers": users, "total": total})
	return nil
}

func (s *Server) getCustomer(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	if _, err := auth.RequireSelfOrStaff(r.Context(), id); err != nil {
		return err
	}
	customer, err := s.users.ByID(r.Context(), id)
	if err != nil {
		return err
	}
	if customer == nil {
		return httpx.NotFound("That customer")
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"customer": customer})
	return nil
}

// createCustomer registers a walk-in at the counter. The temporary password is
// returned once so staff can hand it over; it is never retrievable again.
func (s *Server) createCustomer(w http.ResponseWriter, r *http.Request) error {
	if _, err := auth.Require(r.Context(), auth.RoleStaff); err != nil {
		return err
	}
	var in struct {
		Email         string `json:"email"`
		FullName      string `json:"full_name"`
		Phone         string `json:"phone"`
		Company       string `json:"company"`
		AddressLine1  string `json:"address_line1"`
		City          string `json:"city"`
		State         string `json:"state"`
		PostalCode    string `json:"postal_code"`
		LicenseNumber string `json:"license_number"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	if _, err := mail.ParseAddress(strings.TrimSpace(in.Email)); err != nil {
		return httpx.Invalid(map[string]string{"email": "Enter a valid email address."})
	}
	if strings.TrimSpace(in.FullName) == "" {
		return httpx.Invalid(map[string]string{"full_name": "Enter the customer's name."})
	}

	temporary := auth.GenerateTemporaryPassword()
	customer, err := s.users.Create(r.Context(), auth.NewUser{
		Email: in.Email, Password: temporary, Role: auth.RoleCustomer,
		FullName: in.FullName, Phone: in.Phone, Company: in.Company,
		AddressLine1: in.AddressLine1, City: in.City, State: in.State,
		PostalCode: in.PostalCode, LicenseNumber: in.LicenseNumber,
	})
	if errors.Is(err, auth.ErrEmailTaken) {
		return httpx.Invalid(map[string]string{"email": "That email is already registered."})
	}
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{
		"customer": customer, "temporary_password": temporary,
	})
	return nil
}

// --- administration ---

func (s *Server) stats(w http.ResponseWriter, r *http.Request) error {
	if _, err := auth.Require(r.Context(), auth.RoleStaff); err != nil {
		return err
	}
	stats, err := s.rentals.Stats(r.Context())
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"stats": stats})
	return nil
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) error {
	if _, err := auth.Require(r.Context(), auth.RoleAdmin); err != nil {
		return err
	}
	users, total, err := s.users.List(r.Context(), auth.UserFilter{
		Role:   auth.Role(httpx.Query(r, "role", "")),
		Search: httpx.Query(r, "q", ""),
		Limit:  httpx.QueryInt(r, "limit", 100),
		Offset: httpx.QueryInt(r, "offset", 0),
	})
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"users": users, "total": total})
	return nil
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) error {
	if _, err := auth.Require(r.Context(), auth.RoleAdmin); err != nil {
		return err
	}
	var in struct {
		Email    string    `json:"email"`
		Password string    `json:"password"`
		Role     auth.Role `json:"role"`
		FullName string    `json:"full_name"`
		Phone    string    `json:"phone"`
		Company  string    `json:"company"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}

	problems := map[string]string{}
	if _, err := mail.ParseAddress(strings.TrimSpace(in.Email)); err != nil {
		problems["email"] = "Enter a valid email address."
	}
	if !in.Role.Valid() {
		problems["role"] = "Choose customer, staff or admin."
	}
	if strings.TrimSpace(in.FullName) == "" {
		problems["full_name"] = "Enter a name."
	}
	if len(problems) > 0 {
		return httpx.Invalid(problems)
	}

	password, generated := in.Password, ""
	if len(password) < 8 {
		password = auth.GenerateTemporaryPassword()
		generated = password
	}

	user, err := s.users.Create(r.Context(), auth.NewUser{
		Email: in.Email, Password: password, Role: in.Role,
		FullName: in.FullName, Phone: in.Phone, Company: in.Company,
	})
	if errors.Is(err, auth.ErrEmailTaken) {
		return httpx.Invalid(map[string]string{"email": "That email is already registered."})
	}
	if err != nil {
		return err
	}
	body := map[string]any{"user": user}
	if generated != "" {
		body["temporary_password"] = generated
	}
	httpx.JSON(w, http.StatusCreated, body)
	return nil
}

func (s *Server) updateUser(w http.ResponseWriter, r *http.Request) error {
	admin, err := auth.Require(r.Context(), auth.RoleAdmin)
	if err != nil {
		return err
	}
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		FullName      *string    `json:"full_name"`
		Phone         *string    `json:"phone"`
		Company       *string    `json:"company"`
		AddressLine1  *string    `json:"address_line1"`
		AddressLine2  *string    `json:"address_line2"`
		City          *string    `json:"city"`
		State         *string    `json:"state"`
		PostalCode    *string    `json:"postal_code"`
		LicenseNumber *string    `json:"license_number"`
		Role          *auth.Role `json:"role"`
		Active        *bool      `json:"active"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	if in.Role != nil && !in.Role.Valid() {
		return httpx.Invalid(map[string]string{"role": "Choose customer, staff or admin."})
	}

	// Guard against an admin locking themselves out of their own system.
	if admin.ID == id {
		if in.Active != nil && !*in.Active {
			return httpx.BadRequest("You cannot deactivate your own account.")
		}
		if in.Role != nil && *in.Role != auth.RoleAdmin {
			return httpx.BadRequest("You cannot remove your own admin access.")
		}
	}

	updated, err := s.users.Update(r.Context(), id, auth.UserUpdate{
		FullName: in.FullName, Phone: in.Phone, Company: in.Company,
		AddressLine1: in.AddressLine1, AddressLine2: in.AddressLine2, City: in.City,
		State: in.State, PostalCode: in.PostalCode, LicenseNumber: in.LicenseNumber,
		Role: in.Role, Active: in.Active,
	})
	if err != nil {
		return err
	}
	if updated == nil {
		return httpx.NotFound("That user")
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"user": updated})
	return nil
}

func (s *Server) resetPassword(w http.ResponseWriter, r *http.Request) error {
	if _, err := auth.Require(r.Context(), auth.RoleAdmin); err != nil {
		return err
	}
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		Password string `json:"password"`
	}
	if r.ContentLength > 0 {
		if err := httpx.Decode(r, &in); err != nil {
			return err
		}
	}
	password, generated := in.Password, ""
	if len(password) < 8 {
		password = auth.GenerateTemporaryPassword()
		generated = password
	}
	if err := s.users.SetPassword(r.Context(), id, password); err != nil {
		return httpx.NotFound("That user")
	}
	body := map[string]any{"ok": true}
	if generated != "" {
		body["temporary_password"] = generated
	}
	httpx.JSON(w, http.StatusOK, body)
	return nil
}

func (s *Server) listEmails(w http.ResponseWriter, r *http.Request) error {
	if _, err := auth.Require(r.Context(), auth.RoleStaff); err != nil {
		return err
	}
	emails, total, err := s.notifier.Log(r.Context(), notify.LogFilter{
		Template:      httpx.Query(r, "template", ""),
		Status:        httpx.Query(r, "status", ""),
		ReservationID: int64(httpx.QueryInt(r, "reservation_id", 0)),
		Limit:         httpx.QueryInt(r, "limit", 50),
		Offset:        httpx.QueryInt(r, "offset", 0),
	})
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"emails": emails, "total": total})
	return nil
}

func (s *Server) getEmail(w http.ResponseWriter, r *http.Request) error {
	if _, err := auth.Require(r.Context(), auth.RoleStaff); err != nil {
		return err
	}
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	email, err := s.notifier.GetLogEntry(r.Context(), id)
	if err != nil {
		return httpx.NotFound("That email")
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"email": email})
	return nil
}

func (s *Server) listJobs(w http.ResponseWriter, r *http.Request) error {
	if _, err := auth.Require(r.Context(), auth.RoleAdmin); err != nil {
		return err
	}
	list, err := s.queue.List(r.Context(), jobs.Filter{
		Status: httpx.Query(r, "status", ""),
		Kind:   httpx.Query(r, "kind", ""),
		Limit:  httpx.QueryInt(r, "limit", 100),
	})
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"jobs": list})
	return nil
}
