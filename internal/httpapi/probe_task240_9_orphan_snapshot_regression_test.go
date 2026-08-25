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

func TestTask240Bug09SelfCheckReportsOrphanSnapshot(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/orphan-snapshot.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.PutSnapshot(&model.RoundSnapshot{ID: "s-orphan", RoundID: "round-gone", State: model.SnapshotStatePublish, Summary: "{}"}); err != nil {
		t.Fatal(err)
	}
	h := httpapi.New(service.New(st)).Handler()
	resp := httptest.NewRecorder()
	h.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/selfcheck", nil))
	if resp.Code != http.StatusInternalServerError || !strings.Contains(resp.Body.String(), `"ok":false`) || !strings.Contains(resp.Body.String(), "orphan snapshot") {
		t.Fatalf("orphan snapshot was not reported: %d %s", resp.Code, resp.Body.String())
	}
}
