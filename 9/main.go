package main

import (
	"fmt"
	"gitlab.com/arkine/l2/9/unpack"
	"log"
)

func main() {
	tests := []string{
		"a4bc2d5e",
		"abcd",
		"45",
		"",
		"qwe\\4\\5",
		"qwe\\45",
	}

	for _, t := range tests {
		res, err := unpack.Unpack(t)
		if err != nil {
			log.Printf("Unpack(%q) -> ошибка: %v", t, err)
		} else {
			fmt.Printf("Unpack(%q) -> %q\n", t, res)
		}
	}
}
