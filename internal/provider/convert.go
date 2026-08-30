package provider

import (
	"context"
	"encoding/json"

	fopost "github.com/fopost/fopost-go"
	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// optionalString maps the API's empty string onto a null, so an argument the
// caller never set does not come back as "".
func optionalString(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}

// timestamp renders an API timestamp as RFC 3339, or null when it sent none.
func timestamp(t fopost.Time) types.String {
	if t.IsZero() && t.Raw == "" {
		return types.StringNull()
	}
	return types.StringValue(t.String())
}

// str reads a Terraform string for a request body, treating null and unknown
// as "not set".
func str(v types.String) string {
	if v.IsNull() || v.IsUnknown() {
		return ""
	}
	return v.ValueString()
}

// strPtr is str for a request field that clears the value when sent empty.
func strPtr(v types.String) *string {
	s := str(v)
	return &s
}

// optionalStrPtr omits the field entirely when the caller set nothing.
func optionalStrPtr(v types.String) *string {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	s := v.ValueString()
	return &s
}

// boolPtr is nil when the caller left the argument to the API's default.
func boolPtr(v types.Bool) *bool {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	b := v.ValueBool()
	return &b
}

// encodeJSON renders a free-form API config object as a normalized JSON string.
// jsontypes.Normalized compares semantically, so key order and whitespace never
// produce a diff.
func encodeJSON(m map[string]any) (jsontypes.Normalized, error) {
	if len(m) == 0 {
		return jsontypes.NewNormalizedNull(), nil
	}
	encoded, err := json.Marshal(m)
	if err != nil {
		return jsontypes.NewNormalizedNull(), err
	}
	return jsontypes.NewNormalizedValue(string(encoded)), nil
}

// decodeJSON parses a JSON-object argument into the map the SDK expects.
func decodeJSON(v jsontypes.Normalized) (map[string]any, error) {
	if v.IsNull() || v.IsUnknown() || v.ValueString() == "" {
		return nil, nil
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(v.ValueString()), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// stringSet reads a Terraform set of strings for a request body.
func stringSet(ctx context.Context, set types.Set) ([]string, diag.Diagnostics) {
	if set.IsNull() || set.IsUnknown() {
		return nil, nil
	}
	out := make([]string, 0, len(set.Elements()))
	diags := set.ElementsAs(ctx, &out, false)
	return out, diags
}

// eventsValue renders a list of webhook events back into a Terraform set.
func eventsValue(ctx context.Context, events []string) (types.Set, diag.Diagnostics) {
	if events == nil {
		events = []string{}
	}
	return types.SetValueFrom(ctx, types.StringType, events)
}
