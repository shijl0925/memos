package api

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type CreatedTsOperator string

const (
	CreatedTsGreaterThanOrEqual CreatedTsOperator = ">="
	CreatedTsGreaterThan        CreatedTsOperator = ">"
	CreatedTsLessThanOrEqual    CreatedTsOperator = "<="
	CreatedTsLessThan           CreatedTsOperator = "<"
)

type ShortcutCondition struct {
	Type      string
	Values    []string
	Value     string
	Timestamp int64
	Operator  CreatedTsOperator
}

func ApplyShortcutFilter(find *MemoFind, filter string) error {
	conditions, err := ParseShortcutFilter(filter)
	if err != nil {
		return err
	}

	for _, condition := range conditions {
		switch condition.Type {
		case "TAG_IN":
			find.TagSearchList = append(find.TagSearchList, condition.Values...)
		case "CONTENT_CONTAINS":
			find.ContentContainsList = append(find.ContentContainsList, condition.Value)
		case "VISIBILITY_IN":
			visibilityList := make([]Visibility, 0, len(condition.Values))
			for _, visibility := range condition.Values {
				visibilityList = append(visibilityList, Visibility(strings.ToUpper(visibility)))
			}
			find.VisibilityList = visibilityList
		case "HAS_LINK":
			value := true
			find.HasLink = &value
		case "HAS_TASK_LIST":
			value := true
			find.HasTaskList = &value
		case "HAS_CODE":
			value := true
			find.HasCode = &value
		case "PINNED":
			value := true
			find.Pinned = &value
		case "CREATED_TS_COMPARE":
			value := normalizeTimestampToSeconds(condition.Timestamp)
			switch condition.Operator {
			case CreatedTsGreaterThanOrEqual:
				find.CreatedTsAfter = &value
			case CreatedTsGreaterThan:
				find.CreatedTsGreaterThan = &value
			case CreatedTsLessThanOrEqual:
				find.CreatedTsLessThanOrEqualTo = &value
			case CreatedTsLessThan:
				find.CreatedTsBefore = &value
			}
		}
	}

	return nil
}

func ParseShortcutFilter(filter string) ([]ShortcutCondition, error) {
	parts := splitShortcutConditions(filter)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty shortcut filter")
	}

	conditions := make([]ShortcutCondition, 0, len(parts))
	for _, part := range parts {
		condition, err := parseShortcutCondition(part)
		if err != nil {
			return nil, err
		}
		conditions = append(conditions, condition)
	}
	return conditions, nil
}

func splitShortcutConditions(filter string) []string {
	conditions := []string{}
	current := strings.Builder{}
	inString := false
	bracketDepth := 0

	for i := 0; i < len(filter); i++ {
		char := filter[i]
		var next byte
		if i+1 < len(filter) {
			next = filter[i+1]
		}

		if char == '"' && (i == 0 || filter[i-1] != '\\') {
			inString = !inString
		}
		if !inString {
			switch char {
			case '[':
				bracketDepth++
			case ']':
				bracketDepth--
			case '&':
				if next == '&' && bracketDepth == 0 {
					if value := strings.TrimSpace(current.String()); value != "" {
						conditions = append(conditions, value)
					}
					current.Reset()
					i++
					continue
				}
			}
		}

		_ = current.WriteByte(char)
	}

	if value := strings.TrimSpace(current.String()); value != "" {
		conditions = append(conditions, value)
	}
	return conditions
}

func parseShortcutCondition(condition string) (ShortcutCondition, error) {
	lowerCondition := strings.ToLower(condition)
	if strings.HasPrefix(lowerCondition, "tag in ") {
		values := condition[len("tag in "):]
		list, err := parseShortcutStringArray(values)
		if err != nil || len(list) == 0 {
			return ShortcutCondition{}, fmt.Errorf("invalid tag condition")
		}
		return ShortcutCondition{Type: "TAG_IN", Values: list}, nil
	}

	if strings.HasPrefix(lowerCondition, "content.contains(") && strings.HasSuffix(condition, ")") {
		raw := strings.TrimSpace(condition[len("content.contains(") : len(condition)-1])
		value, err := parseShortcutString(raw)
		if err != nil || value == "" {
			return ShortcutCondition{}, fmt.Errorf("invalid content condition")
		}
		return ShortcutCondition{Type: "CONTENT_CONTAINS", Value: value}, nil
	}

	if strings.HasPrefix(lowerCondition, "visibility in ") {
		values := condition[len("visibility in "):]
		list, err := parseShortcutStringArray(values)
		if err != nil || len(list) == 0 {
			return ShortcutCondition{}, fmt.Errorf("invalid visibility condition")
		}
		return ShortcutCondition{Type: "VISIBILITY_IN", Values: list}, nil
	}

	switch condition {
	case "has_link":
		return ShortcutCondition{Type: "HAS_LINK"}, nil
	case "has_task_list":
		return ShortcutCondition{Type: "HAS_TASK_LIST"}, nil
	case "has_code":
		return ShortcutCondition{Type: "HAS_CODE"}, nil
	case "pinned":
		return ShortcutCondition{Type: "PINNED"}, nil
	}

	for _, operator := range []CreatedTsOperator{CreatedTsGreaterThanOrEqual, CreatedTsLessThanOrEqual, CreatedTsGreaterThan, CreatedTsLessThan} {
		prefix := "created_ts " + string(operator)
		if strings.HasPrefix(lowerCondition, prefix) {
			rawValue := strings.TrimSpace(condition[len(prefix):])
			timestamp, err := strconv.ParseInt(rawValue, 10, 64)
			if err != nil {
				return ShortcutCondition{}, fmt.Errorf("invalid created_ts condition")
			}
			return ShortcutCondition{Type: "CREATED_TS_COMPARE", Operator: operator, Timestamp: timestamp}, nil
		}
	}

	return ShortcutCondition{}, fmt.Errorf("unsupported shortcut condition: %s", condition)
}

func parseShortcutStringArray(value string) ([]string, error) {
	values := []string{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(value)), &values); err != nil {
		return nil, err
	}
	return values, nil
}

func parseShortcutString(value string) (string, error) {
	var parsed string
	if err := json.Unmarshal([]byte(value), &parsed); err != nil {
		return "", err
	}
	return parsed, nil
}

func normalizeTimestampToSeconds(value int64) int64 {
	if value >= 10_000_000_000 {
		return value / 1000
	}
	return value
}
