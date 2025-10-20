package renderfs

import (
	"reflect"
	"strconv"
	"strings"

	"github.com/flosch/pongo2/v6"
)

func resolvePath(ctx pongo2.Context, path string) bool {
	segments, err := parsePath(path)
	if err != nil {
		return false
	}

	var current interface{} = ctx
	for _, segment := range segments {
		next, ok := lookupSegment(current, segment)
		if !ok {
			return false
		}
		current = next
	}

	return true
}

type pathSegment struct {
	name       string
	subscripts []pathIndex
}

type indexKind int

const (
	indexKindUnknown indexKind = iota
	indexKindInt
	indexKindString
)

type pathIndex struct {
	kind        indexKind
	intValue    int
	stringValue string
}

func parsePath(path string) ([]pathSegment, error) {
	var segments []pathSegment
	expr := strings.TrimSpace(path)
	if expr == "" {
		return nil, nil
	}

	for len(expr) > 0 {
		var segment pathSegment
		var nameBuilder strings.Builder

		for len(expr) > 0 {
			switch expr[0] {
			case '.':
				expr = strings.TrimLeft(expr[1:], " ")
				goto segmentReady
			case '[':
				goto segmentReady
			default:
				nameBuilder.WriteByte(expr[0])
				expr = expr[1:]
			}
		}

	segmentReady:
		segment.name = strings.TrimSpace(nameBuilder.String())

		for len(expr) > 0 && expr[0] == '[' {
			end := findClosingBracketInString(expr)
			if end == -1 {
				return nil, errInvalidPath
			}
			content := strings.TrimSpace(expr[1:end])
			idx := parseIndex(content)
			segment.subscripts = append(segment.subscripts, idx)
			expr = strings.TrimSpace(expr[end+1:])
		}

		segments = append(segments, segment)

		if len(expr) > 0 && expr[0] == '.' {
			expr = strings.TrimLeft(expr[1:], " ")
		} else if len(expr) > 0 {
			// Unexpected character, stop parsing remaining to avoid false negatives.
			break
		}
	}

	return segments, nil
}

var errInvalidPath = &pathError{}

type pathError struct{}

func (p *pathError) Error() string { return "invalid variable path" }

func findClosingBracketInString(expr string) int {
	depth := 0
	for i := 0; i < len(expr); i++ {
		switch expr[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func parseIndex(content string) pathIndex {
	if len(content) == 0 {
		return pathIndex{kind: indexKindUnknown}
	}

	if content[0] == '\'' || content[0] == '"' {
		if len(content) < 2 {
			return pathIndex{kind: indexKindUnknown}
		}
		quote := content[0]
		if content[len(content)-1] != quote {
			return pathIndex{kind: indexKindUnknown}
		}
		return pathIndex{
			kind:        indexKindString,
			stringValue: strings.ReplaceAll(content[1:len(content)-1], `\`+string(quote), string(quote)),
		}
	}

	if num, err := strconv.Atoi(content); err == nil {
		return pathIndex{
			kind:     indexKindInt,
			intValue: num,
		}
	}

	return pathIndex{kind: indexKindUnknown}
}

func lookupSegment(current interface{}, segment pathSegment) (interface{}, bool) {
	value, ok := getAttribute(current, segment.name)
	if !ok {
		return nil, false
	}

	for _, sub := range segment.subscripts {
		next, ok := applySubscript(value, sub)
		if !ok {
			return nil, false
		}
		value = next
	}

	return value, true
}

func getAttribute(current interface{}, name string) (interface{}, bool) {
	if name == "" {
		return current, true
	}

	if ctx, ok := current.(pongo2.Context); ok {
		val, ok := ctx[name]
		return val, ok
	}

	if m, ok := current.(map[string]interface{}); ok {
		val, ok := m[name]
		return val, ok
	}

	rv, ok := toReflectValue(current)
	if !ok {
		return nil, false
	}

	switch rv.Kind() {
	case reflect.Map:
		key, allowed := buildMapKey(name, rv.Type().Key())
		if !allowed {
			return nil, false
		}
		val := rv.MapIndex(key)
		if !val.IsValid() {
			return nil, false
		}
		return val.Interface(), true
	case reflect.Struct:
		field := rv.FieldByName(name)
		if !field.IsValid() && name != "" {
			field = rv.FieldByName(toExportedName(name))
		}
		if field.IsValid() && field.CanInterface() {
			return field.Interface(), true
		}

		if method := rv.MethodByName(name); method.IsValid() {
			return callNoArgMethod(method)
		}

		if name != "" {
			if method := rv.MethodByName(toExportedName(name)); method.IsValid() {
				return callNoArgMethod(method)
			}
		}
	}

	return nil, false
}

func toReflectValue(value interface{}) (reflect.Value, bool) {
	if value == nil {
		return reflect.Value{}, false
	}

	if pv, ok := value.(*pongo2.Value); ok {
		return toReflectValue(pv.Interface())
	}

	rv := reflect.ValueOf(value)
	for rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return reflect.Value{}, false
		}
		rv = rv.Elem()
	}

	return rv, true
}

func buildMapKey(name string, keyType reflect.Type) (reflect.Value, bool) {
	switch keyType.Kind() {
	case reflect.String:
		return reflect.ValueOf(name).Convert(keyType), true
	case reflect.Interface:
		return reflect.ValueOf(name), true
	}
	return reflect.Value{}, false
}

func callNoArgMethod(method reflect.Value) (interface{}, bool) {
	if method.Type().NumIn() != 0 {
		return nil, false
	}

	results := method.Call(nil)
	switch len(results) {
	case 0:
		return nil, true
	case 1:
		return results[0].Interface(), true
	case 2:
		if err, ok := results[1].Interface().(error); ok && err != nil {
			return nil, false
		}
		return results[0].Interface(), true
	default:
		return nil, false
	}
}

func applySubscript(current interface{}, sub pathIndex) (interface{}, bool) {
	if sub.kind == indexKindUnknown {
		// Cannot determine statically; assume available.
		return current, true
	}

	rv, ok := toReflectValue(current)
	if !ok {
		return nil, false
	}

	switch rv.Kind() {
	case reflect.Array, reflect.Slice:
		if sub.kind != indexKindInt {
			return nil, false
		}
		if sub.intValue < 0 || sub.intValue >= rv.Len() {
			return nil, false
		}
		return rv.Index(sub.intValue).Interface(), true
	case reflect.String:
		if sub.kind != indexKindInt {
			return nil, false
		}
		runes := []rune(rv.String())
		if sub.intValue < 0 || sub.intValue >= len(runes) {
			return nil, false
		}
		return string(runes[sub.intValue]), true
	case reflect.Map:
		if sub.kind != indexKindString {
			return nil, false
		}
		key, allowed := buildMapKey(sub.stringValue, rv.Type().Key())
		if !allowed {
			return nil, false
		}
		val := rv.MapIndex(key)
		if !val.IsValid() {
			return nil, false
		}
		return val.Interface(), true
	default:
		return nil, false
	}
}

func toExportedName(name string) string {
	if name == "" {
		return ""
	}

	parts := strings.Split(name, "_")
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, "")
}
