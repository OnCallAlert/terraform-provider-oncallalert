package provider

import (
	"context"

	"github.com/OnCallAlert/terraform-provider-oncallalert/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.ResourceWithConfigure   = (*serviceLabelResource)(nil)
	_ resource.ResourceWithImportState = (*serviceLabelResource)(nil)
)

type serviceLabelResource struct{ base }

func newServiceLabelResource() resource.Resource { return &serviceLabelResource{} }

type serviceLabelModel struct {
	ID        types.String `tfsdk:"id"`
	ServiceID types.String `tfsdk:"service_id"`
	Key       types.String `tfsdk:"key"`
	Value     types.String `tfsdk:"value"`
}

func (r *serviceLabelResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_label"
}

func (r *serviceLabelResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A label on a service. Import with `<service_id>/<key>`.",
		Attributes: map[string]schema.Attribute{
			"id":         schema.StringAttribute{Computed: true, Description: "`<service_id>/<key>`."},
			"service_id": schema.StringAttribute{Required: true, PlanModifiers: requiresReplace()},
			"key":        schema.StringAttribute{Required: true, PlanModifiers: requiresReplace(), Description: "Label key, e.g. `team/name`."},
			"value":      schema.StringAttribute{Required: true, Description: "Label value."},
		},
	}
}

func labelID(serviceID, key string) string { return serviceID + "/" + key }

func (r *serviceLabelResource) set(ctx context.Context, plan *serviceLabelModel, st *tfsdk.State, diags *diag.Diagnostics) {
	err := r.c.SetServiceLabel(ctx, plan.ServiceID.ValueString(), plan.Key.ValueString(), plan.Value.ValueString())
	if err != nil {
		apiError(diags, "set service label", err)
		return
	}
	plan.ID = types.StringValue(labelID(plan.ServiceID.ValueString(), plan.Key.ValueString()))
	r.readInto(ctx, plan, st, diags)
}

func (r *serviceLabelResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan serviceLabelModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.set(ctx, &plan, &resp.State, &resp.Diagnostics)
}

func (r *serviceLabelResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan serviceLabelModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.set(ctx, &plan, &resp.State, &resp.Diagnostics)
}

func (r *serviceLabelResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state serviceLabelModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.readInto(ctx, &state, &resp.State, &resp.Diagnostics)
}

func (r *serviceLabelResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state serviceLabelModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// An empty value removes the label.
	if err := r.c.SetServiceLabel(ctx, state.ServiceID.ValueString(), state.Key.ValueString(), ""); err != nil && !client.IsNotFound(err) {
		apiError(&resp.Diagnostics, "delete service label", err)
	}
}

func (r *serviceLabelResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	serviceID, key, err := splitImportID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("service_id"), serviceID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("key"), key)...)
}

func (r *serviceLabelResource) readInto(ctx context.Context, m *serviceLabelModel, st *tfsdk.State, diags *diag.Diagnostics) {
	svc, err := r.c.GetService(ctx, m.ServiceID.ValueString())
	if err != nil {
		apiError(diags, "read service label", err)
		return
	}
	value, ok := findLabel(svc, m.Key.ValueString())
	if !ok {
		st.RemoveResource(ctx)
		return
	}
	m.Value = types.StringValue(value)
	diags.Append(st.Set(ctx, m)...)
}

// findLabel returns the value of label key on svc (svc may be nil).
func findLabel(svc *client.Service, key string) (string, bool) {
	if svc == nil {
		return "", false
	}
	for _, l := range svc.Labels {
		if l.Key == key {
			return l.Value, true
		}
	}
	return "", false
}
