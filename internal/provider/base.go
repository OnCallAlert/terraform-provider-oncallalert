package provider

import (
	"context"
	"fmt"

	"github.com/OnCallAlert/terraform-provider-oncallalert/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

// base holds the API client; every resource and data source embeds it.
type base struct{ c *client.Client }

func (b *base) configure(data any, diags *diag.Diagnostics) {
	if data == nil {
		return // provider not configured yet (e.g. during validate)
	}
	c, ok := data.(*client.Client)
	if !ok {
		diags.AddError("Unexpected provider data", fmt.Sprintf("expected *client.Client, got %T", data))
		return
	}
	b.c = c
}

// Configure implements resource.ResourceWithConfigure.
func (b *base) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	b.configure(req.ProviderData, &resp.Diagnostics)
}

// dsBase is base for data sources.
type dsBase struct{ base }

// Configure implements datasource.DataSourceWithConfigure.
func (b *dsBase) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	b.configure(req.ProviderData, &resp.Diagnostics)
}

// idAttribute is the computed "id" attribute shared by all resources.
func idAttribute() schema.StringAttribute {
	return schema.StringAttribute{
		Computed:      true,
		Description:   "Server-assigned ID.",
		PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
	}
}

// requiresReplace marks a string attribute that cannot be changed in place.
func requiresReplace() []planmodifier.String {
	return []planmodifier.String{stringplanmodifier.RequiresReplace()}
}

func apiError(diags *diag.Diagnostics, action string, err error) {
	diags.AddError("OnCallAlert API error", fmt.Sprintf("%s: %s", action, err))
}
