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
	_ resource.ResourceWithConfigure   = (*serviceResource)(nil)
	_ resource.ResourceWithImportState = (*serviceResource)(nil)
)

type serviceResource struct{ base }

func newServiceResource() resource.Resource { return &serviceResource{} }

type serviceModel struct {
	ID                 types.String `tfsdk:"id"`
	Name               types.String `tfsdk:"name"`
	Description        types.String `tfsdk:"description"`
	EscalationPolicyID types.String `tfsdk:"escalation_policy_id"`
}

func (r *serviceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service"
}

func (r *serviceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A service that receives alerts and escalates them with an escalation policy.",
		Attributes: map[string]schema.Attribute{
			"id":                   idAttribute(),
			"name":                 schema.StringAttribute{Required: true, Description: "Unique name."},
			"description":          schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("")},
			"escalation_policy_id": schema.StringAttribute{Required: true, Description: "ID of the escalation policy."},
		},
	}
}

func (r *serviceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan serviceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, err := r.c.CreateService(ctx, plan.Name.ValueString(), plan.Description.ValueString(), plan.EscalationPolicyID.ValueString())
	if err != nil {
		apiError(&resp.Diagnostics, "create service", err)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
	r.readInto(ctx, id, &resp.State, &resp.Diagnostics)
}

func (r *serviceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state serviceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.readInto(ctx, state.ID.ValueString(), &resp.State, &resp.Diagnostics)
}

func (r *serviceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state serviceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.c.UpdateService(ctx, state.ID.ValueString(), plan.Name.ValueString(), plan.Description.ValueString(), plan.EscalationPolicyID.ValueString())
	if err != nil {
		apiError(&resp.Diagnostics, "update service", err)
		return
	}
	r.readInto(ctx, state.ID.ValueString(), &resp.State, &resp.Diagnostics)
}

func (r *serviceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state serviceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.c.DeleteAll(ctx, "service", state.ID.ValueString()); err != nil {
		apiError(&resp.Diagnostics, "delete service", err)
	}
}

func (r *serviceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *serviceResource) readInto(ctx context.Context, id string, st *tfsdk.State, diags *diag.Diagnostics) {
	s, err := r.c.GetService(ctx, id)
	if err != nil {
		apiError(diags, "read service", err)
		return
	}
	if s == nil {
		st.RemoveResource(ctx)
		return
	}
	diags.Append(st.Set(ctx, serviceToModel(s))...)
}

func serviceToModel(s *client.Service) *serviceModel {
	return &serviceModel{
		ID: types.StringValue(s.ID), Name: types.StringValue(s.Name),
		Description:        types.StringValue(s.Description),
		EscalationPolicyID: types.StringValue(s.EscalationPolicyID),
	}
}
