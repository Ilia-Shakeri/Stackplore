package inference

import (
	"encoding/json"
	"strconv"

	port "github.com/Ilia-Shakeri/stackplore/apps/api/internal/inference"
)

// Compact handles exist only inside one counted request. They cannot select a
// read, change the sealed reference set or escape into a published answer.
func compactReferences(references []string) ([]string, map[string]string, error) {
	return compactEvidenceReferences(references, nil)
}

func compactEvidenceReferences(references, guidance []string) ([]string, map[string]string, error) {
	if len(guidance) > len(references) {
		return nil, nil, port.ErrInvalid
	}
	if _, err := port.AnswerSchemaForReferences(references); err != nil {
		return nil, nil, err
	}
	aliases := make([]string, len(references))
	bindings := make(map[string]string, len(references))
	known := make(map[string]bool, len(references))
	for _, reference := range references {
		known[reference] = true
	}
	guides := make(map[string]bool, len(guidance))
	for _, id := range guidance {
		if !known[id] || guides[id] {
			return nil, nil, port.ErrInvalid
		}
		guides[id] = true
	}
	observations, documents := 0, 0
	for index, reference := range references {
		var alias string
		if guides[reference] {
			documents++
			alias = "g" + strconv.Itoa(documents)
		} else {
			observations++
			alias = "e" + strconv.Itoa(observations)
		}
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
