package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/masculinecache/llpoa/search"
	"github.com/masculinecache/llpoa/tracing"
	"go.opentelemetry.io/otel/attribute"
)

type BylawHandler struct {
	store *search.Store
}

func NewBylawHandler(store *search.Store) *BylawHandler {
	return &BylawHandler{store: store}
}

func (h *BylawHandler) ListBylaws(w http.ResponseWriter, r *http.Request) {
	bylaws := h.store.List()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"count": len(bylaws),
		"data":  bylaws,
	})
}

func (h *BylawHandler) GetBylaw(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	section, ok := h.store.GetByID(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": "bylaw section not found",
		})
		return
	}
	writeJSON(w, http.StatusOK, section)
}

func (h *BylawHandler) SearchBylaws(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "query parameter 'q' is required",
		})
		return
	}

	span, ctx := tracing.NewSpan(r, "search.query")
	defer span.End()
	span.SetAttributes(attribute.String("query", query))

	results := h.store.Search(ctx, query)
	span.SetAttributes(attribute.Int("result_count", len(results)))

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"count":   len(results),
		"query":   query,
		"results": results,
	})
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
