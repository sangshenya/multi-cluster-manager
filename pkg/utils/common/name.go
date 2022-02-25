package common

import (
	"fmt"
)

func GenerateName(part1 string, part2 string) (string, error) {
	if part1 == "" || part2 == "" {
		return "", fmt.Errorf("parameter cannot be empty")
	}
	res := part1 + part2
	return res, nil
}

func GenerateNameByOption(part1 string, part2 string, option string) (string, error) {
	if part1 == "" || part2 == "" || option == "" {
		return "", fmt.Errorf("parameter cannot be empty")
	}
	res := part1 + option + part2
	return res, nil
}
