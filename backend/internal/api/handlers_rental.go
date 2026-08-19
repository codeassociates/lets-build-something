package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/codeassociates/lets-build-something/backend/internal/auth"
	"github.com/codeassociates/lets-build-something/backend/internal/billing"
	"github.com/codeassociates/lets-build-something/backend/internal/httpx"
	"github.com/codeassociates/lets-build-something/backend/internal/rental"
)

func (s *Server) quote(w http.ResponseWriter, r *http.Request) error {
	var in rental.QuoteRequest
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	quote, err := s.rentals.Quote(r.Context(), in)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"quote": quote})
	return nil
}

type createReservationRequest struct {
	rental.QuoteRequest
	Notes string        `json:"notes"`
	Card  *billing.Card `json:"card"`
	// CustomerID lets staff book on a customer's behalf at the counter.
	CustomerID int64 `json:"customer_id"`
}

func (s *Server) createReservation(w http.ResponseWriter, r *http.Request) error {
	user, err := auth.Require(r.Context(), auth.RoleCustomer)
	if err != nil {
		return err
	}
	var in createReservationRequest
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}

	customerID := user.ID
	if in.CustomerID != 0 && in.CustomerID != user.ID {
		if !user.Role.AtLeast(auth.RoleStaff) {
			return httpx.Forbidden("You can only book for yourself.")
		}
		customer, err := s.users.ByID(r.Context(), in.CustomerID)
		if err != nil {
			return err
		}
		if customer == nil {
			return httpx.BadRequest("That customer does not exist.")
		}
		customerID = customer.ID
	}

	result, err := s.rentals.Book(r.Context(), rental.BookRequest{
		QuoteRequest: in.QuoteRequest,
		CustomerID:   customerID,
		Notes:        in.Notes,
		CreatedBy:    user.ID,
		Card:         in.Card,
	})
	if errors.Is(err, rental.ErrUnavailable) {
		return httpx.Conflict(err.Error())
	}
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusCreated, result)
	return nil
}

func (s *Server) listReservations(w http.ResponseWriter, r *http.Request) error {
	user, err := auth.Require(r.Context(), auth.RoleCustomer)
	if err != nil {
		return err
	}

	f := rental.Filter{
		Status:      httpx.Query(r, "status", ""),
		Search:      httpx.Query(r, "q", ""),
		OverdueOnly: httpx.QueryBool(r, "overdue"),
		ActiveOnly:  httpx.QueryBool(r, "active"),
		Limit:       httpx.QueryInt(r, "limit", 50),
		Offset:      httpx.QueryInt(r, "offset", 0),
	}

	// Customers see only their own bookings, whatever they ask for.
	if user.Role.AtLeast(auth.RoleStaff) {
		f.CustomerID = int64(httpx.QueryInt(r, "customer_id", 0))
	} else {
		f.CustomerID = user.ID
	}

	reservations, total, err := s.rentals.List(r.Context(), f)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"reservations": reservations, "total": total})
	return nil
}

// loadReservationFor fetches a reservation and enforces that the caller may see
// it — the check every reservation route needs.
func (s *Server) loadReservationFor(r *http.Request) (*rental.Reservation, error) {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return nil, err
	}
	res, err := s.rentals.Get(r.Context(), id)
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, httpx.NotFound("That reservation")
	}
	if _, err := auth.RequireSelfOrStaff(r.Context(), res.CustomerID); err != nil {
		return nil, err
	}
	return res, nil
}

func (s *Server) getReservation(w http.ResponseWriter, r *http.Request) error {
	res, err := s.loadReservationFor(r)
	if err != nil {
		return err
	}
	invoices, _, err := s.billing.ListInvoices(r.Context(), billing.InvoiceFilter{
		CustomerID: res.CustomerID, Limit: 200,
	})
	if err != nil {
		return err
	}
	mine := []billing.Invoice{}
	for _, inv := range invoices {
		if inv.ReservationID == res.ID {
			mine = append(mine, inv)
		}
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"reservation": res, "invoices": mine})
	return nil
}

func (s *Server) cancelReservation(w http.ResponseWriter, r *http.Request) error {
	res, err := s.loadReservationFor(r)
	if err != nil {
		return err
	}
	var in struct {
		Reason string `json:"reason"`
	}
	if r.ContentLength > 0 {
		if err := httpx.Decode(r, &in); err != nil {
			return err
		}
	}

	updated, err := s.rentals.Cancel(r.Context(), res.ID, in.Reason)
	if errors.Is(err, rental.ErrBadTransition) {
		return httpx.Conflict(err.Error())
	}
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"reservation": updated})
	return nil
}

// --- service desk ---

func (s *Server) deskSummary(w http.ResponseWriter, r *http.Request) error {
	if _, err := auth.Require(r.Context(), auth.RoleStaff); err != nil {
		return err
	}
	day := time.Now().UTC()
	if d, ok, err := httpx.QueryDate(r, "date"); err != nil {
		return err
	} else if ok {
		day = d
	}
	summary, err := s.rentals.Desk(r.Context(), day)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"desk": summary})
	return nil
}

// deskLookup is the counter's search box: a reservation number, a name, or an
// email all find the same booking.
func (s *Server) deskLookup(w http.ResponseWriter, r *http.Request) error {
	if _, err := auth.Require(r.Context(), auth.RoleStaff); err != nil {
		return err
	}
	q := httpx.Query(r, "q", "")
	if q == "" {
		return httpx.BadRequest("Enter a reservation number, name or email to search for.")
	}

	if res, err := s.rentals.ByNumber(r.Context(), q); err == nil && res != nil {
		httpx.JSON(w, http.StatusOK, map[string]any{"reservations": []any{res}, "total": 1})
		return nil
	}
	reservations, total, err := s.rentals.List(r.Context(), rental.Filter{
		Search: q, Limit: 25,
	})
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"reservations": reservations, "total": total})
	return nil
}

// availableUnitsFor tells the counter which physical machines can be handed
// over for each line of a reservation.
func (s *Server) availableUnitsFor(w http.ResponseWriter, r *http.Request) error {
	if _, err := auth.Require(r.Context(), auth.RoleStaff); err != nil {
		return err
	}
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	res, err := s.rentals.Get(r.Context(), id)
	if err != nil {
		return err
	}
	if res == nil {
		return httpx.NotFound("That reservation")
	}

	type lineUnits struct {
		ItemID    int64  `json:"item_id"`
		ModelID   int64  `json:"model_id"`
		ModelName string `json:"model_name"`
		Quantity  int    `json:"quantity"`
		Units     any    `json:"units"`
	}
	out := []lineUnits{}
	for _, item := range res.Items {
		units, err := s.catalog.FreeUnits(r.Context(), item.ModelID)
		if err != nil {
			return err
		}
		out = append(out, lineUnits{
			ItemID: item.ID, ModelID: item.ModelID, ModelName: item.ModelName,
			Quantity: item.Quantity, Units: units,
		})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"lines": out})
	return nil
}

func (s *Server) checkout(w http.ResponseWriter, r *http.Request) error {
	if _, err := auth.Require(r.Context(), auth.RoleStaff); err != nil {
		return err
	}
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	var in rental.CheckoutRequest
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	if len(in.Lines) == 0 {
		return httpx.BadRequest("Assign at least one unit before checking out.")
	}

	res, err := s.rentals.Checkout(r.Context(), id, in)
	switch {
	case errors.Is(err, rental.ErrNotFound):
		return httpx.NotFound("That reservation")
	case errors.Is(err, rental.ErrBadTransition):
		return httpx.Conflict(err.Error())
	case err != nil:
		return err
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"reservation": res})
	return nil
}

func (s *Server) checkin(w http.ResponseWriter, r *http.Request) error {
	if _, err := auth.Require(r.Context(), auth.RoleStaff); err != nil {
		return err
	}
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	var in rental.CheckinRequest
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	if len(in.Lines) == 0 {
		return httpx.BadRequest("Check in at least one item.")
	}

	result, err := s.rentals.Checkin(r.Context(), id, in)
	switch {
	case errors.Is(err, rental.ErrNotFound):
		return httpx.NotFound("That reservation")
	case errors.Is(err, rental.ErrBadTransition):
		return httpx.Conflict(err.Error())
	case err != nil:
		return err
	}
	httpx.JSON(w, http.StatusOK, result)
	return nil
}
