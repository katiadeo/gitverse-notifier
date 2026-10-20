package gitverse

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-resty/resty/v2"
	"github.com/stretchr/testify/require"
)

func TestFindPullRequestByTitle_SkipsBadStateAndPaginates(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		state := r.URL.Query().Get("state")
		page := r.URL.Query().Get("page")
		w.Header().Set("Content-Type", "application/json")
		if state == "open" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"message":"unknown state"}`))
			return
		}
		if state == "opened" && page == "1" {
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"number": 1, "title": "other PR"},
			})
			return
		}
		if state == "opened" && page == "2" {
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"number": 412, "title": "APP-1183 [FE] Доработать форму"},
			})
			return
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{})
	}))
	t.Cleanup(ts.Close)

	client := &Client{resty: resty.New().SetBaseURL(ts.URL)}
	pr, err := client.FindPullRequestByTitle(context.Background(), "org/app", "APP-1183 [FE] Доработать форму")
	require.NoError(t, err)
	require.NotNil(t, pr)
	require.Equal(t, 412, pr.Number)
}
