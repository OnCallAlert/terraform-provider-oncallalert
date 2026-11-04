package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

func TestProvider_SchemaIsValid(t *testing.T) {
	t.Parallel()
	var resp provider.SchemaResponse
	New("test")().Schema(context.Background(), provider.SchemaRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if diags := resp.Schema.ValidateImplementation(context.Background()); diags.HasError() {
		t.Fatal(diags)
	}
}

func TestProvider_ResourceSchemasAreValidAndUnique(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	seen := map[string]bool{}
	for _, newRes := range New("test")().Resources(ctx) {
		r := newRes()
		var meta resource.MetadataResponse
		r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "oncallalert"}, &meta)
		if seen[meta.TypeName] {
			t.Errorf("duplicate resource %s", meta.TypeName)
		}
		seen[meta.TypeName] = true
		var sr resource.SchemaResponse
		r.Schema(ctx, resource.SchemaRequest{}, &sr)
		if sr.Diagnostics.HasError() {
			t.Errorf("%s: %v", meta.TypeName, sr.Diagnostics)
		}
		if diags := sr.Schema.ValidateImplementation(ctx); diags.HasError() {
			t.Errorf("%s: %v", meta.TypeName, diags)
		}
	}
	if len(seen) != 8 {
		t.Errorf("expected 8 resources, got %d", len(seen))
	}
}

func TestProvider_DataSourceSchemasAreValidAndUnique(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	seen := map[string]bool{}
	for _, newDS := range New("test")().DataSources(ctx) {
		d := newDS()
		var meta datasource.MetadataResponse
		d.Metadata(ctx, datasource.MetadataRequest{ProviderTypeName: "oncallalert"}, &meta)
		if seen[meta.TypeName] {
			t.Errorf("duplicate data source %s", meta.TypeName)
		}
		seen[meta.TypeName] = true
		var sr datasource.SchemaResponse
		d.Schema(ctx, datasource.SchemaRequest{}, &sr)
		if sr.Diagnostics.HasError() {
			t.Errorf("%s: %v", meta.TypeName, sr.Diagnostics)
		}
		if diags := sr.Schema.ValidateImplementation(ctx); diags.HasError() {
			t.Errorf("%s: %v", meta.TypeName, diags)
		}
	}
	if len(seen) != 5 {
		t.Errorf("expected 5 data sources, got %d", len(seen))
	}
}
