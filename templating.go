package renderfs

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/flosch/pongo2/v6"
)

var (
	templateCache sync.Map // map[string]*pongo2.Template

	expressionBlockRegex = regexp.MustCompile(`{{-?([^{}]+?)-?}}`)
	tagBlockRegex        = regexp.MustCompile(`{%-?([^{}]+?)-?%}`)
)

var (
	skipBaseIdentifiers = map[string]struct{}{
		"true":    {},
		"false":   {},
		"none":    {},
		"null":    {},
		"not":     {},
		"and":     {},
		"or":      {},
		"in":      {},
		"as":      {},
		"for":     {},
		"end":     {},
		"if":      {},
		"elif":    {},
		"else":    {},
		"set":     {},
		"block":   {},
		"scoped":  {},
		"with":    {},
		"import":  {},
		"from":    {},
		"macro":   {},
		"call":    {},
		"loop":    {},
		"forloop": {},
		"super":   {},
		"self":    {},
		"pongo2":  {}, // provided automatically
	}
)

func renderTemplateString(tpl string, ctx pongo2.Context) (string, error) {
	if err := ensureVariablesPresent(tpl, ctx); err != nil {
		return "", err
	}

	compiled, err := getOrCompileTemplate(tpl)
	if err != nil {
		return "", err
	}

	out, err := compiled.Execute(ctx)
	if err != nil {
		return "", err
	}
	return out, nil
}

func getOrCompileTemplate(tpl string) (*pongo2.Template, error) {
	if cached, ok := templateCache.Load(tpl); ok {
		return cached.(*pongo2.Template), nil
	}

	compiled, err := pongo2.FromString(tpl)
	if err != nil {
		return nil, err
	}

	templateCache.Store(tpl, compiled)
	return compiled, nil
}

func ensureVariablesPresent(tpl string, ctx pongo2.Context) error {
	candidates := collectVariableCandidates(tpl)
	if len(candidates) == 0 {
		return nil
	}

	for _, candidate := range candidates {
		if _, skip := skipBaseIdentifiers[candidate.base]; skip {
			continue
		}
		if ok := resolvePath(ctx, candidate.path); !ok {
			return fmt.Errorf("renderfs: missing context value for '%s'", candidate.path)
		}
	}
	return nil
}

type variableCandidate struct {
	path string
	base string
}

func collectVariableCandidates(tpl string) []variableCandidate {
	var result []variableCandidate
	for _, match := range expressionBlockRegex.FindAllStringSubmatch(tpl, -1) {
		expr := strings.TrimSpace(match[1])
		result = append(result, extractVariablesFromExpression(expr)...)
	}
	for _, match := range tagBlockRegex.FindAllStringSubmatch(tpl, -1) {
		expr := strings.TrimSpace(match[1])
		result = append(result, extractVariablesFromExpression(expr)...)
	}
	dedup := make(map[string]variableCandidate)
	for _, candidate := range result {
		if _, exists := dedup[candidate.path]; !exists {
			dedup[candidate.path] = candidate
		}
	}

	out := make([]variableCandidate, 0, len(dedup))
	for _, candidate := range dedup {
		out = append(out, candidate)
	}
	return out
}

type tokenType int

const (
	tokenIdentifier tokenType = iota + 1
	tokenNumber
	tokenString
	tokenSymbol
)

type token struct {
	typ   tokenType
	value string
}

func extractVariablesFromExpression(expr string) []variableCandidate {
	tokens := tokenize(expr)
	var candidates []variableCandidate
	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		if tok.typ != tokenIdentifier {
			continue
		}

		if shouldSkipIdentifier(tokens, i) {
			continue
		}

		pathBuilder := strings.Builder{}
		pathBuilder.WriteString(tok.value)
		j := i + 1
		for j < len(tokens) {
			switch tokens[j].typ {
			case tokenSymbol:
				switch tokens[j].value {
				case ".":
					if j+1 < len(tokens) && tokens[j+1].typ == tokenIdentifier {
						pathBuilder.WriteString(".")
						pathBuilder.WriteString(tokens[j+1].value)
						j += 2
						continue
					}
				case "[":
					closing := findClosingBracket(tokens, j)
					if closing == -1 {
						j = len(tokens)
						continue
					}
					pathBuilder.WriteString(buildBracketNotation(tokens[j : closing+1]))
					j = closing + 1
					continue
				default:
				}
			}
			break
		}

		// Skip if next token is '(' (function call)
		if j < len(tokens) && tokens[j].typ == tokenSymbol && tokens[j].value == "(" {
			continue
		}

		fullPath := pathBuilder.String()
		if fullPath == "" {
			continue
		}

		candidates = append(candidates, variableCandidate{
			path: fullPath,
			base: tok.value,
		})
	}

	return candidates
}

func tokenize(expr string) []token {
	var tokens []token
	for i := 0; i < len(expr); {
		switch {
		case isWhitespace(expr[i]):
			i++
		case isIdentifierStart(expr[i]):
			start := i
			i++
			for i < len(expr) && isIdentifierPart(expr[i]) {
				i++
			}
			tokens = append(tokens, token{typ: tokenIdentifier, value: expr[start:i]})
		case isDigit(expr[i]):
			start := i
			i++
			for i < len(expr) && (isDigit(expr[i]) || expr[i] == '.') {
				i++
			}
			tokens = append(tokens, token{typ: tokenNumber, value: expr[start:i]})
		case expr[i] == '"' || expr[i] == '\'':
			quote := expr[i]
			start := i
			i++
			for i < len(expr) {
				if expr[i] == '\\' && i+1 < len(expr) {
					i += 2
					continue
				}
				if expr[i] == quote {
					i++
					break
				}
				i++
			}
			tokens = append(tokens, token{typ: tokenString, value: expr[start:i]})
		default:
			tokens = append(tokens, token{typ: tokenSymbol, value: string(expr[i])})
			i++
		}
	}
	return tokens
}

func isWhitespace(b byte) bool {
	return b == ' ' || b == '\n' || b == '\r' || b == '\t'
}

func isIdentifierStart(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || b == '_'
}

func isIdentifierPart(b byte) bool {
	return isIdentifierStart(b) || isDigit(b)
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

func shouldSkipIdentifier(tokens []token, idx int) bool {
	tok := tokens[idx]
	if idx == 0 {
		lower := strings.ToLower(tok.value)
		if strings.HasPrefix(lower, "end") {
			return true
		}
	}

	if idx == 0 {
		return false
	}

	prev := tokens[idx-1]
	if prev.typ == tokenSymbol {
		switch prev.value {
		case ".", "|":
			return true
		case ":":
			// e.g. for key:value pairs; ignore the key
			return true
		}
	}

	if prev.typ == tokenIdentifier {
		switch prev.value {
		case "for", "set", "block", "macro", "call", "as":
			return true
		}
	}

	return false
}

func findClosingBracket(tokens []token, openIdx int) int {
	depth := 0
	for i := openIdx; i < len(tokens); i++ {
		tok := tokens[i]
		if tok.typ != tokenSymbol {
			continue
		}
		switch tok.value {
		case "[":
			depth++
		case "]":
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func buildBracketNotation(tokens []token) string {
	var b strings.Builder
	for _, tok := range tokens {
		b.WriteString(tok.value)
	}
	return b.String()
}
