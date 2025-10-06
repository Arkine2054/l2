package main

import (
	"fmt"
	"sort"
	"strings"
)

func sortString(s string) string {
	r := []rune(s)
	sort.Slice(r, func(i, j int) bool {
		return r[i] < r[j]
	})
	return string(r)
}

func findAnagramSets(words []string) map[string][]string {
	anagrams := make(map[string][]string)
	firstWord := make(map[string]string)

	for _, w := range words {
		word := strings.ToLower(w)
		key := sortString(word)
		anagrams[key] = append(anagrams[key], word)
		if _, ok := firstWord[key]; !ok {
			firstWord[key] = word
		}
	}

	result := make(map[string][]string)
	for key, group := range anagrams {
		if len(group) > 1 {
			sort.Strings(group)
			result[firstWord[key]] = group
		}
	}
	return result
}

func main() {
	input := []string{"пятак", "пятка", "тяпка", "листок", "слиток", "столик", "стол"}
	result := findAnagramSets(input)

	for k, v := range result {
		fmt.Printf("%q: %v\n", k, v)
	}
}
