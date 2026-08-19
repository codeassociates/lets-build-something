package api

import (
	"net/http"
	"time"

	"github.com/codeassociates/lets-build-something/backend/internal/auth"
	"github.com/codeassociates/lets-build-something/backend/internal/catalog"
	"github.com/codeassociates/lets-build-something/backend/internal/httpx"
)

func (s *Server) listCategories(w http.ResponseWriter, r *http.Request) error {
	categories, err := s.catalog.ListCategories(r.Context())
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"categories": categories})
	return nil
}

// window reads an optional start/end pair from the query string. Availability
// is only meaningful against dates, so callers that omit them get today.
func window(r *http.Request) (time.Time, time.Time, error) {
	start, _, err := httpx.QueryDate(r, "start")
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	end, _, err := httpx.QueryDate(r, "end")
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if !start.IsZero() && end.IsZero() {
		end = start
	}
	if start.IsZero() && !end.IsZero() {
		start = end
	}
	if !start.IsZero() && end.Before(start) {
		return time.Time{}, time.Time{}, httpx.BadRequest("The end date cannot be before the start date.")
	}
	return start, end, nil
}

func (s *Server) listModels(w http.ResponseWriter, r *http.Request) error {
	start, end, err := window(r)
	if err != nil {
		return err
	}
	models, total, err := s.catalog.List(r.Context(), catalog.ModelFilter{
		CategorySlug: httpx.Query(r, "category", ""),
		Search:       httpx.Query(r, "q", ""),
		Start:        start,
		End:          end,
		Limit:        httpx.QueryInt(r, "limit", 60),
		Offset:       httpx.QueryInt(r, "offset", 0),
	})
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"models": models, "total": total})
	return nil
}

func (s *Server) getModel(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	start, end, err := window(r)
	if err != nil {
		return err
	}
	model, err := s.catalog.Get(r.Context(), id, start, end)
	if err != nil {
		return err
	}
	if model == nil {
		return httpx.NotFound("That item")
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"model": model})
	return nil
}

// --- admin catalog management ---

func (s *Server) adminListModels(w http.ResponseWriter, r *http.Request) error {
	if _, err := auth.Require(r.Context(), auth.RoleStaff); err != nil {
		return err
	}
	models, total, err := s.catalog.List(r.Context(), catalog.ModelFilter{
		CategorySlug: httpx.Query(r, "category", ""),
		Search:       httpx.Query(r, "q", ""),
		IncludeIdle:  true,
		Limit:        httpx.QueryInt(r, "limit", 200),
		Offset:       httpx.QueryInt(r, "offset", 0),
	})
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"models": models, "total": total})
	return nil
}

func (s *Server) createCategory(w http.ResponseWriter, r *http.Request) error {
	if _, err := auth.Require(r.Context(), auth.RoleAdmin); err != nil {
		return err
	}
	var in catalog.Category
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	if in.Name == "" || in.Slug == "" {
		return httpx.Invalid(map[string]string{"name": "A category needs a name and a slug."})
	}
	created, err := s.catalog.CreateCategory(r.Context(), in)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"category": created})
	return nil
}

func (s *Server) updateCategory(w http.ResponseWriter, r *http.Request) error {
	if _, err := auth.Require(r.Context(), auth.RoleAdmin); err != nil {
		return err
	}
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	var in catalog.Category
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	updated, err := s.catalog.UpdateCategory(r.Context(), id, in)
	if err != nil {
		return err
	}
	if updated == nil {
		return httpx.NotFound("That category")
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"category": updated})
	return nil
}

func (s *Server) createModel(w http.ResponseWriter, r *http.Request) error {
	if _, err := auth.Require(r.Context(), auth.RoleAdmin); err != nil {
		return err
	}
	var in catalog.ModelInput
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	if problems := validateModel(in); len(problems) > 0 {
		return httpx.Invalid(problems)
	}
	id, err := s.catalog.CreateModel(r.Context(), in)
	if err != nil {
		return err
	}
	model, err := s.catalog.Get(r.Context(), id, time.Time{}, time.Time{})
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"model": model})
	return nil
}

func (s *Server) updateModel(w http.ResponseWriter, r *http.Request) error {
	if _, err := auth.Require(r.Context(), auth.RoleAdmin); err != nil {
		return err
	}
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	var in catalog.ModelInput
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	if problems := validateModel(in); len(problems) > 0 {
		return httpx.Invalid(problems)
	}
	if err := s.catalog.UpdateModel(r.Context(), id, in); err != nil {
		return httpx.NotFound("That item")
	}
	model, err := s.catalog.Get(r.Context(), id, time.Time{}, time.Time{})
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"model": model})
	return nil
}

func validateModel(in catalog.ModelInput) map[string]string {
	problems := map[string]string{}
	if in.Name == "" {
		problems["name"] = "Give the item a name."
	}
	if in.SKU == "" {
		problems["sku"] = "Give the item a SKU."
	}
	if in.CategoryID == 0 {
		problems["category_id"] = "Choose a category."
	}
	if in.DailyRateCents <= 0 {
		problems["daily_rate_cents"] = "A daily rate is required."
	}
	for field, v := range map[string]int64{
		"weekly_rate_cents":  int64(in.WeeklyRateCents),
		"monthly_rate_cents": int64(in.MonthlyRateCents),
		"deposit_cents":      int64(in.DepositCents),
	} {
		if v < 0 {
			problems[field] = "This cannot be negative."
		}
	}
	return problems
}

func (s *Server) listUnits(w http.ResponseWriter, r *http.Request) error {
	if _, err := auth.Require(r.Context(), auth.RoleStaff); err != nil {
		return err
	}
	units, total, err := s.catalog.ListUnits(r.Context(), catalog.UnitFilter{
		ModelID: int64(httpx.QueryInt(r, "model_id", 0)),
		Status:  httpx.Query(r, "status", ""),
		Search:  httpx.Query(r, "q", ""),
		Limit:   httpx.QueryInt(r, "limit", 100),
		Offset:  httpx.QueryInt(r, "offset", 0),
	})
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"units": units, "total": total})
	return nil
}

func (s *Server) createUnit(w http.ResponseWriter, r *http.Request) error {
	if _, err := auth.Require(r.Context(), auth.RoleAdmin); err != nil {
		return err
	}
	var in catalog.UnitInput
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	if problems := validateUnit(in); len(problems) > 0 {
		return httpx.Invalid(problems)
	}
	id, err := s.catalog.CreateUnit(r.Context(), in)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"id": id})
	return nil
}

func (s *Server) updateUnit(w http.ResponseWriter, r *http.Request) error {
	if _, err := auth.Require(r.Context(), auth.RoleStaff); err != nil {
		return err
	}
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	var in catalog.UnitInput
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	if problems := validateUnit(in); len(problems) > 0 {
		return httpx.Invalid(problems)
	}
	if err := s.catalog.UpdateUnit(r.Context(), id, in); err != nil {
		return httpx.NotFound("That unit")
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"id": id})
	return nil
}

func validateUnit(in catalog.UnitInput) map[string]string {
	problems := map[string]string{}
	if in.ModelID == 0 {
		problems["model_id"] = "Choose which item this unit is."
	}
	if in.AssetTag == "" {
		problems["asset_tag"] = "Every unit needs an asset tag."
	}
	switch in.Status {
	case "available", "out", "maintenance", "retired":
	default:
		problems["status"] = "Status must be available, out, maintenance or retired."
	}
	return problems
}
