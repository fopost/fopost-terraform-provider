package provider

import (
	"context"
	"testing"
	"time"

	fopost "github.com/fopost/fopost-go"
	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestOptionalStringTreatsEmptyAsUnset(t *testing.T) {
	t.Parallel()

	if got := optionalString(""); !got.IsNull() {
		t.Error("an empty string from the API must read as unset, not as a value")
	}
	if got := optionalString("hello"); got.ValueString() != "hello" {
		t.Errorf("optionalString(hello) = %q", got.ValueString())
	}
}

func TestTimestamp(t *testing.T) {
	t.Parallel()

	if got := timestamp(fopost.Time{}); !got.IsNull() {
		t.Error("a missing timestamp must read as null")
	}
	value := fopost.Time{Time: time.Date(2026, 8, 30, 9, 0, 0, 0, time.UTC), Raw: "2026-08-30T09:00:00Z"}
	if got := timestamp(value); got.ValueString() != "2026-08-30T09:00:00Z" {
		t.Errorf("timestamp = %q", got.ValueString())
	}
}

func TestStringHelpers(t *testing.T) {
	t.Parallel()

	if got := str(types.StringUnknown()); got != "" {
		t.Errorf("an unknown string must read as unset, got %q", got)
	}
	if got := optionalStrPtr(types.StringNull()); got != nil {
		t.Error("optionalStrPtr must omit an unset argument entirely")
	}
	if got := strPtr(types.StringNull()); got == nil || *got != "" {
		t.Error("strPtr must send an empty value, so removing an argument clears it upstream")
	}
	if got := boolPtr(types.BoolNull()); got != nil {
		t.Error("boolPtr must omit an unset argument entirely")
	}
}

func TestJSONRoundTrip(t *testing.T) {
	t.Parallel()

	encoded, err := encodeJSON(map[string]any{"feed_url": "https://example.com/feed.xml", "limit": float64(5)})
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeJSON(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if decoded["feed_url"] != "https://example.com/feed.xml" || decoded["limit"] != float64(5) {
		t.Errorf("round trip lost data: %#v", decoded)
	}

	empty, err := encodeJSON(nil)
	if err != nil {
		t.Fatal(err)
	}
	if !empty.IsNull() {
		t.Error("an absent config object must encode as null, not as an empty object")
	}
	if _, err := decodeJSON(jsontypes.NewNormalizedValue("not json")); err == nil {
		t.Error("invalid JSON must be reported, not swallowed")
	}
}

func TestStringSetRoundTrip(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	set, diags := eventsValue(ctx, []string{"post.published", "post.failed"})
	if diags.HasError() {
		t.Fatal(diags)
	}
	events, diags := stringSet(ctx, set)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if len(events) != 2 {
		t.Fatalf("events = %v", events)
	}

	if events, diags := stringSet(ctx, types.SetNull(types.StringType)); diags.HasError() || events != nil {
		t.Error("an unset set must read as no events")
	}
}
