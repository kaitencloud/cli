package output

import (
	"bytes"
	"encoding/json"

	"gopkg.in/yaml.v3"
)

// jsonDocument returns value as the YAML tree of its JSON encoding. The API's types describe
// themselves in JSON only: the names of their fields, the fields they leave out and the
// typed values they hold are spelled by json tags and MarshalJSON methods, none of which a
// YAML encoder reads. Left to itself it names a field after the Go one (externalcustomerid),
// prints every absent field as null and a typed value as an empty mapping. Going through
// JSON makes --output yaml the document --output json prints, key for key and in the same
// order.
func jsonDocument(value any) (*yaml.Node, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	// A number is carried over as written, never through a float64 that would round a
	// large integer or respell a decimal.
	decoder.UseNumber()

	return jsonNode(decoder)
}

// jsonNode reads the next JSON value from decoder as a YAML node.
func jsonNode(decoder *json.Decoder) (*yaml.Node, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}

	switch typed := token.(type) {
	case json.Delim:
		node := &yaml.Node{Kind: yaml.MappingNode}
		if typed == '[' {
			node.Kind = yaml.SequenceNode
		}
		// An object's tokens alternate between a key and its value, which is the order a
		// mapping node holds them in, so one loop reads an object and an array alike.
		for decoder.More() {
			child, err := jsonNode(decoder)
			if err != nil {
				return nil, err
			}
			node.Content = append(node.Content, child)
		}
		// The closing delimiter.
		if _, err := decoder.Token(); err != nil {
			return nil, err
		}
		return node, nil
	case json.Number:
		return &yaml.Node{Kind: yaml.ScalarNode, Value: typed.String()}, nil
	default:
		// A string, a boolean or null: the encoder knows how each is written, and which
		// strings need quoting to stay strings.
		node := &yaml.Node{}
		if err := node.Encode(typed); err != nil {
			return nil, err
		}
		return node, nil
	}
}
