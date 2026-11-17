package theme

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
)

// TextMate themes use XML property lists; the decoder never resolves external
// entities or fetches the document's DTD.
func readPlist(data []byte) (themeDocument, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	var parse func(xml.StartElement, int) (any, error)
	parse = func(start xml.StartElement, depth int) (any, error) {
		if depth > 64 {
			return nil, fmt.Errorf("plist depth limit")
		}
		switch start.Name.Local {
		case "plist":
			tok, err := dec.Token()
			if err != nil {
				return nil, err
			}
			for {
				if child, ok := tok.(xml.StartElement); ok {
					return parse(child, depth+1)
				}
				tok, err = dec.Token()
				if err != nil {
					return nil, err
				}
			}
		case "dict":
			result := map[string]any{}
			var key string
			haveKey := false
			for {
				tok, err := dec.Token()
				if err != nil {
					return nil, err
				}
				switch el := tok.(type) {
				case xml.EndElement:
					if haveKey {
						return nil, fmt.Errorf("plist missing value")
					}
					return result, nil
				case xml.StartElement:
					if !haveKey {
						if el.Name.Local != "key" {
							return nil, fmt.Errorf("plist expected key")
						}
						if err = dec.DecodeElement(&key, &el); err != nil {
							return nil, err
						}
						haveKey = true
					} else {
						value, err := parse(el, depth+1)
						if err != nil {
							return nil, err
						}
						result[key] = value
						haveKey = false
					}
				}
			}
		case "array":
			result := []any{}
			for {
				tok, err := dec.Token()
				if err != nil {
					return nil, err
				}
				switch el := tok.(type) {
				case xml.EndElement:
					return result, nil
				case xml.StartElement:
					value, err := parse(el, depth+1)
					if err != nil {
						return nil, err
					}
					result = append(result, value)
				}
			}
		case "true", "false":
			if err := dec.Skip(); err != nil {
				return nil, err
			}
			return start.Name.Local == "true", nil
		default:
			var s string
			err := dec.DecodeElement(&s, &start)
			return s, err
		}
	}
	for {
		tok, err := dec.Token()
		if err != nil {
			return themeDocument{}, err
		}
		if start, ok := tok.(xml.StartElement); ok {
			value, err := parse(start, 0)
			if err != nil {
				return themeDocument{}, err
			}
			for {
				trailing, err := dec.Token()
				if err == io.EOF {
					break
				}
				if err != nil {
					return themeDocument{}, err
				}
				if _, ok := trailing.(xml.StartElement); ok {
					return themeDocument{}, fmt.Errorf("unexpected trailing plist element")
				}
			}
			b, err := json.Marshal(value)
			if err != nil {
				return themeDocument{}, err
			}
			var result themeDocument
			err = json.Unmarshal(b, &result)
			if err == nil && result.Settings == nil {
				err = fmt.Errorf("tmTheme requires a settings array")
			}
			return result, err
		}
	}
}
