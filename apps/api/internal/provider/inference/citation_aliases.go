package inference

import (
	"encoding/json"
	"strconv"

	port "github.com/Ilia-Shakeri/stackplore/apps/api/internal/inference"
)

// Compact handles exist only inside one counted request. They cannot select a
// read, change the sealed reference set or escape into a published answer.
func compactReferences(references []string) ([]string, map[string]string, error) {
	if _, err := port.AnswerSchemaForReferences(references); err != nil {
		return nil, nil, err
	}
	aliases := make([]string, len(references))
	bindings := make(map[string]string, len(references))
	for index, reference := range references {
		alias := "e" + strconv.Itoa(index+1)
		aliases[index], bindings[alias] = alias, reference
	}
	return aliases, bindings, nil
}

func canonicalAnswer(contents []byte, bindings map[string]string) ([]byte, error) {
	if len(contents) > port.MaximumOutputBytes {
		return nil, port.ErrOutputLimit
	}
	var answer port.Answer
	if decodeClosed(contents, &answer) != nil {
		return nil, port.ErrInvalidOutput
	}
	allowed := make(map[string]struct{}, len(bindings))
	for alias := range bindings {
		allowed[alias] = struct{}{}
	}
	if port.ValidateAnswer(answer, allowed) != nil {
		return nil, port.ErrInvalidOutput
	}
	for index := range answer.Sections {
		for citation, alias := range answer.Sections[index].Citations {
			answer.Sections[index].Citations[citation] = bindings[alias]
		}
	}
	encoded, err := json.Marshal(answer)
	if err != nil || len(encoded) > port.MaximumOutputBytes {
		clear(encoded)
		return nil, port.ErrOutputLimit
	}
	return encoded, nil
}
