package config

import (
	"reflect"
	"strings"
	"testing"
)

func TestLegacyKeyNormalization_NoUnderscoreCollisions(t *testing.T) {
	root := reflect.TypeOf(Config{})
	structs := map[reflect.Type]struct{}{}
	collectConfigStructTypes(root, structs)

	for typ := range structs {
		seen := map[string]string{}
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			if !field.IsExported() {
				continue
			}
			name := canonicalTagName(field)
			if name == "" || name == "-" {
				continue
			}
			legacy := strings.ReplaceAll(name, "_", "")
			if prev, ok := seen[legacy]; ok && prev != name {
				t.Fatalf("legacy key collision in %s: %q and %q both map to %q",
					typ.Name(), prev, name, legacy)
			}
			seen[legacy] = name
		}
	}
}

func collectConfigStructTypes(t reflect.Type, seen map[reflect.Type]struct{}) {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return
	}
	if _, ok := seen[t]; ok {
		return
	}
	// Only include structs from this package
	if t.PkgPath() != reflect.TypeOf(Config{}).PkgPath() {
		return
	}

	seen[t] = struct{}{}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldType := field.Type
		if fieldType.Kind() == reflect.Pointer {
			fieldType = fieldType.Elem()
		}
		switch fieldType.Kind() {
		case reflect.Struct:
			collectConfigStructTypes(fieldType, seen)
		case reflect.Slice:
			elem := fieldType.Elem()
			if elem.Kind() == reflect.Pointer {
				elem = elem.Elem()
			}
			if elem.Kind() == reflect.Struct {
				collectConfigStructTypes(elem, seen)
			}
		}
	}
}
