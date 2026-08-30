package provider

import (
	"errors"
	"strings"
	"testing"
	"time"

	fopost "github.com/fopost/fopost-go"
)

func TestAPIDiagnosticMapsEveryStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		err         *fopost.Error
		wantSummary string
		wantIn      []string
	}{
		{
			name:        "401",
			err:         &fopost.Error{Status: 401, Code: "unauthorized", Message: "invalid api key"},
			wantSummary: "FoPost Authentication Failed",
			wantIn:      []string{APIKeyEnvVar, "invalid api key"},
		},
		{
			name: "402 carries the upgrade url",
			err: &fopost.Error{
				Status:  402,
				Code:    "subscription_required",
				Message: "no active subscription",
				Body:    []byte(`{"error":"subscription_required","upgrade_url":"https://fopost.com/pricing"}`),
			},
			wantSummary: "FoPost Subscription Required",
			wantIn:      []string{"https://fopost.com/pricing"},
		},
		{
			name:        "403",
			err:         &fopost.Error{Status: 403, Code: "forbidden", Message: "missing scope"},
			wantSummary: "FoPost Permission Denied",
			wantIn:      []string{"scope"},
		},
		{
			name:        "404",
			err:         &fopost.Error{Status: 404, Code: "not_found", Message: "no such workspace"},
			wantSummary: "FoPost Resource Not Found",
		},
		{
			name:        "409",
			err:         &fopost.Error{Status: 409, Code: "conflict", Message: "already publishing"},
			wantSummary: "FoPost Resource Conflict",
		},
		{
			name:        "422",
			err:         &fopost.Error{Status: 422, Code: "validation_error", Message: "slug is taken"},
			wantSummary: "Invalid FoPost Request",
			wantIn:      []string{"slug is taken"},
		},
		{
			name:        "429 reports the wait the API asked for",
			err:         &fopost.Error{Status: 429, Code: "rate_limited", Message: "slow down", RetryAfter: 12 * time.Second},
			wantSummary: "FoPost Rate Limit Exceeded",
			wantIn:      []string{"12s"},
		},
		{
			name:        "500",
			err:         &fopost.Error{Status: 500, Message: "boom"},
			wantSummary: "FoPost API Error",
			wantIn:      []string{"https://fopost.com/contact"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			diagnostic := apiDiagnostic("create the workspace", test.err)
			if diagnostic.Summary() != test.wantSummary {
				t.Errorf("summary = %q, want %q", diagnostic.Summary(), test.wantSummary)
			}
			if !strings.Contains(diagnostic.Detail(), "create the workspace") {
				t.Error("the detail must name the action that failed")
			}
			for _, want := range test.wantIn {
				if !strings.Contains(diagnostic.Detail(), want) {
					t.Errorf("detail is missing %q:\n%s", want, diagnostic.Detail())
				}
			}
		})
	}
}

func TestAPIDiagnosticHandlesATransportError(t *testing.T) {
	t.Parallel()

	diagnostic := apiDiagnostic("read the label", errors.New("dial tcp: connection refused"))
	if diagnostic.Summary() != "FoPost API Request Failed" {
		t.Errorf("summary = %q", diagnostic.Summary())
	}
	if !strings.Contains(diagnostic.Detail(), "connection refused") {
		t.Errorf("detail lost the underlying error:\n%s", diagnostic.Detail())
	}
}

func TestIsNotFound(t *testing.T) {
	t.Parallel()

	if !isNotFound(&fopost.Error{Status: 404}) {
		t.Error("a 404 must be recognised, or a deleted object never leaves state")
	}
	if isNotFound(&fopost.Error{Status: 403}) {
		t.Error("a 403 is not a 404; treating it as one would silently drop a real resource from state")
	}
	if isNotFound(errors.New("network")) {
		t.Error("a transport error is not a 404")
	}
}
