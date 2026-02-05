package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/felipepmaragno/ai-gateway/internal/crypto"
	"github.com/felipepmaragno/ai-gateway/internal/domain"
	"github.com/felipepmaragno/ai-gateway/internal/repository"
	"github.com/google/uuid"
)

type AdminHandler struct {
	tenantRepo repository.TenantRepository
	mux        *http.ServeMux
}

func NewAdminHandler(tenantRepo repository.TenantRepository) *AdminHandler {
	h := &AdminHandler{
		tenantRepo: tenantRepo,
		mux:        http.NewServeMux(),
	}

	h.mux.HandleFunc("GET /admin/tenants", h.listTenants)
	h.mux.HandleFunc("POST /admin/tenants", h.createTenant)
	h.mux.HandleFunc("GET /admin/tenants/{id}", h.getTenant)
	h.mux.HandleFunc("PUT /admin/tenants/{id}", h.updateTenant)
	h.mux.HandleFunc("DELETE /admin/tenants/{id}", h.deleteTenant)
	h.mux.HandleFunc("POST /admin/tenants/{id}/rotate-key", h.rotateAPIKey)

	return h
}

func (h *AdminHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func (h *AdminHandler) listTenants(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	tenants, err := h.tenantRepo.List(ctx)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "failed to list tenants")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tenants": tenants,
		"count":   len(tenants),
	})
}

func (h *AdminHandler) createTenant(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req CreateTenantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		writeAdminError(w, http.StatusBadRequest, "name is required")
		return
	}

	apiKey := generateAPIKey()
	tenant := &domain.Tenant{
		ID:           uuid.New().String(),
		Name:         req.Name,
		APIKey:       apiKey,
		APIKeyHash:   crypto.HashAPIKey(apiKey),
		RateLimitRPM: req.RateLimitRPM,
		BudgetUSD:    req.BudgetUSD,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if tenant.RateLimitRPM == 0 {
		tenant.RateLimitRPM = 60
	}

	if err := h.tenantRepo.Create(ctx, tenant); err != nil {
		slog.Error("failed to create tenant", "error", err)
		writeAdminError(w, http.StatusInternalServerError, "failed to create tenant")
		return
	}

	slog.Info("tenant created", "tenant_id", tenant.ID, "name", tenant.Name)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(tenant)
}

func (h *AdminHandler) getTenant(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")

	tenant, err := h.tenantRepo.GetByID(ctx, id)
	if err != nil {
		writeAdminError(w, http.StatusNotFound, "tenant not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tenant)
}

func (h *AdminHandler) updateTenant(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")

	tenant, err := h.tenantRepo.GetByID(ctx, id)
	if err != nil {
		writeAdminError(w, http.StatusNotFound, "tenant not found")
		return
	}

	var req UpdateTenantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name != "" {
		tenant.Name = req.Name
	}
	if req.RateLimitRPM != nil {
		tenant.RateLimitRPM = *req.RateLimitRPM
	}
	if req.BudgetUSD != nil {
		tenant.BudgetUSD = *req.BudgetUSD
	}
	if req.Enabled != nil {
		tenant.Enabled = *req.Enabled
	}
	tenant.UpdatedAt = time.Now()

	if err := h.tenantRepo.Update(ctx, tenant); err != nil {
		slog.Error("failed to update tenant", "error", err)
		writeAdminError(w, http.StatusInternalServerError, "failed to update tenant")
		return
	}

	slog.Info("tenant updated", "tenant_id", tenant.ID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tenant)
}

func (h *AdminHandler) deleteTenant(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")

	if err := h.tenantRepo.Delete(ctx, id); err != nil {
		writeAdminError(w, http.StatusNotFound, "tenant not found")
		return
	}

	slog.Info("tenant deleted", "tenant_id", id)

	w.WriteHeader(http.StatusNoContent)
}

func (h *AdminHandler) rotateAPIKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")

	tenant, err := h.tenantRepo.GetByID(ctx, id)
	if err != nil {
		writeAdminError(w, http.StatusNotFound, "tenant not found")
		return
	}

	tenant.APIKey = generateAPIKey()
	tenant.UpdatedAt = time.Now()

	if err := h.tenantRepo.Update(ctx, tenant); err != nil {
		slog.Error("failed to rotate API key", "error", err)
		writeAdminError(w, http.StatusInternalServerError, "failed to rotate API key")
		return
	}

	slog.Info("API key rotated", "tenant_id", tenant.ID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"api_key": tenant.APIKey,
	})
}

type CreateTenantRequest struct {
	Name         string  `json:"name"`
	RateLimitRPM int     `json:"rate_limit_rpm"`
	BudgetUSD    float64 `json:"budget_usd"`
}

type UpdateTenantRequest struct {
	Name         string   `json:"name,omitempty"`
	RateLimitRPM *int     `json:"rate_limit_rpm,omitempty"`
	BudgetUSD    *float64 `json:"budget_usd,omitempty"`
	Enabled      *bool    `json:"enabled,omitempty"`
}

func generateAPIKey() string {
	return "gw-" + uuid.New().String()
}

func writeAdminError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": message,
	})
}
