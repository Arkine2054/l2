package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

type intSet map[int]struct{}

func parseFields(spec string) intSet {
	fields := make(intSet)
	parts := strings.Split(spec, ",")
	for _, part := range parts {
		if strings.Contains(part, "-") {
			bounds := strings.SplitN(part, "-", 2)
			if len(bounds) != 2 {
				continue
			}
			start, err1 := strconv.Atoi(bounds[0])
			end, err2 := strconv.Atoi(bounds[1])
			if err1 != nil || err2 != nil || start <= 0 || end < start {
				continue
			}
			for i := start; i <= end; i++ {
				fields[i-1] = struct{}{}
			}
		} else {
			idx, err := strconv.Atoi(part)
			if err != nil || idx <= 0 {
				continue
			}
			fields[idx-1] = struct{}{}
		}
	}
	return fields
}

func cutStream(delim string, fields intSet, separated bool) {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()

		if !strings.Contains(line, delim) {
			if separated {
				continue
			}
			fmt.Println(line)
			continue
		}

		parts := strings.Split(line, delim)
		var selected []string

		indexes := make([]int, 0, len(fields))
		for idx := range fields {
			indexes = append(indexes, idx)
		}
		sort.Ints(indexes)

		for _, idx := range indexes {
			if idx < len(parts) {
				selected = append(selected, parts[idx])
			}
		}
		fmt.Println(strings.Join(selected, delim))
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "ошибка чтения:", err)
	}
}

func main() {
	fieldsSpec := flag.String("f", "", "Номера полей через запятую (можно диапазоны, например 1,3-5)")
	delim := flag.String("d", "\t", "Разделитель (по умолчанию табуляция)")
	separated := flag.Bool("s", false, "Игнорировать строки без разделителя")
	flag.Parse()

	if *fieldsSpec == "" {
		fmt.Fprintln(os.Stderr, "Ошибка: нужно указать флаг -f")
		os.Exit(1)
	}

	fields := parseFields(*fieldsSpec)
	if len(fields) == 0 {
		fmt.Fprintln(os.Stderr, "Ошибка: не указаны корректные номера полей")
		os.Exit(1)
	}

	cutStream(*delim, fields, *separated)
}
