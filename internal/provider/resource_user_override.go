package provider

import (
	"context"

	"github.com/OnCallAlert/terraform-provider-oncallalert/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.ResourceWithConfigure      = (*userOverrideResource)(nil)
	_ resource.ResourceWithImportState    = (*userOverrideResource)(nil)
	_ resource.ResourceWithValidateConfig = (*userOverrideResource)(nil)
)

type userOverrideResource struct{ base }

func newUserOverrideResource() resource.Resource { return &userOverrideResource{} }

type userOverrideModel struct {
	ID           types.String      `tfsdk:"id"`
	ScheduleID   types.String      `tfsdk:"schedule_id"`
	Start        timetypes.RFC3339 `tfsdk:"start"`
	End          timetypes.RFC3339 `tfsdk:"end"`
	AddUserID    types.String      `tfsdk:"add_user_id"`
	RemoveUserID types.String      `tfsdk:"remove_user_id"`
}

func (r *userOverrideResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_override"
}

func (r *userOverrideResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A temporary override on a schedule: add a user, remove a user, or replace one with another. " +
			"Set `add_user_id`, `remove_user_id`, or both.",
		Attributes: map[string]schema.Attribute{
			"id":             idAttribute(),
			"schedule_id":    schema.StringAttribute{Required: true, PlanModifiers: requiresReplace()},
			"start":          schema.StringAttribute{Required: true, CustomType: timetypes.RFC3339Type{}, Description: "Override start (RFC3339)."},
			"end":            schema.StringAttribute{Required: true, CustomType: timetypes.RFC3339Type{}, Description: "Override end (RFC3339)."},
			"add_user_id":    schema.StringAttribute{Optional: true, Description: "User to put on call."},
			"remove_user_id": schema.StringAttribute{Optional: true, Description: "User to take off call."},
		},
	}
}

func (r *userOverrideResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var m userOverrideModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if m.AddUserID.IsNull() && m.RemoveUserID.IsNull() {
		resp.Diagnostics.AddError("Missing user", "At least one of `add_user_id` or `remove_user_id` is required.")
	}
}

func (m userOverrideModel) toClient() client.UserOverride {
	return client.UserOverride{
		ID: m.ID.ValueString(), ScheduleID: m.ScheduleID.ValueString(),
		Start: m.Start.ValueString(), End: m.End.ValueString(),
		AddUserID: m.AddUserID.ValueString(), RemoveUserID: m.RemoveUserID.ValueString(),
	}
}

func (r *userOverrideResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan userOverrideModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, err := r.c.CreateUserOverride(ctx, plan.toClient())
	if err != nil {
		apiError(&resp.Diagnostics, "create user override", err)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
	r.readInto(ctx, id, &plan, &resp.State, &resp.Diagnostics)
}

func (r *userOverrideResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state userOverrideModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.readInto(ctx, state.ID.ValueString(), &state, &resp.State, &resp.Diagnostics)
}

func (r *userOverrideResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state userOverrideModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = state.ID
	if err := r.c.UpdateUserOverride(ctx, plan.toClient()); err != nil {
		apiError(&resp.Diagnostics, "update user override", err)
		return
	}
	r.readInto(ctx, plan.ID.ValueString(), &plan, &resp.State, &resp.Diagnostics)
}

func (r *userOverrideResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state userOverrideModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.c.DeleteAll(ctx, "userOverride", state.ID.ValueString()); err != nil {
		apiError(&resp.Diagnostics, "delete user override", err)
	}
}

func (r *userOverrideResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *userOverrideResource) readInto(ctx context.Context, id string, prior *userOverrideModel, st *tfsdk.State, diags *diag.Diagnostics) {
	o, err := r.c.GetUserOverride(ctx, id)
	if err != nil {
		apiError(diags, "read user override", err)
		return
	}
	if o == nil {
		st.RemoveResource(ctx)
		return
	}
	m := userOverrideToModel(o, prior)
	diags.Append(st.Set(ctx, &m)...)
}

func userOverrideToModel(o *client.UserOverride, prior *userOverrideModel) userOverrideModel {
	m := userOverrideModel{
		ID: types.StringValue(o.ID), ScheduleID: types.StringValue(o.ScheduleID),
		Start: timetypes.RFC3339{StringValue: types.StringValue(o.Start)}, End: timetypes.RFC3339{StringValue: types.StringValue(o.End)},
		AddUserID: optString(o.AddUserID), RemoveUserID: optString(o.RemoveUserID),
	}
	if prior != nil {
		m.Start = timetypes.RFC3339{StringValue: keepEqualTime(o.Start, prior.Start.StringValue)}
		m.End = timetypes.RFC3339{StringValue: keepEqualTime(o.End, prior.End.StringValue)}
	}
	return m
}
