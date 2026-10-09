package manifest

import (
	"encoding/json"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// This evaluates the editor schema's basic-shape subset, not runtime policy.
func shapeValid(schema, defs map[string]any, value any) bool {
	if ref, ok := schema["$ref"].(string); ok {
		return shapeValid(defs[strings.TrimPrefix(ref, "#/$defs/")].(map[string]any), defs, value)
	}
	if expected, ok := schema["const"]; ok && !reflect.DeepEqual(value, expected) {
		return false
	}
	if enum, ok := schema["enum"].([]any); ok {
		found := false
		for _, item := range enum {
			found = found || reflect.DeepEqual(value, item)
		}
		if !found {
			return false
		}
	}
	switch schema["type"] {
	case "object":
		object, ok := value.(map[string]any)
		if !ok {
			return false
		}
		properties := schema["properties"].(map[string]any)
		if required, ok := schema["required"].([]any); ok {
			for _, key := range required {
				if _, ok := object[key.(string)]; !ok {
					return false
				}
			}
		}
		for key, item := range object {
			property, ok := properties[key]
			if !ok {
				if schema["additionalProperties"] == false {
					return false
				}
				continue
			}
			if !shapeValid(property.(map[string]any), defs, item) {
				return false
			}
		}
	case "array":
		array, ok := value.([]any)
		if !ok {
			return false
		}
		if min, ok := schema["minItems"].(float64); ok && len(array) < int(min) {
			return false
		}
		if max, ok := schema["maxItems"].(float64); ok && len(array) > int(max) {
			return false
		}
		for i, item := range array {
			if !shapeValid(schema["items"].(map[string]any), defs, item) {
				return false
			}
			if schema["uniqueItems"] == true {
				for _, previous := range array[:i] {
					if reflect.DeepEqual(previous, item) {
						return false
					}
				}
			}
		}
	case "string":
		text, ok := value.(string)
		if !ok {
			return false
		}
		if min, ok := schema["minLength"].(float64); ok && len([]rune(text)) < int(min) {
			return false
		}
		if max, ok := schema["maxLength"].(float64); ok && len([]rune(text)) > int(max) {
			return false
		}
		if pattern, ok := schema["pattern"].(string); ok && !regexp.MustCompile(pattern).MatchString(text) {
			return false
		}
	case "integer":
		number, ok := value.(float64)
		if !ok || number != float64(int(number)) {
			return false
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return false
		}
	default:
		return false
	}
	return true
}

func TestEditorSchemaCorpusParity(t *testing.T) {
	data, err := os.ReadFile("../../schemas/pakuvalnia-v1.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err = json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	for name, input := range map[string]string{
		"valid":   valid,
		"unknown": valid + "unknown: true\n",
		"schema":  strings.Replace(valid, "schema: 1", "schema: 2", 1),
		"profile": strings.Replace(valid, "cli-v1", "cli-v2", 1),
		"type":    strings.Replace(valid, "  license: MIT", "  license: 123", 1),
		"null":    strings.Replace(valid, "  scoop: true", "  scoop: null", 1),
		"formats": strings.Replace(valid, "[deb, rpm, apk]", "[deb, deb]", 1),
		"args":    strings.Replace(valid, `args: ["--version"]`, "args: []", 1),
		"missing": "schema: 1\n",
	} {
		t.Run(name, func(t *testing.T) {
			var generic any
			if err := yaml.Unmarshal([]byte(input), &generic); err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(generic)
			if err != nil {
				t.Fatal(err)
			}
			if err = json.Unmarshal(encoded, &generic); err != nil {
				t.Fatal(err)
			}
			_, err = LoadManifest(strings.NewReader(input))
			if shapeValid(schema, schema["$defs"].(map[string]any), generic) != (err == nil) {
				t.Fatalf("basic-shape/runtime disagreement: %v", err)
			}
		})
	}
}
