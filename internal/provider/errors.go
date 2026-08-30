package provider

import (
	"fmt"
	"net/http"

	fopost "github.com/fopost/fopost-go"
	"github.com/hashicorp/terraform-plugin-framework/diag"
)

// isNotFound reports the API's 404. On a read that means the object is gone
// upstream, and the caller must drop it from state instead of erroring.
func isNotFound(err error) bool { return fopost.IsNotFound(err) }

// apiDiagnostic turns an SDK error into a Terraform diagnostic. action reads as
// the thing that failed, e.g. "create the workspace".
//
// The API key never reaches a diagnostic: the SDK's error carries only the
// status, the machine code, the human message, and the response body, and none
// of them echo the credential.
func apiDiagnostic(action string, err error) diag.Diagnostic {
	apiErr, ok := fopost.APIError(err)
	if !ok {
		return diag.NewErrorDiagnostic(
			"FoPost API Request Failed",
			fmt.Sprintf("Could not %s. The request never completed: %s", action, err),
		)
	}
	summary, hint := explain(apiErr)
	detail := fmt.Sprintf("Could not %s.\n\nThe FoPost API answered HTTP %d", action, apiErr.Status)
	if apiErr.Code != "" {
		detail += fmt.Sprintf(" (%s)", apiErr.Code)
	}
	detail += ": " + apiErr.Message
	if hint != "" {
		detail += "\n\n" + hint
	}
	return diag.NewErrorDiagnostic(summary, detail)
}

func explain(err *fopost.Error) (summary, hint string) {
	switch {
	case err.Status == http.StatusUnauthorized:
		return "FoPost Authentication Failed",
			"The API key was rejected. Check the provider's api_key argument or the FOPOST_API_KEY environment variable against Settings → API Keys in the FoPost dashboard."
	case err.Status == http.StatusPaymentRequired:
		hint = "The FoPost plan on this account does not cover the request, or its allowance is used up."
		if upgrade := err.UpgradeURL(); upgrade != "" {
			hint += "\n\nUpgrade at: " + upgrade
		}
		return "FoPost Subscription Required", hint
	case err.Status == http.StatusForbidden:
		return "FoPost Permission Denied",
			"The API key is valid but lacks the scope or the workspace access this call needs. API key scopes are set in the FoPost dashboard under Settings → API Keys."
	case err.Status == http.StatusNotFound:
		return "FoPost Resource Not Found",
			"The object does not exist, or it sits outside the workspaces this API key can reach."
	case err.Status == http.StatusConflict:
		return "FoPost Resource Conflict",
			"The object is in a state that forbids this change."
	case err.Status == http.StatusTooManyRequests:
		hint = "The API key is over its rate limit. The provider already retried with backoff before giving up."
		if err.RetryAfter > 0 {
			hint += fmt.Sprintf(" The API asked for a wait of %s.", err.RetryAfter)
		}
		return "FoPost Rate Limit Exceeded", hint
	case err.Status == http.StatusBadRequest || err.Status == http.StatusUnprocessableEntity:
		return "Invalid FoPost Request",
			"The API rejected the arguments. Fix the configuration and apply again."
	case err.Status >= 500:
		return "FoPost API Error",
			"The FoPost API failed. The provider already retried with backoff. If this persists, report it at https://fopost.com/contact."
	default:
		return "FoPost API Request Failed", ""
	}
}
