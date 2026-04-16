package internal

import "fmt"

func RequireID(value, field string) error {
	if value == "" {
		return fmt.Errorf("%s is required", field)
	}
	return nil
}

func RequireOneOf(value, field string, allowed ...string) error {
	if value == "" {
		return fmt.Errorf("%s is required", field)
	}
	for _, a := range allowed {
		if value == a {
			return nil
		}
	}
	return fmt.Errorf("%s must be one of: %v", field, allowed)
}
