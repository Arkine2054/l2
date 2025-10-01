package unpack

import (
	"errors"
	"strconv"
	"unicode"
)

func Unpack(s string) (string, error) {
	if s == "" {
		return "", nil
	}

	var result []rune
	runes := []rune(s)
	escaped := false

	for i := 0; i < len(runes); i++ {
		current := runes[i]

		if escaped {
			result = append(result, current)
			escaped = false

			if i+1 < len(runes) && unicode.IsDigit(runes[i+1]) {
				n, _ := strconv.Atoi(string(runes[i+1]))
				for j := 1; j < n; j++ {
					result = append(result, current)
				}
				i++
			}
			continue
		}

		if current == '\\' {
			escaped = true
			continue
		}

		if unicode.IsDigit(current) {
			return "", errors.New("строка некорректна: цифра без символа перед ней")
		}

		count := 1
		if i+1 < len(runes) && unicode.IsDigit(runes[i+1]) {
			n, _ := strconv.Atoi(string(runes[i+1]))
			count = n
			i++
		}

		for j := 0; j < count; j++ {
			result = append(result, current)
		}
	}
	if escaped {
		return "", errors.New("строка некорректна: символ '\\' без следующего")

	}

	return string(result), nil
}
