package provider

import (
	"context"
	"strings"
	"testing"

	fwdatasource "github.com/hashicorp/terraform-plugin-framework/datasource"
	fwprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// testAccProtoV6ProviderFactories wires the in-process provider that the
// acceptance tests drive.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"fopost": providerserver.NewProtocol6WithError(New("test")()),
}

func TestProviderSchemaIsValid(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	resp := &fwprovider.SchemaResponse{}
	New("test")().Schema(ctx, fwprovider.SchemaRequest{}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("provider schema: %v", resp.Diagnostics)
	}
	if diags := resp.Schema.ValidateImplementation(ctx); diags.HasError() {
		t.Fatalf("provider schema implementation: %v", diags)
	}

	apiKey, ok := resp.Schema.Attributes["api_key"]
	if !ok {
		t.Fatal("the provider schema has no api_key attribute")
	}
	if !apiKey.IsSensitive() {
		t.Error("api_key must be marked sensitive so it never lands in a plan file in the clear")
	}
	if !apiKey.IsOptional() {
		t.Error("api_key must be optional so " + APIKeyEnvVar + " can supply it")
	}
	if baseURL := resp.Schema.Attributes["base_url"]; baseURL == nil || !baseURL.IsOptional() {
		t.Error("base_url must be an optional attribute")
	}
}

func TestResourceSchemasAreValid(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	for _, newResource := range New("test")().(*fopostProvider).Resources(ctx) {
		res := newResource()

		metaResp := &fwresource.MetadataResponse{}
		res.Metadata(ctx, fwresource.MetadataRequest{ProviderTypeName: "fopost"}, metaResp)
		if !strings.HasPrefix(metaResp.TypeName, "fopost_") {
			t.Errorf("resource type name %q is not namespaced to the provider", metaResp.TypeName)
		}

		schemaResp := &fwresource.SchemaResponse{}
		res.Schema(ctx, fwresource.SchemaRequest{}, schemaResp)
		if schemaResp.Diagnostics.HasError() {
			t.Fatalf("%s schema: %v", metaResp.TypeName, schemaResp.Diagnostics)
		}
		if diags := schemaResp.Schema.ValidateImplementation(ctx); diags.HasError() {
			t.Fatalf("%s schema implementation: %v", metaResp.TypeName, diags)
		}
		if schemaResp.Schema.MarkdownDescription == "" {
			t.Errorf("%s has no description, so its registry page would be blank", metaResp.TypeName)
		}
		for name, attribute := range schemaResp.Schema.Attributes {
			if attribute.GetMarkdownDescription() == "" {
				t.Errorf("%s.%s has no MarkdownDescription", metaResp.TypeName, name)
			}
		}
		if _, importable := res.(fwresource.ResourceWithImportState); !importable {
			t.Errorf("%s cannot be imported; every managed resource must implement ImportState", metaResp.TypeName)
		}
	}
}

func TestDataSourceSchemasAreValid(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	for _, newDataSource := range New("test")().(*fopostProvider).DataSources(ctx) {
		ds := newDataSource()

		metaResp := &fwdatasource.MetadataResponse{}
		ds.Metadata(ctx, fwdatasource.MetadataRequest{ProviderTypeName: "fopost"}, metaResp)
		if !strings.HasPrefix(metaResp.TypeName, "fopost_") {
			t.Errorf("data source type name %q is not namespaced to the provider", metaResp.TypeName)
		}

		schemaResp := &fwdatasource.SchemaResponse{}
		ds.Schema(ctx, fwdatasource.SchemaRequest{}, schemaResp)
		if schemaResp.Diagnostics.HasError() {
			t.Fatalf("%s schema: %v", metaResp.TypeName, schemaResp.Diagnostics)
		}
		if diags := schemaResp.Schema.ValidateImplementation(ctx); diags.HasError() {
			t.Fatalf("%s schema implementation: %v", metaResp.TypeName, diags)
		}
		if schemaResp.Schema.MarkdownDescription == "" {
			t.Errorf("%s has no description, so its registry page would be blank", metaResp.TypeName)
		}
		for name, attribute := range schemaResp.Schema.Attributes {
			if attribute.GetMarkdownDescription() == "" {
				t.Errorf("%s.%s has no MarkdownDescription", metaResp.TypeName, name)
			}
		}
	}
}

// TestExpectedResourcesAndDataSources pins the scope decision: posts are events,
// not infrastructure, and must never become a managed resource here.
func TestExpectedResourcesAndDataSources(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	p := New("test")().(*fopostProvider)

	wantResources := map[string]bool{
		"fopost_workspace":  true,
		"fopost_label":      true,
		"fopost_webhook":    true,
		"fopost_automation": true,
	}
	got := map[string]bool{}
	for _, newResource := range p.Resources(ctx) {
		resp := &fwresource.MetadataResponse{}
		newResource().Metadata(ctx, fwresource.MetadataRequest{ProviderTypeName: "fopost"}, resp)
		got[resp.TypeName] = true
	}
	for name := range wantResources {
		if !got[name] {
			t.Errorf("resource %s is missing", name)
		}
	}
	for name := range got {
		if !wantResources[name] {
			t.Errorf("resource %s is not part of the provider's declared scope; publishing content is an "+
				"event, not infrastructure", name)
		}
	}

	wantDataSources := map[string]bool{
		"fopost_workspace":  true,
		"fopost_workspaces": true,
		"fopost_account":    true,
		"fopost_accounts":   true,
		"fopost_labels":     true,
	}
	gotDataSources := map[string]bool{}
	for _, newDataSource := range p.DataSources(ctx) {
		resp := &fwdatasource.MetadataResponse{}
		newDataSource().Metadata(ctx, fwdatasource.MetadataRequest{ProviderTypeName: "fopost"}, resp)
		gotDataSources[resp.TypeName] = true
	}
	for name := range wantDataSources {
		if !gotDataSources[name] {
			t.Errorf("data source %s is missing", name)
		}
	}
}

func TestConfigure(t *testing.T) {
	const secret = "fp_super_secret_value"

	tests := []struct {
		name        string
		apiKey      *string
		envKey      string
		wantError   bool
		wantSummary string
	}{
		{name: "key from the provider block", apiKey: ptr(secret)},
		{name: "key from the environment", envKey: secret},
		{name: "no key at all", wantError: true, wantSummary: "Missing FoPost API Key"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv(APIKeyEnvVar, test.envKey)
			t.Setenv(BaseURLEnvVar, "")

			resp := configureProvider(t, test.apiKey, nil)
			if test.wantError {
				if !resp.Diagnostics.HasError() {
					t.Fatal("expected a diagnostic, got none")
				}
				if summary := resp.Diagnostics.Errors()[0].Summary(); summary != test.wantSummary {
					t.Errorf("summary = %q, want %q", summary, test.wantSummary)
				}
				return
			}
			if resp.Diagnostics.HasError() {
				t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
			}
			if resp.ResourceData == nil || resp.DataSourceData == nil {
				t.Fatal("Configure did not hand the SDK client to the resources and data sources")
			}
		})
	}
}

// TestConfigureNeverLeaksTheAPIKey is the invariant behind every diagnostic in
// this provider: a credential must never travel in an error a user can see.
func TestConfigureNeverLeaksTheAPIKey(t *testing.T) {
	const secret = "fp_leak_canary_0123456789"
	t.Setenv(APIKeyEnvVar, "")
	t.Setenv(BaseURLEnvVar, "")

	// A base URL the SDK cannot use forces the failure path.
	resp := configureProvider(t, ptr(secret), ptr("://not a url"))
	for _, diagnostic := range resp.Diagnostics {
		if strings.Contains(diagnostic.Summary()+diagnostic.Detail(), secret) {
			t.Fatalf("a diagnostic carried the API key: %s / %s", diagnostic.Summary(), diagnostic.Detail())
		}
	}
}

func TestConfigureRejectsAnUnknownAPIKey(t *testing.T) {
	t.Setenv(APIKeyEnvVar, "")
	t.Setenv(BaseURLEnvVar, "")

	ctx := context.Background()
	p := New("test")()
	schemaResp := &fwprovider.SchemaResponse{}
	p.Schema(ctx, fwprovider.SchemaRequest{}, schemaResp)

	raw := tftypes.NewValue(schemaResp.Schema.Type().TerraformType(ctx), map[string]tftypes.Value{
		"api_key":  tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		"base_url": tftypes.NewValue(tftypes.String, nil),
	})
	resp := &fwprovider.ConfigureResponse{}
	p.Configure(ctx, fwprovider.ConfigureRequest{
		Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: raw},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("an unknown api_key must be reported at plan time, not deferred")
	}
	if summary := resp.Diagnostics.Errors()[0].Summary(); summary != "Unknown FoPost API Key" {
		t.Errorf("summary = %q", summary)
	}
}

func configureProvider(t *testing.T, apiKey, baseURL *string) *fwprovider.ConfigureResponse {
	t.Helper()

	ctx := context.Background()
	p := New("test")()
	schemaResp := &fwprovider.SchemaResponse{}
	p.Schema(ctx, fwprovider.SchemaRequest{}, schemaResp)

	value := func(s *string) tftypes.Value {
		if s == nil {
			return tftypes.NewValue(tftypes.String, nil)
		}
		return tftypes.NewValue(tftypes.String, *s)
	}
	raw := tftypes.NewValue(schemaResp.Schema.Type().TerraformType(ctx), map[string]tftypes.Value{
		"api_key":  value(apiKey),
		"base_url": value(baseURL),
	})

	resp := &fwprovider.ConfigureResponse{}
	p.Configure(ctx, fwprovider.ConfigureRequest{
		Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: raw},
	}, resp)
	return resp
}

func ptr[T any](v T) *T { return &v }
