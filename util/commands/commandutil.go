package commands

import (
	"errors"
	"strings"
)

func TrimSplitCommand(command string) (string, []string, error) {
	command = strings.TrimSpace(command)
	parts := []string{}
	escaped := false
	stringmode := false
	str := ""
	for _, c := range command {
		if c != ' ' || stringmode {
			if c == '\\' && !escaped {
				escaped = true
				continue
			}
			if c == '"' && !escaped {
				stringmode = !stringmode
				continue
			}
			str += string(c)
			escaped = false
			continue
		}
		if c == ' ' {
			parts = append(parts, str)
			str = ""
		}
	}
	if len(str) > 0 {
		parts = append(parts, str)
	}
	if stringmode {
		return "", nil, errors.New("Malformed string, missing closing quotation mark")
	}
	if len(parts) == 0 {
		return "", nil, nil
	}
	return parts[0], parts[1:], nil
}
