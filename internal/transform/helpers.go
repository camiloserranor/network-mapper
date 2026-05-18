// Package transform converts raw gNMI responses into topology model objects.
package transform

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/camiloserranor/network-mapper/internal/gnmi"
)

// GetString safely extracts a string value from a map.
func GetString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
		return fmt.Sprintf("%v", v)
	}
	return ""
}

// GetFirstString tries multiple keys in order and returns the first non-empty value.
func GetFirstString(m map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if v := GetString(m, key); v != "" {
			return v
		}
	}
	return ""
}

// GetMap safely extracts a nested map.
func GetMap(m map[string]interface{}, key string) map[string]interface{} {
	if v, ok := m[key]; ok {
		if sub, ok := v.(map[string]interface{}); ok {
			return sub
		}
	}
	return nil
}

// GetSlice extracts a []interface{} from a map.
func GetSlice(m map[string]interface{}, key string) []interface{} {
	if v, ok := m[key]; ok {
		if s, ok := v.([]interface{}); ok {
			return s
		}
	}
	return nil
}

// GetNumber extracts a numeric value as uint64.
func GetNumber(m map[string]interface{}, key string) uint64 {
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return uint64(n)
	case int64:
		return uint64(n)
	case uint64:
		return n
	case json.Number:
		i, _ := n.Int64()
		return uint64(i)
	case string:
		// some gNMI implementations return counters as strings
		var val uint64
		fmt.Sscanf(n, "%d", &val)
		return val
	}
	return 0
}

// GetInt extracts an integer value.
func GetInt(m map[string]interface{}, key string) int {
	return int(GetNumber(m, key))
}

// AsMapSlice converts a value to []map[string]interface{}.
func AsMapSlice(v interface{}) []map[string]interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		return []map[string]interface{}{val}
	case []interface{}:
		var out []map[string]interface{}
		for _, item := range val {
			if m, ok := item.(map[string]interface{}); ok {
				out = append(out, m)
			}
		}
		return out
	}
	return nil
}

// ExtractPathKey extracts a key value from a gNMI path string.
// e.g., "/interfaces/interface[name=Ethernet1]/state" with key "name" → "Ethernet1"
func ExtractPathKey(path, key string) string {
	search := key + "="
	idx := strings.Index(path, search)
	if idx == -1 {
		return ""
	}
	start := idx + len(search)
	end := strings.Index(path[start:], "]")
	if end == -1 {
		return path[start:]
	}
	return path[start : start+end]
}

// NormalizeInterfaceName converts interface names to canonical short format.
// NX-OS uses short names (Eth1/1) everywhere except OpenConfig paths which use
// the long form (Ethernet1/1). We normalize to short form for consistency.
func NormalizeInterfaceName(name string) string {
	// Cisco NX-OS long form → short form: Ethernet1/51 → Eth1/51
	if strings.HasPrefix(name, "Ethernet") {
		return "Eth" + name[len("Ethernet"):]
	}
	if strings.HasPrefix(name, "PortChannel") || strings.HasPrefix(name, "Loopback") ||
		strings.HasPrefix(name, "Management") {
		return name
	}
	// Cisco NX-OS: eth1/1 → Eth1/1
	if strings.HasPrefix(name, "eth") {
		return "Eth" + name[3:]
	}
	return name
}

// --- Flat-leaf helpers for SONiC Subscribe ONCE format ---

// isFlatLeafFormat detects whether notifications are in the SONiC flat-leaf format
// where each notification has multiple updates with scalar values and path-like keys.
func isFlatLeafFormat(notifs []gnmi.Notification) bool {
	if len(notifs) == 0 {
		return false
	}
	first := notifs[0]
	if len(first.Updates) < 2 {
		return false
	}
	for _, u := range first.Updates {
		if _, isMap := u.Value.(map[string]interface{}); isMap {
			return false
		}
		if _, isSlice := u.Value.([]interface{}); isSlice {
			return false
		}
	}
	return true
}

// buildLeafMap creates a map from update paths to their values for flat-leaf format.
func buildLeafMap(updates []gnmi.Update) map[string]interface{} {
	m := make(map[string]interface{}, len(updates))
	for _, u := range updates {
		m[u.Path] = u.Value
	}
	return m
}

// getLeafString extracts a string value from a leaf map by path.
func getLeafString(m map[string]interface{}, path string) string {
	v, ok := m[path]
	if !ok {
		return ""
	}
	switch s := v.(type) {
	case string:
		return s
	case float64:
		return fmt.Sprintf("%.0f", s)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// getLeafInt extracts an integer value from a leaf map by path.
func getLeafInt(m map[string]interface{}, path string) int {
	v, ok := m[path]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int64:
		return int(n)
	case uint64:
		return int(n)
	default:
		return 0
	}
}

// getLeafNumber extracts a uint64 value from a leaf map by path.
func getLeafNumber(m map[string]interface{}, path string) uint64 {
	v, ok := m[path]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return uint64(n)
	case int64:
		return uint64(n)
	case uint64:
		return n
	default:
		return 0
	}
}

// getLeafBySuffix finds the first value in a leaf map whose key ends with the given suffix.
// This is useful for LLDP where paths include list keys (e.g., /neighbors/neighbor[id=X]/state/chassis-id).
func getLeafBySuffix(m map[string]interface{}, suffix string) string {
	for k, v := range m {
		if strings.HasSuffix(k, suffix) {
			switch s := v.(type) {
			case string:
				return s
			case float64:
				return fmt.Sprintf("%.0f", s)
			default:
				return fmt.Sprintf("%v", v)
			}
		}
	}
	return ""
}

// GetBool extracts a boolean value from a map.
func GetBool(m map[string]interface{}, key string) bool {
	v, ok := m[key]
	if !ok {
		return false
	}
	switch b := v.(type) {
	case bool:
		return b
	case string:
		return b == "true" || b == "True" || b == "1"
	case float64:
		return b != 0
	}
	return false
}
