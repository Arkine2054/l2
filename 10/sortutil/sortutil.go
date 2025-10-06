package sortutil

import (
	"sort"
	"strconv"
	"strings"
)

// SortLines сортирует строки по заданным параметрам:
// k — номер колонки (1-based, 0 = вся строка)
// num — числовая сортировка
// reverse — обратный порядок
// unique — вывод только уникальных строк
func SortLines(lines []string, k int, num, reverse, unique bool) {
	if unique {
		lines = uniqueLines(lines)
	}

	less := func(i, j int) bool {
		a := getField(lines[i], k)
		b := getField(lines[j], k)

		if num {
			af, aerr := strconv.ParseFloat(a, 64)
			bf, berr := strconv.ParseFloat(b, 64)
			if aerr == nil && berr == nil {
				if reverse {
					return af > bf
				}
				return af < bf
			}
		}

		if reverse {
			return a > b
		}
		return a < b
	}

	sort.Slice(lines, less)
}

func getField(line string, k int) string {
	fields := strings.Split(line, "\t")
	if k <= 0 || k > len(fields) {
		return line
	}
	return fields[k-1]
}

func uniqueLines(lines []string) []string {
	seen := make(map[string]struct{}, len(lines))
	var result []string
	for _, line := range lines {
		if _, ok := seen[line]; !ok {
			seen[line] = struct{}{}
			result = append(result, line)
		}
	}
	return result
}
