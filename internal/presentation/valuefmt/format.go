package valuefmt

import "encoding/json"

// Format produces the compact, masked representation shared by terminal
// presenters. Full-value rendering remains the responsibility of each adapter.
func Format(raw json.RawMessage, sensitive bool) string {
	if sensitive {
		return "(sensitive)"
	}
	if raw == nil || string(raw) == "null" {
		return "null"
	}
	if len(raw) > 0 && raw[0] == '"' {
		var value string
		if err := json.Unmarshal(raw, &value); err == nil {
			return truncate(value)
		}
	}
	return truncate(string(raw))
}

func truncate(value string) string {
	const max = 120
	if len(value) > max {
		return value[:max] + "..."
	}
	return value
}
