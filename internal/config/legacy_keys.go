package config

import (
	"reflect"
	"strings"
)

// normalizeLegacyConfigMap maps legacy YAML keys (without underscores) to the
// canonical snake_case keys defined by mapstructure tags. It mutates and returns
// the provided map.
func normalizeLegacyConfigMap(data map[string]interface{}) map[string]interface{} {
	if data == nil {
		return nil
	}
	applyLegacyPathMappings(data)
	return normalizeMapForStruct(data, reflect.TypeOf(Config{}))
}

func applyLegacyPathMappings(data map[string]interface{}) {
	if git, ok := data["git"].(map[string]interface{}); ok {
		if val, ok := git["worktree_dir"]; ok {
			worktree := ensureMap(git, "worktree")
			if _, exists := worktree["dir"]; !exists {
				worktree["dir"] = val
			}
			delete(git, "worktree_dir")
		}

		if val, ok := git["worktree_mode"]; ok {
			worktree := ensureMap(git, "worktree")
			if _, exists := worktree["mode"]; !exists {
				worktree["mode"] = val
			}
			delete(git, "worktree_mode")
		}

		if val, ok := git["auto_clean"]; ok {
			worktree := ensureMap(git, "worktree")
			if _, exists := worktree["auto_clean"]; !exists {
				worktree["auto_clean"] = val
			}
			delete(git, "auto_clean")
		}

		if val, ok := git["auto_commit"]; ok {
			task := ensureMap(git, "task")
			if _, exists := task["auto_commit"]; !exists {
				task["auto_commit"] = val
			}
			delete(git, "auto_commit")
		}
	}

	issuesMap, ok := data["issues"].(map[string]interface{})
	if !ok {
		return
	}
	prompt, ok := issuesMap["prompt"].(map[string]interface{})
	if !ok {
		return
	}
	if lang, ok := prompt["language"].(string); ok {
		prompt["language"] = normalizeIssueLanguage(lang)
	}
}

func ensureMap(data map[string]interface{}, key string) map[string]interface{} {
	if existing, ok := data[key].(map[string]interface{}); ok {
		return existing
	}
	next := make(map[string]interface{})
	data[key] = next
	return next
}

func normalizeMapForStruct(data map[string]interface{}, t reflect.Type) map[string]interface{} {
	if data == nil {
		return nil
	}
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return data
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		name := canonicalTagName(field)
		if name == "" || name == "-" {
			continue
		}

		legacy := strings.ReplaceAll(name, "_", "")
		if legacy != name {
			if val, ok := data[legacy]; ok {
				if _, exists := data[name]; !exists {
					data[name] = val
				}
				delete(data, legacy)
			}
		}

		if val, ok := data[name]; ok {
			data[name] = normalizeValueForType(val, field.Type)
		}
	}

	return data
}

func normalizeValueForType(value interface{}, t reflect.Type) interface{} {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	switch t.Kind() {
	case reflect.Struct:
		if m, ok := value.(map[string]interface{}); ok {
			return normalizeMapForStruct(m, t)
		}
	case reflect.Slice:
		// Only normalize slices of structs/pointers to structs.
		if t.Elem().Kind() == reflect.Struct || (t.Elem().Kind() == reflect.Pointer && t.Elem().Elem().Kind() == reflect.Struct) {
			if list, ok := value.([]interface{}); ok {
				out := make([]interface{}, 0, len(list))
				for _, item := range list {
					out = append(out, normalizeValueForType(item, t.Elem()))
				}
				return out
			}
		}
	}

	return value
}

func canonicalTagName(field reflect.StructField) string {
	if tag := field.Tag.Get("mapstructure"); tag != "" {
		return strings.Split(tag, ",")[0]
	}
	if tag := field.Tag.Get("yaml"); tag != "" {
		return strings.Split(tag, ",")[0]
	}
	return strings.ToLower(field.Name)
}

func normalizeIssueLanguage(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return value
	}
	if mapped, ok := issueLanguageAliases[normalized]; ok {
		return mapped
	}
	return normalized
}

var issueLanguageAliases = map[string]string{
	"en":    "english",
	"es":    "spanish",
	"fr":    "french",
	"de":    "german",
	"pt":    "portuguese",
	"pt-br": "portuguese",
	"pt_br": "portuguese",
	"zh":    "chinese",
	"zh-cn": "chinese",
	"zh_cn": "chinese",
	"zh-tw": "chinese",
	"zh_tw": "chinese",
	"ja":    "japanese",
	"jp":    "japanese",
}
