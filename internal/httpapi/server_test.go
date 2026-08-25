package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"task240-fedlineage/internal/httpapi"
	"task240-fedlineage/internal/service"
	"task240-fedlineage/internal/store"
)

func TestServerHealthAndRoundRegistration(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/http.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	h := httpapi.New(service.New(st)).Handler()
	health := httptest.NewRecorder()
	h.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if health.Code != http.StatusOK || !strings.Contains(health.Body.String(), `"status":"ok"`) {
		t.Fatalf("unexpected health response: %d %s", health.Code, health.Body.String())
	}
	req := httptest.NewRequest(http.MethodPost, "/api/rounds/register", strings.NewReader(`{"id":"r1","expected_dim":8}`))
	req.Header.Set("Content-Type", "application/json")
	created := httptest.NewRecorder()
	h.ServeHTTP(created, req)
	if created.Code != http.StatusCreated || !strings.Contains(created.Body.String(), `"ID":"r1"`) {
		t.Fatalf("unexpected registration response: %d %s", created.Code, created.Body.String())
	}
}
