package provider

import (
	"reflect"
	"testing"

	"github.com/OnCallAlert/terraform-provider-oncallalert/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestEpToModel_SortsStepsByStepNumber(t *testing.T) {
	t.Parallel()
	ep := &client.EscalationPolicy{ID: "p", Steps: []client.EscalationPolicyStep{
		{ID: "b", StepNumber: 1}, {ID: "a", StepNumber: 0}, {ID: "c", StepNumber: 2},
	}}
	m := epToModel(ep, nil)
	var ids []string
	for _, s := range m.Steps {
		ids = append(ids, s.ID.ValueString())
	}
	if !reflect.DeepEqual(ids, []string{"a", "b", "c"}) {
		t.Errorf("got %v", ids)
	}
}

func TestEpToModel_EmptyStepsKeepsPriorNullness(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		prior    *escalationPolicyModel
		wantNull bool
	}{
		{"no prior", nil, true},
		{"prior null", &escalationPolicyModel{}, true},
		{"prior explicit empty", &escalationPolicyModel{Steps: []epStepModel{}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m := epToModel(&client.EscalationPolicy{ID: "p"}, tt.prior)
			if (m.Steps == nil) != tt.wantNull {
				t.Errorf("Steps nil=%v want %v", m.Steps == nil, tt.wantNull)
			}
		})
	}
}

func TestWeekdayConversion(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		names  []string
		filter []bool
	}{
		{"all", allWeekdays, []bool{true, true, true, true, true, true, true}},
		{"weekdays", []string{"mon", "tue", "wed", "thu", "fri"}, []bool{false, true, true, true, true, true, false}},
		{"sunday only", []string{"sun"}, []bool{true, false, false, false, false, false, false}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := filterFromNames(tt.names); !reflect.DeepEqual(got, tt.filter) {
				t.Errorf("filterFromNames=%v", got)
			}
			if got := namesFromFilter(tt.filter); !reflect.DeepEqual(got, tt.names) {
				t.Errorf("namesFromFilter=%v", got)
			}
		})
	}
}

func TestKeepEqualTime(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, server string
		prior        types.String
		want         string
	}{
		{"same instant different format keeps prior", "2026-01-01T07:00:00Z", types.StringValue("2026-01-01T09:00:00+02:00"), "2026-01-01T09:00:00+02:00"},
		{"different instant uses server", "2026-01-02T07:00:00Z", types.StringValue("2026-01-01T09:00:00+02:00"), "2026-01-02T07:00:00Z"},
		{"null prior uses server", "2026-01-01T07:00:00Z", types.StringNull(), "2026-01-01T07:00:00Z"},
		{"unparsable prior uses server", "2026-01-01T07:00:00Z", types.StringValue("nope"), "2026-01-01T07:00:00Z"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := keepEqualTime(tt.server, tt.prior).ValueString(); got != tt.want {
				t.Errorf("got %q want %q", got, tt.want)
			}
		})
	}
}

func TestKeepEqualClock(t *testing.T) {
	t.Parallel()
	if got := keepEqualClock("09:00:00", types.StringValue("09:00")).ValueString(); got != "09:00" {
		t.Errorf("got %q", got)
	}
	if got := keepEqualClock("10:00", types.StringValue("09:00")).ValueString(); got != "10:00" {
		t.Errorf("got %q", got)
	}
}

func TestSplitImportID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		id, a, b string
		wantErr  bool
	}{
		{"sched/rule", "sched", "rule", false},
		{"svc/team/name", "svc", "team/name", false},
		{"nosep", "", "", true},
		{"/x", "", "", true},
		{"x/", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			t.Parallel()
			a, b, err := splitImportID(tt.id)
			if (err != nil) != tt.wantErr || a != tt.a || b != tt.b {
				t.Errorf("got %q %q %v", a, b, err)
			}
		})
	}
}

func TestUserOverrideToModel_EmptyUserIDsBecomeNull(t *testing.T) {
	t.Parallel()
	m := userOverrideToModel(&client.UserOverride{ID: "o", AddUserID: "u"}, nil)
	if !m.RemoveUserID.IsNull() || m.AddUserID.ValueString() != "u" {
		t.Errorf("got %+v", m)
	}
}

func TestRotationToModel_EmptyUserIDsKeepPriorNullness(t *testing.T) {
	t.Parallel()
	if m := rotationToModel(&client.Rotation{}, &rotationModel{}); m.UserIDs != nil {
		t.Error("expected null user_ids")
	}
	if m := rotationToModel(&client.Rotation{}, &rotationModel{UserIDs: []types.String{}}); m.UserIDs == nil {
		t.Error("expected empty, non-null user_ids")
	}
}

func TestFindLabel(t *testing.T) {
	t.Parallel()
	svc := &client.Service{Labels: []client.Label{{Key: "a/b", Value: "v"}}}
	if v, ok := findLabel(svc, "a/b"); !ok || v != "v" {
		t.Errorf("got %q %v", v, ok)
	}
	if _, ok := findLabel(svc, "x"); ok {
		t.Error("unexpected match")
	}
	if _, ok := findLabel(nil, "a/b"); ok {
		t.Error("nil service matched")
	}
}

func TestKeepEqualTZ_KeepsPriorForAliasOfSameZone(t *testing.T) {
	t.Parallel()
	cases := []struct {
		server, prior, want string
	}{
		{"Etc/UTC", "UTC", "UTC"},
		{"Asia/Kolkata", "Asia/Calcutta", "Asia/Calcutta"},
		{"Europe/Berlin", "UTC", "Europe/Berlin"},
		{"America/New_York", "America/Chicago", "America/New_York"},
		{"Etc/UTC", "", "Etc/UTC"},
	}
	for _, c := range cases {
		prior := types.StringValue(c.prior)
		if c.prior == "" {
			prior = types.StringNull()
		}
		if got := keepEqualTZ(c.server, prior).ValueString(); got != c.want {
			t.Errorf("keepEqualTZ(%q, %q) = %q, want %q", c.server, c.prior, got, c.want)
		}
	}
}
