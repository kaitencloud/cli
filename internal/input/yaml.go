package input

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	yamlMergeTag     = "!!merge"
	yamlNullTag      = "!!null"
	yamlStringTag    = "!!str"
	yamlTimestampTag = "!!timestamp"
)

// parseYAML reads data as one YAML document. It fails on what is not YAML at all, and on
// nothing else: whether the document fits the request is decodeYAML's to say.
func parseYAML(data []byte) (*yaml.Node, error) {
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, err
	}
	return &document, nil
}

// decodeYAML decodes document into a value of type T as the JSON document it stands for.
//
// The types of the API describe themselves in JSON only: the names of their fields come
// from json tags or from matching a key whatever its case, and a typed value reads itself
// through UnmarshalJSON. A YAML decoder reads none of that. It matched the keys of one word
// by accident and dropped every other without an error -- externalCustomerId, default_variant
// -- so the request went out incomplete. Decoding the JSON the document stands for gives a
// YAML file the request the same file written in JSON makes.
//
// A key the request has no field for is an error rather than a drop, which is the failure
// this replaces.
func decodeYAML[T any](document *yaml.Node) (T, error) {
	var zero T

	// An empty document has no node to decode, and stands for null.
	var generic any
	if document.Kind != 0 {
		keepText(document, reflect.TypeFor[T]())
		if err := document.Decode(&generic); err != nil {
			return zero, err
		}
	}

	raw, err := json.Marshal(generic)
	if err != nil {
		var unsupported *json.UnsupportedTypeError
		if errors.As(err, &unsupported) {
			// The one thing a YAML document holds that JSON cannot: a mapping whose keys are
			// not text.
			return zero, errors.New("a key must be text")
		}
		return zero, asYAMLError(err)
	}

	var out T
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&out); err != nil {
		return zero, asYAMLError(err)
	}
	return out, nil
}

// asYAMLError reports a failure to read the JSON a YAML document stands for without naming
// JSON, which the operator did not write.
func asYAMLError(err error) error {
	return errors.New(strings.TrimPrefix(err.Error(), "json: "))
}

// keepText marks as text the plain scalars of node that reading the document as JSON would
// otherwise change, so that each reaches the request as it was written:
//
//   - a key, since the keys of a JSON object are text: 1024 is the key "1024";
//   - a date or a time, which JSON has no type for and carries as text: left alone it
//     becomes a time and comes back respelled, 2026-08-01 as 2026-08-01T00:00:00Z;
//   - a value bound for a text field: version: 1.0 is the text 1.0 there, not the number 1
//     that JSON refuses for it.
//
// bound is the Go type the node is decoded into, or nil when nothing is known of it. A
// quoted or explicitly tagged scalar already says what it is and is left alone.
func keepText(node *yaml.Node, bound reflect.Type) {
	for bound != nil && bound.Kind() == reflect.Pointer {
		bound = bound.Elem()
	}
	if bound != nil && reflect.PointerTo(bound).Implements(reflect.TypeFor[json.Unmarshaler]()) {
		// The type reads its own JSON: what it holds is not for its Go fields to tell.
		bound = nil
	}

	switch node.Kind {
	case yaml.DocumentNode:
		for _, content := range node.Content {
			keepText(content, bound)
		}
	case yaml.SequenceNode:
		for _, item := range node.Content {
			keepText(item, elementOf(bound))
		}
	case yaml.MappingNode:
		for i := 0; i+1 < len(node.Content); i += 2 {
			key, value := node.Content[i], node.Content[i+1]
			if key.Kind == yaml.ScalarNode && key.Tag != yamlMergeTag && isPlain(key) {
				key.Tag = yamlStringTag
			}
			keepText(value, memberOf(bound, key.Value))
		}
	case yaml.ScalarNode:
		if !isPlain(node) || node.Tag == yamlNullTag {
			return
		}
		if node.Tag == yamlTimestampTag || (bound != nil && bound.Kind() == reflect.String) {
			node.Tag = yamlStringTag
		}
	}
}

// isPlain reports whether a scalar was written bare: neither quoted nor given a tag, which
// are the two ways a document says what a value is.
func isPlain(node *yaml.Node) bool {
	const explicit = yaml.TaggedStyle | yaml.DoubleQuotedStyle | yaml.SingleQuotedStyle | yaml.LiteralStyle | yaml.FoldedStyle
	return node.Style&explicit == 0
}

// elementOf returns the type of the items of a list type, or nil.
func elementOf(bound reflect.Type) reflect.Type {
	if bound == nil {
		return nil
	}
	if bound.Kind() == reflect.Slice || bound.Kind() == reflect.Array {
		return bound.Elem()
	}
	return nil
}

// memberOf returns the type of what key holds in a map or a struct type, or nil. For a
// struct it is the field JSON gives the key to: the one named by its json tag, or by its
// own name, whatever the case.
func memberOf(bound reflect.Type, key string) reflect.Type {
	if bound == nil {
		return nil
	}

	switch bound.Kind() {
	case reflect.Map:
		return bound.Elem()
	case reflect.Struct:
		for i := range bound.NumField() {
			field := bound.Field(i)
			if !field.IsExported() {
				continue
			}
			name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
			if name == "" {
				name = field.Name
			}
			if strings.EqualFold(name, key) {
				return field.Type
			}
		}
	}
	return nil
}
