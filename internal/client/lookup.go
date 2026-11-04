package client

import (
	"context"
	"fmt"
	"strings"
)

// Named is a search result: an object with an ID and a name.
type Named struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// User is an OnCallAlert user.
type User struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

// searchQueries maps a kind to its search query. %s is replaced by the result fields.
var searchQueries = map[string]string{
	"service":           `query($s:String!){r:services(input:{search:$s,first:100}){nodes{%s}}}`,
	"schedule":          `query($s:String!){r:schedules(input:{search:$s,first:100}){nodes{%s}}}`,
	"escalation policy": `query($s:String!){r:escalationPolicies(input:{search:$s,first:100}){nodes{%s}}}`,
	"rotation":          `query($s:String!){r:rotations(input:{search:$s,first:100}){nodes{%s}}}`,
}

// FindByName returns the one object of kind ("service", "schedule", "escalation policy", "rotation")
// whose name equals name exactly (case-insensitive, as names are unique that way on the server).
func (c *Client) FindByName(ctx context.Context, kind, name string) (*Named, error) {
	q, ok := searchQueries[kind]
	if !ok {
		return nil, fmt.Errorf("unknown kind %q", kind)
	}
	var out struct {
		R struct{ Nodes []Named }
	}
	if err := c.Do(ctx, fmt.Sprintf(q, "id name description"), map[string]any{"s": name}, &out); err != nil {
		return nil, err
	}
	return pickOne(kind, name, out.R.Nodes, func(n Named) string { return n.Name })
}

// maxUserPages caps FindUserByEmail at 100 pages of 100 users.
const maxUserPages = 100

// FindUserByEmail returns the one user whose email equals email (case-insensitive).
// The server's user search matches names only, so this pages through all users.
func (c *Client) FindUserByEmail(ctx context.Context, email string) (*User, error) {
	var all []User
	after := ""
	for page := 0; page < maxUserPages; page++ {
		var out struct {
			Users struct {
				Nodes    []User
				PageInfo struct {
					HasNextPage bool
					EndCursor   string
				}
			}
		}
		err := c.Do(ctx, `query($a:String!){users(input:{first:100,after:$a}){nodes{id name email role} pageInfo{hasNextPage endCursor}}}`,
			map[string]any{"a": after}, &out)
		if err != nil {
			return nil, err
		}
		all = append(all, out.Users.Nodes...)
		if !out.Users.PageInfo.HasNextPage || out.Users.PageInfo.EndCursor == "" {
			break
		}
		after = out.Users.PageInfo.EndCursor
	}
	return pickOne("user", email, all, func(u User) string { return u.Email })
}

func pickOne[T any](kind, want string, nodes []T, key func(T) string) (*T, error) {
	var found []T
	for _, n := range nodes {
		if strings.EqualFold(key(n), want) {
			found = append(found, n)
		}
	}
	switch len(found) {
	case 0:
		return nil, fmt.Errorf("no %s found matching %q", kind, want)
	case 1:
		return &found[0], nil
	default:
		return nil, fmt.Errorf("%d %ss match %q; expected exactly one", len(found), kind, want)
	}
}
