package provider

import (
	"context"

	"github.com/OnCallAlert/terraform-provider-oncallalert/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.ResourceWithConfigure   = (*scheduleResource)(nil)
	_ resource.ResourceWithImportState = (*scheduleResource)(nil)
)

type scheduleResource struct{ base }

func newScheduleResource() resource.Resource { return &scheduleResource{} }

type scheduleModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	TimeZone    types.String `tfsdk:"time_zone"`
}

func (r *scheduleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_schedule"
}

func (r *scheduleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "An on-call schedule. Add who is on call with `oncallalert_schedule_rule`.",
		Attributes: map[string]schema.Attribute{
			"id":          idAttribute(),
			"name":        schema.StringAttribute{Required: true, Description: "Unique name."},
			"description": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("")},
			"time_zone":   schema.StringAttribute{Required: true, Description: "IANA time zone, e.g. `America/New_York`. Prefer canonical names (`Etc/UTC`, not `UTC`): the server stores aliases under their canonical name, so an imported object with an alias in config shows one no-op update."},
		},
	}
}

func (r *scheduleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan scheduleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, err := r.c.CreateSchedule(ctx, plan.Name.ValueString(), plan.Description.ValueString(), plan.TimeZone.ValueString())
	if err != nil {
		apiError(&resp.Diagnostics, "create schedule", err)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
	r.readInto(ctx, id, plan.TimeZone, &resp.State, &resp.Diagnostics)
}

func (r *scheduleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state scheduleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.readInto(ctx, state.ID.ValueString(), state.TimeZone, &resp.State, &resp.Diagnostics)
}

func (r *scheduleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state scheduleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.c.UpdateSchedule(ctx, state.ID.ValueString(), plan.Name.ValueString(), plan.Description.ValueString(), plan.TimeZone.ValueString())
	if err != nil {
		apiError(&resp.Diagnostics, "update schedule", err)
		return
	}
	r.readInto(ctx, state.ID.ValueString(), plan.TimeZone, &resp.State, &resp.Diagnostics)
}

func (r *scheduleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state scheduleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.c.DeleteAll(ctx, "schedule", state.ID.ValueString()); err != nil {
		apiError(&resp.Diagnostics, "delete schedule", err)
	}
}

func (r *scheduleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *scheduleResource) readInto(ctx context.Context, id string, priorTZ types.String, st *tfsdk.State, diags *diag.Diagnostics) {
	s, err := r.c.GetSchedule(ctx, id)
	if err != nil {
		apiError(diags, "read schedule", err)
		return
	}
	if s == nil {
		st.RemoveResource(ctx)
		return
	}
	diags.Append(st.Set(ctx, scheduleToModel(s, priorTZ))...)
}

func scheduleToModel(s *client.Schedule, priorTZ types.String) *scheduleModel {
	return &scheduleModel{
		ID: types.StringValue(s.ID), Name: types.StringValue(s.Name),
		Description: types.StringValue(s.Description), TimeZone: keepEqualTZ(s.TimeZone, priorTZ),
	}
}
