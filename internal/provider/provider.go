// Package provider implements the OnCallAlert Terraform provider.
package provider

import (
	"context"
	"os"
	"time"

	"github.com/OnCallAlert/terraform-provider-oncallalert/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ provider.Provider = (*oncallalertProvider)(nil)

type oncallalertProvider struct{ version string }

type providerModel struct {
	URL                types.String `tfsdk:"url"`
	Token              types.String `tfsdk:"token"`
	InsecureSkipVerify types.Bool   `tfsdk:"insecure_skip_verify"`
	TimeoutSeconds     types.Int64  `tfsdk:"timeout_seconds"`
}

// New returns a constructor for the provider at the given version.
func New(version string) func() provider.Provider {
	return func() provider.Provider { return &oncallalertProvider{version: version} }
}

func (p *oncallalertProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "oncallalert"
	resp.Version = p.version
}

func (p *oncallalertProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manage a self-hosted OnCallAlert server (escalation policies, schedules, rotations, services, ...) as code.",
		Attributes: map[string]schema.Attribute{
			"url": schema.StringAttribute{
				Optional:    true,
				Description: "Base URL of your OnCallAlert server, e.g. `https://oncall.example.com`. Can also be set with `ONCALLALERT_URL`.",
			},
			"token": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "API token with Admin scope (profile page -> API Tokens). Can also be set with `ONCALLALERT_TOKEN`.",
			},
			"insecure_skip_verify": schema.BoolAttribute{
				Optional:    true,
				Description: "Skip TLS certificate checks. Only for test servers with self-signed certificates.",
			},
			"timeout_seconds": schema.Int64Attribute{
				Optional:    true,
				Description: "HTTP timeout per API request, in seconds. Default 30.",
			},
		},
	}
}

func (p *oncallalertProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if cfg.URL.IsUnknown() || cfg.Token.IsUnknown() {
		resp.Diagnostics.AddWarning("Provider not configured yet", "url or token is unknown until apply; API calls are deferred.")
		return
	}
	url := stringOr(cfg.URL, os.Getenv("ONCALLALERT_URL"))
	token := stringOr(cfg.Token, os.Getenv("ONCALLALERT_TOKEN"))
	if url == "" {
		resp.Diagnostics.AddAttributeError(path.Root("url"), "Missing server URL", "Set `url` or ONCALLALERT_URL.")
	}
	if token == "" {
		resp.Diagnostics.AddAttributeError(path.Root("token"), "Missing API token", "Set `token` or ONCALLALERT_TOKEN.")
	}
	if resp.Diagnostics.HasError() {
		return
	}
	c, err := client.New(client.Config{
		URL:                url,
		Token:              token,
		UserAgent:          "terraform-provider-oncallalert/" + p.version,
		Timeout:            time.Duration(cfg.TimeoutSeconds.ValueInt64()) * time.Second,
		InsecureSkipVerify: cfg.InsecureSkipVerify.ValueBool(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Invalid provider configuration", err.Error())
		return
	}
	resp.ResourceData = c
	resp.DataSourceData = c
}

func (p *oncallalertProvider) Resources(context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		newEscalationPolicyResource,
		newRotationResource,
		newScheduleResource,
		newScheduleRuleResource,
		newServiceResource,
		newServiceLabelResource,
		newIntegrationKeyResource,
		newUserOverrideResource,
	}
}

func (p *oncallalertProvider) DataSources(context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		newUserDataSource,
		newNamedDataSource("service"),
		newNamedDataSource("schedule"),
		newNamedDataSource("escalation_policy"),
		newNamedDataSource("rotation"),
	}
}

func stringOr(v types.String, fallback string) string {
	if v.IsNull() || v.ValueString() == "" {
		return fallback
	}
	return v.ValueString()
}
