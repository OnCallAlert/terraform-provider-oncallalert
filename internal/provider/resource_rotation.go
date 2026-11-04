package provider

import (
	"context"

	"github.com/OnCallAlert/terraform-provider-oncallalert/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.ResourceWithConfigure   = (*rotationResource)(nil)
	_ resource.ResourceWithImportState = (*rotationResource)(nil)
)

type rotationResource struct{ base }

func newRotationResource() resource.Resource { return &rotationResource{} }

type rotationModel struct {
	ID          types.String      `tfsdk:"id"`
	Name        types.String      `tfsdk:"name"`
	Description types.String      `tfsdk:"description"`
	TimeZone    types.String      `tfsdk:"time_zone"`
	Start       timetypes.RFC3339 `tfsdk:"start"`
	Type        types.String      `tfsdk:"type"`
	ShiftLength types.Int64       `tfsdk:"shift_length"`
	UserIDs     []types.String    `tfsdk:"user_ids"`
}

func (r *rotationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_rotation"
}

func (r *rotationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "An on-call rotation: users take turns in shifts.",
		Attributes: map[string]schema.Attribute{
			"id":          idAttribute(),
			"name":        schema.StringAttribute{Required: true, Description: "Unique name."},
			"description": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("")},
			"time_zone":   schema.StringAttribute{Required: true, Description: "IANA time zone, e.g. `America/New_York`. Prefer canonical names (`Etc/UTC`, not `UTC`): the server stores aliases under their canonical name, so an imported object with an alias in config shows one no-op update."},
			"start": schema.StringAttribute{
				Required: true, Description: "When the first shift starts (RFC3339).",
				CustomType: timetypes.RFC3339Type{},
			},
			"type": schema.StringAttribute{
				Required: true, Description: "Shift unit: `monthly`, `weekly`, `daily` or `hourly`.",
				Validators: []validator.String{oneOf{"monthly", "weekly", "daily", "hourly"}},
			},
			"shift_length": schema.Int64Attribute{
				Optional: true, Computed: true, Default: int64default.StaticInt64(1),
				Description: "Number of `type` units per shift. Default 1.",
			},
			"user_ids": schema.ListAttribute{
				Optional: true, ElementType: types.StringType,
				Description: "User IDs in rotation order.",
			},
		},
	}
}

func (m rotationModel) toClient() client.Rotation {
	r := client.Rotation{
		ID: m.ID.ValueString(), Name: m.Name.ValueString(), Description: m.Description.ValueString(),
		TimeZone: m.TimeZone.ValueString(), Start: m.Start.ValueString(), Type: m.Type.ValueString(),
		ShiftLength: int(m.ShiftLength.ValueInt64()),
	}
	for _, u := range m.UserIDs {
		r.UserIDs = append(r.UserIDs, u.ValueString())
	}
	return r
}

func (r *rotationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan rotationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, err := r.c.CreateRotation(ctx, plan.toClient())
	if err != nil {
		apiError(&resp.Diagnostics, "create rotation", err)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
	r.readInto(ctx, id, &plan, &resp.State, &resp.Diagnostics)
}

func (r *rotationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state rotationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.readInto(ctx, state.ID.ValueString(), &state, &resp.State, &resp.Diagnostics)
}

func (r *rotationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state rotationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = state.ID
	if err := r.c.UpdateRotation(ctx, plan.toClient()); err != nil {
		apiError(&resp.Diagnostics, "update rotation", err)
		return
	}
	r.readInto(ctx, plan.ID.ValueString(), &plan, &resp.State, &resp.Diagnostics)
}

func (r *rotationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state rotationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.c.DeleteAll(ctx, "rotation", state.ID.ValueString()); err != nil {
		apiError(&resp.Diagnostics, "delete rotation", err)
	}
}

func (r *rotationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *rotationResource) readInto(ctx context.Context, id string, prior *rotationModel, st *tfsdk.State, diags *diag.Diagnostics) {
	rot, err := r.c.GetRotation(ctx, id)
	if err != nil {
		apiError(diags, "read rotation", err)
		return
	}
	if rot == nil {
		st.RemoveResource(ctx)
		return
	}
	m := rotationToModel(rot, prior)
	diags.Append(st.Set(ctx, &m)...)
}

func rotationToModel(r *client.Rotation, prior *rotationModel) rotationModel {
	m := rotationModel{
		ID: types.StringValue(r.ID), Name: types.StringValue(r.Name), Description: types.StringValue(r.Description),
		TimeZone: types.StringValue(r.TimeZone), Type: types.StringValue(r.Type),
		ShiftLength: types.Int64Value(int64(r.ShiftLength)),
		Start:       timetypes.RFC3339{StringValue: types.StringValue(r.Start)},
	}
	if prior != nil {
		m.Start = timetypes.RFC3339{StringValue: keepEqualTime(r.Start, prior.Start.StringValue)}
		m.TimeZone = keepEqualTZ(r.TimeZone, prior.TimeZone)
	}
	for _, u := range r.UserIDs {
		m.UserIDs = append(m.UserIDs, types.StringValue(u))
	}
	if len(m.UserIDs) == 0 && prior != nil && prior.UserIDs != nil {
		m.UserIDs = []types.String{} // explicit empty list stays empty, not null
	}
	return m
}
