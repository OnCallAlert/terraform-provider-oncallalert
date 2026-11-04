package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// allWeekdays lists weekday names in WeekdayFilter order (index 0 = Sunday).
var allWeekdays = []string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}

// keepEqualTime returns the prior string when server and prior are the same instant
// in different RFC3339 spellings (e.g. "+02:00" vs "Z"); otherwise the server value.
func keepEqualTime(server string, prior types.String) types.String {
	if prior.IsNull() || prior.IsUnknown() {
		return types.StringValue(server)
	}
	a, errA := time.Parse(time.RFC3339, server)
	b, errB := time.Parse(time.RFC3339, prior.ValueString())
	if errA == nil && errB == nil && a.Equal(b) {
		return prior
	}
	return types.StringValue(server)
}

// keepEqualTZ keeps the prior time zone name when the server returns another name for the
// same zone (e.g. "UTC" is stored as "Etc/UTC"). Zones count as equal when their UTC offsets
// match at every sampled instant over three years, which covers DST rules.
func keepEqualTZ(server string, prior types.String) types.String {
	if prior.IsNull() || prior.IsUnknown() || prior.ValueString() == server {
		return types.StringValue(server)
	}
	a, errA := time.LoadLocation(server)
	b, errB := time.LoadLocation(prior.ValueString())
	if errA != nil || errB != nil {
		return types.StringValue(server)
	}
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	for t := start; t.Year() < 2028; t = t.Add(6 * time.Hour) {
		_, offA := t.In(a).Zone()
		_, offB := t.In(b).Zone()
		if offA != offB {
			return types.StringValue(server)
		}
	}
	return prior
}

// keepEqualClock is keepEqualTime for "HH:MM" values: "09:00:00" equals "09:00".
func keepEqualClock(server string, prior types.String) types.String {
	if !prior.IsNull() && !prior.IsUnknown() && server == prior.ValueString()+":00" {
		return prior
	}
	return types.StringValue(server)
}

// optString maps "" to null (used for optional attributes the server returns as "").
func optString(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}

// filterFromNames converts weekday names to a 7-bool filter. Unknown names are ignored.
func filterFromNames(names []string) []bool {
	f := make([]bool, 7)
	for _, n := range names {
		for i, d := range allWeekdays {
			if strings.EqualFold(n, d) {
				f[i] = true
			}
		}
	}
	return f
}

// namesFromFilter converts a weekday filter to names in Sunday-first order.
func namesFromFilter(f []bool) []string {
	var out []string
	for i, on := range f {
		if on && i < len(allWeekdays) {
			out = append(out, allWeekdays[i])
		}
	}
	return out
}

func weekdaySet(names []string) types.Set {
	vals := make([]types.String, 0, len(names))
	for _, n := range names {
		vals = append(vals, types.StringValue(n))
	}
	s, _ := types.SetValueFrom(context.Background(), types.StringType, vals)
	return s
}

func setStrings(ctx context.Context, s types.Set, diags *diag.Diagnostics) []string {
	var out []string
	diags.Append(s.ElementsAs(ctx, &out, false)...)
	return out
}

// splitImportID splits "<a>/<b>" on the first "/".
func splitImportID(id string) (string, string, error) {
	a, b, ok := strings.Cut(id, "/")
	if !ok || a == "" || b == "" {
		return "", "", fmt.Errorf("expected import ID in the form <parent_id>/<id>, got %q", id)
	}
	return a, b, nil
}

// oneOf validates that a string is one of the allowed values.
type oneOf []string

var _ validator.String = oneOf(nil)

func (v oneOf) Description(context.Context) string {
	return "value must be one of: " + strings.Join(v, ", ")
}
func (v oneOf) MarkdownDescription(ctx context.Context) string { return v.Description(ctx) }

func (v oneOf) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	for _, s := range v {
		if req.ConfigValue.ValueString() == s {
			return
		}
	}
	resp.Diagnostics.AddAttributeError(req.Path, "Invalid value",
		fmt.Sprintf("%q is not valid; %s", req.ConfigValue.ValueString(), v.Description(context.Background())))
}
