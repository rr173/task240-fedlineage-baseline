package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"task240-fedlineage/internal/httpapi"
	"task240-fedlineage/internal/model"
	"task240-fedlineage/internal/service"
	"task240-fedlineage/internal/store"
)

func TestTask240Bug08MissingParentReturnsForkedVerdict(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/missing-parent.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	sv := service.New(st)
	if _, err := sv.Round.Register("r1", "", 8); err != nil {
		t.Fatal(err)
	}
	if _, err := sv.Round.Open("r1"); err != nil {
		t.Fatal(err)
	}
	if _, err := sv.Update.Receive(&model.ClientUpdate{ID: "u-missing-parent", RoundID: "r1", ClientID: "c1", ParentModel: "m-gone", ParamDigest: "d1", Dimension: 8}); err != nil {
		t.Fatal(err)
	}
	if _, err := sv.Round.Close("r1"); err != nil {
		t.Fatal(err)
	}
	h := httpapi.New(sv).Handler()
	body, _ := json.Marshal(map[string]string{"id": "u-missing-parent"})
	req := httptest.NewRequest(http.MethodPost, "/api/lineage/verify", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	h.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), `"Verdict":"forked"`) {
		t.Fatalf("missing parent did not return forked verdict: %d %s", resp.Code, resp.Body.String())
	}
	u, err := sv.Update.Get("u-missing-parent")
	if err != nil {
		t.Fatal(err)
	}
	if u.State != model.UpdateStateForked {
		t.Fatalf("missing parent update state = %s", u.State)
	}
}
