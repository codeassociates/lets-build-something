// Package api wires the HTTP surface: middleware, routing, and handlers.
package api

import (
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/codeassociates/lets-build-something/backend/internal/auth"
	"github.com/codeassociates/lets-build-something/backend/internal/billing"
	"github.com/codeassociates/lets-build-something/backend/internal/catalog"
	"github.com/codeassociates/lets-build-something/backend/internal/config"
	"github.com/codeassociates/lets-build-something/backend/internal/db"
	"github.com/codeassociates/lets-build-something/backend/internal/httpx"
	"github.com/codeassociates/lets-build-something/backend/internal/jobs"
	"github.com/codeassociates/lets-build-something/backend/internal/notify"
	"github.com/codeassociates/lets-build-something/backend/internal/rental"
)

type Server struct {
	cfg      *config.Config
	pool     *db.DB
	users    *auth.Store
	catalog  *catalog.Store
	rentals  *rental.Service
	billing  *billing.Store
	notifier *notify.Notifier
	queue    *jobs.Queue
	// secureCookies is off for plain-HTTP local development and on behind TLS.
	secureCookies bool
}

type Deps struct {
	Config   *config.Config
	Pool     *db.DB
	Users    *auth.Store
	Catalog  *catalog.Store
	Rentals  *rental.Service
	Billing  *billing.Store
	Notifier *notify.Notifier
	Queue    *jobs.Queue
}

func NewServer(d Deps) *Server {
	return &Server{
		cfg: d.Config, pool: d.Pool, users: d.Users, catalog: d.Catalog,
		rentals: d.Rentals, billing: d.Billing, notifier: d.Notifier, queue: d.Queue,
		secureCookies: strings.HasPrefix(d.Config.PublicBaseURL, "https://"),
	}
}

// Handler builds the full route table. Routes are grouped by who may reach
// them, and each handler re-checks its own authorization rather than relying on
// where it sits in this list.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// --- open ---
	mux.HandleFunc("GET /api/v1/health", s.health)
	mux.Handle("POST /api/v1/auth/register", httpx.Wrap(s.register))
	mux.Handle("POST /api/v1/auth/login", httpx.Wrap(s.login))
	mux.Handle("POST /api/v1/auth/logout", httpx.Wrap(s.logout))
	mux.Handle("GET /api/v1/auth/me", httpx.Wrap(s.me))
	mux.Handle("PATCH /api/v1/auth/me", httpx.Wrap(s.updateMe))
	mux.Handle("POST /api/v1/auth/password", httpx.Wrap(s.changePassword))

	// --- shop: readable by anyone, so customers can browse before signing up ---
	mux.Handle("GET /api/v1/categories", httpx.Wrap(s.listCategories))
	mux.Handle("GET /api/v1/models", httpx.Wrap(s.listModels))
	mux.Handle("GET /api/v1/models/{id}", httpx.Wrap(s.getModel))
	mux.Handle("POST /api/v1/quote", httpx.Wrap(s.quote))

	// --- customer ---
	mux.Handle("GET /api/v1/reservations", httpx.Wrap(s.listReservations))
	mux.Handle("POST /api/v1/reservations", httpx.Wrap(s.createReservation))
	mux.Handle("GET /api/v1/reservations/{id}", httpx.Wrap(s.getReservation))
	mux.Handle("POST /api/v1/reservations/{id}/cancel", httpx.Wrap(s.cancelReservation))
	mux.Handle("GET /api/v1/invoices", httpx.Wrap(s.listInvoices))
	mux.Handle("GET /api/v1/invoices/{id}", httpx.Wrap(s.getInvoice))
	mux.Handle("POST /api/v1/invoices/{id}/pay", httpx.Wrap(s.payInvoice))
	mux.Handle("GET /api/v1/payments", httpx.Wrap(s.listPayments))

	// --- service desk ---
	mux.Handle("GET /api/v1/desk/summary", httpx.Wrap(s.deskSummary))
	mux.Handle("GET /api/v1/desk/lookup", httpx.Wrap(s.deskLookup))
	mux.Handle("GET /api/v1/desk/reservations/{id}/available-units", httpx.Wrap(s.availableUnitsFor))
	mux.Handle("POST /api/v1/desk/reservations/{id}/checkout", httpx.Wrap(s.checkout))
	mux.Handle("POST /api/v1/desk/reservations/{id}/checkin", httpx.Wrap(s.checkin))
	mux.Handle("POST /api/v1/invoices/{id}/refund", httpx.Wrap(s.refundInvoice))
	mux.Handle("GET /api/v1/customers", httpx.Wrap(s.listCustomers))
	mux.Handle("POST /api/v1/customers", httpx.Wrap(s.createCustomer))
	mux.Handle("GET /api/v1/customers/{id}", httpx.Wrap(s.getCustomer))

	// --- administration ---
	mux.Handle("GET /api/v1/admin/stats", httpx.Wrap(s.stats))
	mux.Handle("POST /api/v1/admin/categories", httpx.Wrap(s.createCategory))
	mux.Handle("PATCH /api/v1/admin/categories/{id}", httpx.Wrap(s.updateCategory))
	mux.Handle("GET /api/v1/admin/models", httpx.Wrap(s.adminListModels))
	mux.Handle("POST /api/v1/admin/models", httpx.Wrap(s.createModel))
	mux.Handle("PATCH /api/v1/admin/models/{id}", httpx.Wrap(s.updateModel))
	mux.Handle("GET /api/v1/admin/units", httpx.Wrap(s.listUnits))
	mux.Handle("POST /api/v1/admin/units", httpx.Wrap(s.createUnit))
	mux.Handle("PATCH /api/v1/admin/units/{id}", httpx.Wrap(s.updateUnit))
	mux.Handle("GET /api/v1/admin/users", httpx.Wrap(s.listUsers))
	mux.Handle("POST /api/v1/admin/users", httpx.Wrap(s.createUser))
	mux.Handle("PATCH /api/v1/admin/users/{id}", httpx.Wrap(s.updateUser))
	mux.Handle("POST /api/v1/admin/users/{id}/password", httpx.Wrap(s.resetPassword))
	mux.Handle("GET /api/v1/admin/emails", httpx.Wrap(s.listEmails))
	mux.Handle("GET /api/v1/admin/emails/{id}", httpx.Wrap(s.getEmail))
	mux.Handle("GET /api/v1/admin/jobs", httpx.Wrap(s.listJobs))

	return s.recoverPanics(s.logRequests(s.cors(s.users.Middleware(mux))))
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	status := map[string]any{"status": "ok", "time": time.Now().UTC()}
	if err := s.pool.Ping(r.Context()); err != nil {
		status["status"] = "degraded"
		status["database"] = err.Error()
		httpx.JSON(w, http.StatusServiceUnavailable, status)
		return
	}
	status["database"] = "ok"
	httpx.JSON(w, http.StatusOK, status)
}

// cors permits the configured frontend origins with credentials, which the
// session cookie requires. A wildcard origin cannot carry credentials, so the
// allowed list is echoed back exactly.
func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && (slices.Contains(s.cfg.CORSOrigins, origin) ||
			slices.Contains(s.cfg.CORSOrigins, "*")) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept")
			w.Header().Set("Access-Control-Max-Age", "600")
			w.Header().Add("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		level := slog.LevelInfo
		if rec.status >= 500 {
			level = slog.LevelError
		}
		slog.Log(r.Context(), level, "request",
			"method", r.Method, "path", r.URL.Path, "status", rec.status,
			"took", time.Since(start).Round(time.Millisecond))
	})
}

// recoverPanics keeps one bad request from taking the process down.
func (s *Server) recoverPanics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic serving request", "err", rec, "path", r.URL.Path)
				httpx.JSON(w, http.StatusInternalServerError, &httpx.Error{
					Code: "internal", Message: "Something went wrong on our end.",
				})
			}
		}()
		next.ServeHTTP(w, r)
	})
}
