package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSourceWithConfigure = (*userDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*namedDataSource)(nil)
)

type userDataSource struct{ dsBase }

func newUserDataSource() datasource.DataSource { return &userDataSource{} }

type userDSModel struct {
	Email types.String `tfsdk:"email"`
	ID    types.String `tfsdk:"id"`
	Name  types.String `tfsdk:"name"`
	Role  types.String `tfsdk:"role"`
}

func (d *userDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (d *userDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Looks up a user by email address.",
		Attributes: map[string]schema.Attribute{
			"email": schema.StringAttribute{Required: true},
			"id":    schema.StringAttribute{Computed: true},
			"name":  schema.StringAttribute{Computed: true},
			"role":  schema.StringAttribute{Computed: true, Description: "The user's role: `viewer`, `user`, `admin` or `owner` (`viewer` and `owner` need server v2.1.0 or newer). Viewers are read-only, and so are their API tokens."},
		},
	}
}

func (d *userDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m userDSModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	u, err := d.c.FindUserByEmail(ctx, m.Email.ValueString())
	if err != nil {
		apiError(&resp.Diagnostics, "look up user", err)
		return
	}
	m.ID, m.Name, m.Role = types.StringValue(u.ID), types.StringValue(u.Name), types.StringValue(u.Role)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

// namedDataSource looks up one object by name. kind is the Terraform suffix
// (e.g. "escalation_policy"); the server kind uses spaces ("escalation policy").
type namedDataSource struct {
	dsBase
	kind       string
	serverKind string
}

func newNamedDataSource(kind string) func() datasource.DataSource {
	serverKind := map[string]string{"escalation_policy": "escalation policy"}[kind]
	if serverKind == "" {
		serverKind = kind
	}
	return func() datasource.DataSource { return &namedDataSource{kind: kind, serverKind: serverKind} }
}

type namedDSModel struct {
	Name        types.String `tfsdk:"name"`
	ID          types.String `tfsdk:"id"`
	Description types.String `tfsdk:"description"`
}

func (d *namedDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + d.kind
}

func (d *namedDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Looks up a " + d.serverKind + " by exact name (case-insensitive).",
		Attributes: map[string]schema.Attribute{
			"name":        schema.StringAttribute{Required: true},
			"id":          schema.StringAttribute{Computed: true},
			"description": schema.StringAttribute{Computed: true},
		},
	}
}

func (d *namedDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m namedDSModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	n, err := d.c.FindByName(ctx, d.serverKind, m.Name.ValueString())
	if err != nil {
		apiError(&resp.Diagnostics, "look up "+d.serverKind, err)
		return
	}
	m.ID, m.Description = types.StringValue(n.ID), types.StringValue(n.Description)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
