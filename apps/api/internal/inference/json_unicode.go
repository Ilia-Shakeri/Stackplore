package inference

// encoding/json repairs unpaired UTF-16 escapes. Reject them before decoding
// so malformed processor output cannot become a different accepted answer.
func scalarJSONStringEncoding(contents []byte) bool {
	quoted := false
	for index := 0; index < len(contents); index++ {
		if contents[index] == '"' {
			quoted = !quoted
			continue
		}
		if !quoted || contents[index] != '\\' {
			continue
		}
		index++
		if index >= len(contents) {
			return false
		}
		if contents[index] != 'u' {
			continue
		}
		unit, ok := jsonHexUnit(contents, index+1)
		if !ok {
			return false
		}
		index += 4
		if unit >= 0xdc00 && unit <= 0xdfff {
			return false
		}
		if unit >= 0xd800 && unit <= 0xdbff {
			if index+6 >= len(contents) || contents[index+1] != '\\' || contents[index+2] != 'u' {
				return false
			}
			low, ok := jsonHexUnit(contents, index+3)
			if !ok || low < 0xdc00 || low > 0xdfff {
				return false
			}
			index += 6
		}
	}
	return !quoted
}

func jsonHexUnit(contents []byte, start int) (uint16, bool) {
	if start < 0 || start > len(contents)-4 {
		return 0, false
	}
	var unit uint16
	for _, value := range contents[start : start+4] {
		var digit byte
		switch {
		case value >= '0' && value <= '9':
			digit = value - '0'
		case value >= 'a' && value <= 'f':
			digit = value - 'a' + 10
		case value >= 'A' && value <= 'F':
			digit = value - 'A' + 10
		default:
			return 0, false
		}
		unit = unit<<4 | uint16(digit)
	}
	return unit, true
}
