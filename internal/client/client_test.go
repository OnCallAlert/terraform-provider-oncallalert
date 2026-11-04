package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newTestClient starts a server running h and returns a client pointed at it.
func newTestClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c, err := New(Config{URL: srv.URL, Token: "tok", UserAgent: "ua"})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func reply(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(body))
}

func TestDo_SendsAuthHeaderAndJSONBody(t *testing.T) {
	t.Parallel()
	var gotAuth, gotPath string
	var gotBody gqlRequest
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotPath = r.Header.Get("Authorization"), r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		reply(w, `{"data":{"x":1}}`)
	})
	var out struct{ X int }
	if err := c.Do(context.Background(), "query{x}", map[string]any{"a": "b"}, &out); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer tok" || gotPath != "/api/graphql" {
		t.Errorf("auth=%q path=%q", gotAuth, gotPath)
	}
	if gotBody.Query != "query{x}" || gotBody.Variables["a"] != "b" || out.X != 1 {
		t.Errorf("body=%+v out=%+v", gotBody, out)
	}
}

func TestDo_GraphQLErrorReturnsError(t *testing.T) {
	t.Parallel()
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		reply(w, `{"errors":[{"message":"boom"},{"message":"bang"}]}`)
	})
	err := c.Do(context.Background(), "q", nil, nil)
	ge, ok := err.(*Error)
	if !ok || len(ge.Messages) != 2 || !strings.Contains(err.Error(), "boom; bang") {
		t.Fatalf("got %v", err)
	}
}

func TestDo_UnauthorizedMentionsToken(t *testing.T) {
	t.Parallel()
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusUnauthorized) })
	err := c.Do(context.Background(), "q", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "token") {
		t.Fatalf("got %v", err)
	}
}

func TestIsNotFound(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"not found", &Error{Messages: []string{"Rotation not found"}}, true},
		{"does not exist", &Error{Messages: []string{"x", "It Does Not Exist"}}, true},
		{"other", &Error{Messages: []string{"boom"}}, false},
		{"non graphql", http.ErrAbortHandler, false},
		{"nil", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IsNotFound(tt.err); got != tt.want {
				t.Errorf("got %v want %v", got, tt.want)
			}
		})
	}
}

func TestDeleteAll_IgnoresNotFound(t *testing.T) {
	t.Parallel()
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		reply(w, `{"errors":[{"message":"not found"}]}`)
	})
	if err := c.DeleteAll(context.Background(), "service", "1"); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteAll_ReturnsOtherErrors(t *testing.T) {
	t.Parallel()
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		reply(w, `{"errors":[{"message":"denied"}]}`)
	})
	if err := c.DeleteAll(context.Background(), "service", "1"); err == nil {
		t.Fatal("expected error")
	}
}

func TestPickOne(t *testing.T) {
	t.Parallel()
	key := func(s string) string { return s }
	tests := []struct {
		name    string
		nodes   []string
		want    string
		wantErr string
	}{
		{"none", []string{"b"}, "a", "no thing found"},
		{"one case-insensitive", []string{"A", "b"}, "a", ""},
		{"many", []string{"a", "A"}, "a", "2 things match"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := pickOne("thing", tt.want, tt.nodes, key)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err=%v want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil || *got != "A" {
				t.Fatalf("got %v, %v", got, err)
			}
		})
	}
}

func TestGetScheduleRule_FindsRuleAcrossTargets(t *testing.T) {
	t.Parallel()
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		reply(w, `{"data":{"schedule":{"id":"s","targets":[
			{"rules":[{"id":"r1"}]},{"rules":[{"id":"r2","start":"09:00"}]}]}}}`)
	})
	r, err := c.GetScheduleRule(context.Background(), "s", "r2")
	if err != nil || r == nil || r.Start != "09:00" {
		t.Fatalf("got %+v, %v", r, err)
	}
	r, err = c.GetScheduleRule(context.Background(), "s", "zzz")
	if err != nil || r != nil {
		t.Fatalf("missing rule: got %+v, %v", r, err)
	}
}

func TestGetUserOverride_SetsScheduleIDFromTarget(t *testing.T) {
	t.Parallel()
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		reply(w, `{"data":{"userOverride":{"id":"o","target":{"id":"sched","type":"schedule"}}}}`)
	})
	o, err := c.GetUserOverride(context.Background(), "o")
	if err != nil || o == nil || o.ScheduleID != "sched" {
		t.Fatalf("got %+v, %v", o, err)
	}
}

func TestGetUserOverride_NullReturnsNil(t *testing.T) {
	t.Parallel()
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		reply(w, `{"data":{"userOverride":null}}`)
	})
	o, err := c.GetUserOverride(context.Background(), "o")
	if err != nil || o != nil {
		t.Fatalf("got %+v, %v", o, err)
	}
}

func TestFindUserByEmail_PagesUntilMatch(t *testing.T) {
	t.Parallel()
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		var req gqlRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Variables["a"] == "" {
			reply(w, `{"data":{"users":{"nodes":[{"id":"1","email":"a@x.io"}],"pageInfo":{"hasNextPage":true,"endCursor":"c1"}}}}`)
			return
		}
		reply(w, `{"data":{"users":{"nodes":[{"id":"2","email":"Bob@X.io"}],"pageInfo":{"hasNextPage":false,"endCursor":"c2"}}}}`)
	})
	u, err := c.FindUserByEmail(context.Background(), "bob@x.io")
	if err != nil {
		t.Fatal(err)
	}
	if u.ID != "2" {
		t.Fatalf("got user %q, want 2", u.ID)
	}
}
