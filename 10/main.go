package main

import (
	"flag"
	"fmt"
	"gitlab.com/arkine/l2/10/sortutil"
	"os"
)

func main() {
	k := flag.Int("k", 0, "column to sort by (1-based)")
	num := flag.Bool("n", false, "numeric sort")
	reverse := flag.Bool("r", false, "reverse sort")
	unique := flag.Bool("u", false, "unique lines")
	flag.Parse()

	lines := flag.Args()
	if len(lines) == 0 {
		fmt.Fprintln(os.Stderr, "Нет строк для сортировки")
		os.Exit(1)
	}

	sortutil.SortLines(lines, *k, *num, *reverse, *unique)

	for _, line := range lines {
		fmt.Println(line)
	}
}
