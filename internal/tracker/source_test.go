package tracker

import (
	"errors"
	"strings"
	"testing"
)

func TestCheckFailureDetailExplainsNexusForbiddenWithoutAPIKey(t *testing.T) {
	result := CheckResult{
		URL: "https://www.nexusmods.com/7daystodie/mods/10836",
		Err: &HTTPStatusError{
			StatusCode: 403,
			Status:     "403 Forbidden",
			RequestID:  "request-123",
		},
	}

	detail := CheckFailureDetail(result, false)
	for _, expected := range []string{
		"Nexus: 403 Forbidden",
		"Add a Nexus API key in Settings",
		"Request ID: request-123",
	} {
		if !strings.Contains(detail, expected) {
			t.Fatalf("detail %q does not contain %q", detail, expected)
		}
	}
}

func TestCheckFailureDetailExplainsRejectedNexusAPIKey(t *testing.T) {
	result := CheckResult{
		URL: "https://www.nexusmods.com/7daystodie/mods/10836",
		Err: &HTTPStatusError{StatusCode: 403, Status: "403 Forbidden"},
	}

	detail := CheckFailureDetail(result, true)
	if !strings.Contains(detail, "Check that the Nexus API key") {
		t.Fatalf("unexpected detail: %q", detail)
	}
}

func TestCheckFailureDetailExplainsMissingVersion(t *testing.T) {
	result := CheckResult{
		URL: "https://7daystodiemods.com/mods/smart-doors/",
		Err: ErrNoVersion,
	}

	detail := CheckFailureDetail(result, false)
	if !strings.Contains(detail, "no mod version could be identified") {
		t.Fatalf("unexpected detail: %q", detail)
	}
}

func TestHTTPStatusErrorSupportsErrorsAs(t *testing.T) {
	err := error(&HTTPStatusError{StatusCode: 429, Status: "429 Too Many Requests"})
	var statusErr *HTTPStatusError
	if !errors.As(err, &statusErr) || statusErr.StatusCode != 429 {
		t.Fatalf("could not recover HTTP status error from %v", err)
	}
}
