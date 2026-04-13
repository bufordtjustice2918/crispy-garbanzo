package audit

import "fmt"

// Validate checks that an Event has all required fields populated.
func Validate(e Event) error {
	if e.RequestID == "" {
		return fmt.Errorf("missing request_id")
	}
	if e.Decision == "" {
		return fmt.Errorf("missing decision")
	}
	if e.Decision != "allow" && e.Decision != "deny" && e.Decision != "allow-upstream-error" {
		return fmt.Errorf("invalid decision: %q", e.Decision)
	}
	if e.PolicyID == "" {
		return fmt.Errorf("missing policy_id")
	}
	if e.Destination == "" {
		return fmt.Errorf("missing destination")
	}
	if e.Method == "" {
		return fmt.Errorf("missing http_method")
	}
	return nil
}
