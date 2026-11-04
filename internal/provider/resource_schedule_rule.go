package provider

import (
	"context"

	"github.com/OnCallAlert/terraform-provider-oncallalert/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.ResourceWithConfigure   = (*scheduleRuleResource)(nil)
	_ resource.ResourceWithImportState = (*scheduleRuleResource)(nil)
)

type scheduleRuleResource struct{ base }

func newScheduleRuleResource() resource.Resource { return &scheduleRuleResource{} }

type scheduleRuleModel struct {
	ID         types.String `tfsdk:"id"`
	ScheduleID types.String `tfsdk:"schedule_id"`
	TargetType types.String `tfsdk:"target_type"`
	TargetID   types.String `tfsdk:"target_id"`
	Start      types.String `tfsdk:"start"`
	End        types.String `tfsdk:"end"`
	Weekdays   types.Set    `tfsdk:"weekdays"`
}

func (r *scheduleRuleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_schedule_rule"
}

func (r *scheduleRuleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A rule that puts a user or rotation on call for a schedule during a daily time window. " +
			"Import with `<schedule_id>/<rule_id>`.",
		Attributes: map[string]schema.Attribute{
			"id":          idAttribute(),
			"schedule_id": schema.StringAttribute{Required: true, PlanModifiers: requiresReplace(), Description: "Schedule ID."},
			"target_type": schema.StringAttribute{
				Required: true, Description: "`user` or `rotation`.",
				Validators: []validator.String{oneOf{"user", "rotation"}},
			},
			"target_id": schema.StringAttribute{Required: true, Description: "ID of the user or rotation."},
			"start":     schema.StringAttribute{Required: true, Description: "Daily start time, `HH:MM` (schedule time zone)."},
			"end":       schema.StringAttribute{Required: true, Description: "Daily end time, `HH:MM`."},
			"weekdays": schema.SetAttribute{
				Optional: true, Computed: true, ElementType: types.StringType,
				Default:     setdefault.StaticValue(weekdaySet(allWeekdays)),
				Description: "Days the rule applies: any of `sun`, `mon`, `tue`, `wed`, `thu`, `fri`, `sat`. Default all.",
			},
		},
	}
}

func (m scheduleRuleModel) toClient(ctx context.Context, diags *diag.Diagnostics) client.ScheduleRule {
	return client.ScheduleRule{
		ID: m.ID.ValueString(), ScheduleID: m.ScheduleID.ValueString(),
		Start: m.Start.ValueString(), End: m.End.ValueString(),
		WeekdayFilter: filterFromNames(setStrings(ctx, m.Weekdays, diags)),
		Target:        client.Target{ID: m.TargetID.ValueString(), Type: m.TargetType.ValueString()},
	}
}

func (r *scheduleRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan scheduleRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	rule := plan.toClient(ctx, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	id, err := r.c.CreateScheduleRule(ctx, rule)
	if err != nil {
		apiError(&resp.Diagnostics, "create schedule rule", err)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
	r.readInto(ctx, plan.ScheduleID.ValueString(), id, &plan, &resp.State, &resp.Diagnostics)
}

func (r *scheduleRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state scheduleRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.readInto(ctx, state.ScheduleID.ValueString(), state.ID.ValueString(), &state, &resp.State, &resp.Diagnostics)
}

func (r *scheduleRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state scheduleRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	plan.ID = state.ID
	rule := plan.toClient(ctx, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.c.UpdateScheduleRule(ctx, rule); err != nil {
		apiError(&resp.Diagnostics, "update schedule rule", err)
		return
	}
	r.readInto(ctx, plan.ScheduleID.ValueString(), plan.ID.ValueString(), &plan, &resp.State, &resp.Diagnostics)
}

func (r *scheduleRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state scheduleRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.c.DeleteScheduleRule(ctx, state.ID.ValueString()); err != nil {
		apiError(&resp.Diagnostics, "delete schedule rule", err)
	}
}

func (r *scheduleRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	scheduleID, ruleID, err := splitImportID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("schedule_id"), scheduleID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), ruleID)...)
}

func (r *scheduleRuleResource) readInto(ctx context.Context, scheduleID, id string, prior *scheduleRuleModel, st *tfsdk.State, diags *diag.Diagnostics) {
	rule, err := r.c.GetScheduleRule(ctx, scheduleID, id)
	if err != nil {
		apiError(diags, "read schedule rule", err)
		return
	}
	if rule == nil {
		st.RemoveResource(ctx)
		return
	}
	m := scheduleRuleToModel(rule, scheduleID, prior)
	diags.Append(st.Set(ctx, &m)...)
}

func scheduleRuleToModel(r *client.ScheduleRule, scheduleID string, prior *scheduleRuleModel) scheduleRuleModel {
	m := scheduleRuleModel{
		ID: types.StringValue(r.ID), ScheduleID: types.StringValue(scheduleID),
		TargetType: types.StringValue(r.Target.Type), TargetID: types.StringValue(r.Target.ID),
		Start: types.StringValue(r.Start), End: types.StringValue(r.End),
		Weekdays: weekdaySet(namesFromFilter(r.WeekdayFilter)),
	}
	if prior != nil {
		m.Start = keepEqualClock(r.Start, prior.Start)
		m.End = keepEqualClock(r.End, prior.End)
	}
	return m
}
