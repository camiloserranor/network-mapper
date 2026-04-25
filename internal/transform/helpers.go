// Package transform converts raw gNMI responses into topology model objects.
package transform

import (
	"encoding/json"
	"fmt"
	"strings"
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

// NormalizeInterfaceName converts interface names to canonical format.
func NormalizeInterfaceName(name string) string {
	if strings.HasPrefix(name, "Ethernet") || strings.HasPrefix(name, "PortChannel") ||
		strings.HasPrefix(name, "Loopback") || strings.HasPrefix(name, "Management") {
		return name
	}
	// Cisco NX-OS: eth1/1 → Eth1/1
	if strings.HasPrefix(name, "eth") {
		return "Eth" + name[3:]
	}
	return name
}
