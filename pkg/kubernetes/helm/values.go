package helm

// StringAt and BoolAt read one value out of a release's values tree, which
// only holds what was set explicitly, so any step of the path may be missing.

func StringAt(values map[string]any, keys ...string) string {
	value, ok := at(values, keys...).(string)
	if !ok {
		return ""
	}
	return value
}

func BoolAt(values map[string]any, fallback bool, keys ...string) bool {
	value, ok := at(values, keys...).(bool)
	if !ok {
		return fallback
	}
	return value
}

func at(values map[string]any, keys ...string) any {
	var current any = values
	for _, key := range keys {
		node, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current, ok = node[key]
		if !ok {
			return nil
		}
	}
	return current
}
