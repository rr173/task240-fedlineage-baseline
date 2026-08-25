package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"task240-fedlineage/internal/httpapi"
	"task240-fedlineage/internal/model"
	"task240-fedlineage/internal/service"
	"task240-fedlineage/internal/store"
)

func TestTask240Bug10AncestorPathRejectsMissingParent(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/missing-ancestor.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.PutModel(&model.GlobalModel{ID: "m-child", RoundID: "r1", ParamDigest: "d1", Dimension: 8, State: model.NodeStateCandidate}); err != nil {
		t.Fatal(err)
	}
	if err := st.PutEdge(model.LineageEdge{Child: "m-child", Parent: "m-gone"}); err != nil {
		t.Fatal(err)
	}
	h := httpapi.New(service.New(st)).Handler()
	req := httptest.NewRequest(http.MethodPost, "/api/lineage/ancestors", strings.NewReader(`{"id":"m-child"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	h.ServeHTTP(resp, req)
	if resp.Code != http.StatusConflict || !strings.Contains(resp.Body.String(), "inconsistent") {
		t.Fatalf("missing parent path was accepted: %d %s", resp.Code, resp.Body.String())
	}
}
