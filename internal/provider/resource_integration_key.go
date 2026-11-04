package provider

import (
	"context"

	"github.com/OnCallAlert/terraform-provider-oncallalert/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.ResourceWithConfigure   = (*integrationKeyResource)(nil)
	_ resource.ResourceWithImportState = (*integrationKeyResource)(nil)
)

type integrationKeyResource struct{ base }

func newIntegrationKeyResource() resource.Resource { return &integrationKeyResource{} }

type integrationKeyModel struct {
	ID                 types.String `tfsdk:"id"`
	ServiceID          types.String `tfsdk:"service_id"`
	Type               types.String `tfsdk:"type"`
	Name               types.String `tfsdk:"name"`
	ExternalSystemName types.String `tfsdk:"external_system_name"`
	Href               types.String `tfsdk:"href"`
}

func (r *integrationKeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration_key"
}

func (r *integrationKeyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "An integration key that lets an external system send alerts to a service.",
		Attributes: map[string]schema.Attribute{
			"id":         idAttribute(),
			"service_id": schema.StringAttribute{Required: true, PlanModifiers: requiresReplace()},
			"type": schema.StringAttribute{
				Required: true, PlanModifiers: requiresReplace(),
				Description: "One of `generic`, `grafana`, `site24x7`, `prometheusAlertmanager`, `email`, `universal`.",
				Validators:  []validator.String{oneOf{"generic", "grafana", "site24x7", "prometheusAlertmanager", "email", "universal"}},
			},
			"name":                 schema.StringAttribute{Required: true},
			"external_system_name": schema.StringAttribute{Optional: true, Description: "Name shown for the sending system."},
			"href": schema.StringAttribute{
				Computed: true, Sensitive: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Description:   "Alert ingest URL (contains the secret key).",
			},
		},
	}
}

func (m integrationKeyModel) externalName() *string {
	if m.ExternalSystemName.IsNull() || m.ExternalSystemName.IsUnknown() {
		return nil
	}
	v := m.ExternalSystemName.ValueString()
	return &v
}

func (r *integrationKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan integrationKeyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	k, err := r.c.CreateIntegrationKey(ctx, client.IntegrationKey{
		ServiceID: plan.ServiceID.ValueString(), Type: plan.Type.ValueString(),
		Name: plan.Name.ValueString(), ExternalSystemName: plan.externalName(),
	})
	if err != nil {
		apiError(&resp.Diagnostics, "create integration key", err)
		return
	}
	m := integrationKeyToModel(k, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *integrationKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state integrationKeyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.readInto(ctx, state.ID.ValueString(), &state, &resp.State, &resp.Diagnostics)
}

func (r *integrationKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state integrationKeyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()
	if err := r.c.UpdateIntegrationKey(ctx, id, plan.Name.ValueString(), plan.externalName()); err != nil {
		apiError(&resp.Diagnostics, "update integration key", err)
		return
	}
	r.readInto(ctx, id, &plan, &resp.State, &resp.Diagnostics)
}

func (r *integrationKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state integrationKeyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.c.DeleteAll(ctx, "integrationKey", state.ID.ValueString()); err != nil {
		apiError(&resp.Diagnostics, "delete integration key", err)
	}
}

func (r *integrationKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *integrationKeyResource) readInto(ctx context.Context, id string, prior *integrationKeyModel, st *tfsdk.State, diags *diag.Diagnostics) {
	k, err := r.c.GetIntegrationKey(ctx, id)
	if err != nil {
		apiError(diags, "read integration key", err)
		return
	}
	if k == nil {
		st.RemoveResource(ctx)
		return
	}
	m := integrationKeyToModel(k, prior)
	diags.Append(st.Set(ctx, &m)...)
}

func integrationKeyToModel(k *client.IntegrationKey, prior *integrationKeyModel) integrationKeyModel {
	m := integrationKeyModel{
		ID: types.StringValue(k.ID), ServiceID: types.StringValue(k.ServiceID),
		Type: types.StringValue(k.Type), Name: types.StringValue(k.Name),
		Href:               types.StringValue(k.Href),
		ExternalSystemName: types.StringNull(),
	}
	if k.ExternalSystemName != nil && (*k.ExternalSystemName != "" || (prior != nil && !prior.ExternalSystemName.IsNull())) {
		m.ExternalSystemName = types.StringValue(*k.ExternalSystemName)
	}
	return m
}
