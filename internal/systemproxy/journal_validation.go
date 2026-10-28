package systemproxy

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
)

func validateJournalChanges(changes []Change) error {
	allowed := map[string]bool{}
	for _, key := range keys {
		allowed[key] = true
	}
	for _, change := range changes {
		if strings.TrimSpace(change.ID) == "" || change.Expected == nil || change.Desired == nil || len(change.Keys) == 0 {
			return errors.New("恢复记录缺少服务标识、代理状态或变更字段")
		}
		for _, list := range [][]string{change.Keys, change.GuardKeys} {
			seen := map[string]bool{}
			for _, key := range list {
				if !allowed[key] || seen[key] {
					return errors.New("恢复记录包含无效或重复的代理字段")
				}
				seen[key] = true
			}
		}
		for _, settings := range []Settings{change.Expected, change.Desired} {
			for key, raw := range settings {
				if !allowed[key] {
					return errors.New("恢复记录包含未知代理字段")
				}
				if err := validateJournalValue(key, raw); err != nil {
					return fmt.Errorf("恢复记录中的 %s 无效：%w", key, err)
				}
			}
		}
	}
	return nil
}

func validateJournalValue(key string, raw json.RawMessage) error {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return err
	}
	valid := false
	switch {
	case key == "__ProtocolEnabled":
		_, valid = value.(bool)
	case strings.HasSuffix(key, "Enable"):
		if number, ok := value.(float64); ok {
			valid = number == 0 || number == 1
		} else {
			_, valid = value.(bool)
		}
	case strings.HasSuffix(key, "Port"):
		if number, ok := value.(float64); ok {
			valid = number >= 0 && number <= 65535 && math.Trunc(number) == number
		}
	case key == "ExceptionsList":
		if entries, ok := value.([]any); ok {
			valid = true
			for _, entry := range entries {
				if _, ok := entry.(string); !ok {
					valid = false
					break
				}
			}
		}
	default:
		_, valid = value.(string)
	}
	if !valid {
		return errors.New("字段类型或取值范围不正确")
	}
	return nil
}
