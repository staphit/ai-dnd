package schema

import (
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"testing"
)

// assertStrictObjects mirrors the check in dm-schema.test.mjs: every object node
// must list every one of its properties in `required`, as Structured Outputs
// demands.
func assertStrictObjects(t *testing.T, node any, path string) {
	t.Helper()
	m, ok := node.(map[string]any)
	if !ok {
		return
	}

	if m["type"] == "object" {
		props, _ := m["properties"].(map[string]any)
		propNames := make([]string, 0, len(props))
		for name := range props {
			propNames = append(propNames, name)
		}
		requiredRaw, _ := m["required"].([]any)
		requiredNames := make([]string, 0, len(requiredRaw))
		for _, r := range requiredRaw {
			if s, ok := r.(string); ok {
				requiredNames = append(requiredNames, s)
			}
		}
		sort.Strings(propNames)
		sort.Strings(requiredNames)
		if !reflect.DeepEqual(propNames, requiredNames) {
			t.Errorf("%s.required must include every property: required=%v properties=%v", path, requiredNames, propNames)
		}
	}

	for key, value := range m {
		switch v := value.(type) {
		case []any:
			for i, entry := range v {
				assertStrictObjects(t, entry, path+"."+key+"["+itoa(i)+"]")
			}
		case map[string]any:
			assertStrictObjects(t, v, path+"."+key)
		}
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func TestDMSchemaMakesEveryPropertyRequired(t *testing.T) {
	var doc any
	if err := json.Unmarshal(Raw, &doc); err != nil {
		t.Fatalf("schema is not valid JSON: %v", err)
	}
	assertStrictObjects(t, doc, "root")
}

// TestEmbeddedSchemaMatchesFile guards against the embedded copy drifting from
// the on-disk schema Codex is pointed at.
func TestEmbeddedSchemaMatchesFile(t *testing.T) {
	onDisk, err := os.ReadFile("dm-turn.schema.json")
	if err != nil {
		t.Fatalf("read schema file: %v", err)
	}
	if string(onDisk) != string(Raw) {
		t.Error("embedded schema differs from dm-turn.schema.json")
	}
}
