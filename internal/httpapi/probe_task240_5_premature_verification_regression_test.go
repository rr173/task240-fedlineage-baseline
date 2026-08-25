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

func TestTask240Bug05OpenRoundCannotBeVerified(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/verify.db")
	if err != nil { t.Fatal(err) }
	defer st.Close()
	sv := service.New(st)
	if _, err := sv.Node.Register("m-root", "", "r0", "root", 8); err != nil { t.Fatal(err) }
	if _, err := sv.Node.Confirm("m-root"); err != nil { t.Fatal(err) }
	if _, err := sv.Round.Register("r1", "", 8); err != nil { t.Fatal(err) }
	if _, err := sv.Round.Open("r1"); err != nil { t.Fatal(err) }
	if _, err := sv.Update.Receive(&model.ClientUpdate{ID: "u1", RoundID: "r1", ClientID: "c1", ParentModel: "m-root", ParamDigest: "root", Dimension: 8}); err != nil { t.Fatal(err) }
	h := httpapi.New(sv).Handler()
	body, _ := json.Marshal(map[string]string{"id": "u1"})
	req := httptest.NewRequest(http.MethodPost, "/api/lineage/verify", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	h.ServeHTTP(resp, req)
	if resp.Code != http.StatusConflict { t.Fatalf("open round verification returned HTTP %d: %s", resp.Code, resp.Body.String()) }
	u, err := sv.Update.Get("u1")
	if err != nil { t.Fatal(err) }
	if u.State != model.UpdateStateNew { t.Fatalf("open-round verification changed update state to %s", u.State) }
}
