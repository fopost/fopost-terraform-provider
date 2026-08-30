package provider

import (
	fopost "github.com/fopost/fopost-go"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// importIDPath is the attribute every resource's `terraform import` writes into.
var importIDPath = path.Root("id")

const issuesURL = "https://github.com/fopost/terraform-provider-fopost/issues"

// clientFromResource pulls the shared SDK client out of a resource Configure
// request, reporting the framework's own wiring bug if it ever holds otherwise.
func clientFromResource(req resource.ConfigureRequest, resp *resource.ConfigureResponse) *fopost.Client {
	if req.ProviderData == nil {
		return nil
	}
	client, ok := req.ProviderData.(*fopost.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			"The provider handed the resource something other than a FoPost client. This is a bug in the "+
				"provider; please report it at "+issuesURL+".",
		)
		return nil
	}
	return client
}

// clientFromDataSource is clientFromResource for a data source.
func clientFromDataSource(req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) *fopost.Client {
	if req.ProviderData == nil {
		return nil
	}
	client, ok := req.ProviderData.(*fopost.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			"The provider handed the data source something other than a FoPost client. This is a bug in the "+
				"provider; please report it at "+issuesURL+".",
		)
		return nil
	}
	return client
}

// firstNonEmpty is the fallback chain behind every configurable value: the
// explicit argument, then the environment variable, then the default.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
