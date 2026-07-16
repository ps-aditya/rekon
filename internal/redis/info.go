package redis

import "strings"

// ParseInfo turns Redis's raw INFO reply into a lookup map.
//
// INFO's format is line-based: section headers start with '#'
// (e.g. "# Memory"), blank lines separate sections, and data lines are
// "key:value". This function only extracts key:value pairs — it
// deliberately doesn't preserve section grouping, since nothing in
// Rekon needs "which section was this field in" as of this sprint
// (every field name is already unique across the whole INFO reply).
func ParseInfo(raw string) map[string]string {
	result := make(map[string]string)

	for _, line := range strings.Split(raw, "\r\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, ":")
		if !found {
			// Malformed line — skip rather than error. INFO is Redis's
			// own trusted output, but being defensive here costs
			// nothing and avoids a single unexpected line taking down
			// parsing of an otherwise-good reply.
			continue
		}
		result[key] = value
	}

	return result
}
