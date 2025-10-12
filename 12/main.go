package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

type options struct {
	after      int
	before     int
	countOnly  bool
	ignoreCase bool
	invert     bool
	fixed      bool
	showLine   bool
	pattern    string
	file       string
}

func parseFlags() options {
	opts := options{}
	flag.IntVar(&opts.after, "A", 0, "Print N lines of trailing context")
	flag.IntVar(&opts.before, "B", 0, "Print N lines of leading context")
	C := flag.Int("C", 0, "Print N lines of output context")
	flag.BoolVar(&opts.countOnly, "c", false, "Print only a count of matching lines")
	flag.BoolVar(&opts.ignoreCase, "i", false, "Ignore case distinctions")
	flag.BoolVar(&opts.invert, "v", false, "Select non-matching lines")
	flag.BoolVar(&opts.fixed, "F", false, "Interpret pattern as a fixed string")
	flag.BoolVar(&opts.showLine, "n", false, "Print line number with output lines")

	flag.Parse()

	if *C > 0 {
		opts.after = *C
		opts.before = *C
	}

	args := flag.Args()
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: mygrep [OPTIONS] PATTERN [FILE]")
		os.Exit(1)
	}
	opts.pattern = args[0]
	if len(args) > 1 {
		opts.file = args[1]
	}
	return opts
}

func compileMatcher(opts options) func(string) bool {
	if opts.fixed {
		pattern := opts.pattern
		if opts.ignoreCase {
			pattern = strings.ToLower(pattern)
		}
		return func(s string) bool {
			if opts.ignoreCase {
				return strings.Contains(strings.ToLower(s), pattern)
			}
			return strings.Contains(s, pattern)
		}
	}

	pat := opts.pattern
	if opts.ignoreCase {
		pat = "(?i)" + pat
	}
	re, err := regexp.Compile(pat)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid regex: %v\n", err)
		os.Exit(1)
	}
	return func(s string) bool {
		return re.MatchString(s)
	}
}

func grep(r io.Reader, opts options) {
	scanner := bufio.NewScanner(r)
	lines := []string{}
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "error reading input: %v\n", err)
		os.Exit(1)
	}

	matcher := compileMatcher(opts)

	matchIdx := map[int]bool{}
	for i, line := range lines {
		match := matcher(line)
		if opts.invert {
			match = !match
		}
		if match {
			matchIdx[i] = true
			for j := 1; j <= opts.before; j++ {
				if i-j >= 0 {
					matchIdx[i-j] = true
				}
			}
			for j := 1; j <= opts.after; j++ {
				if i+j < len(lines) {
					matchIdx[i+j] = true
				}
			}
		}
	}

	if opts.countOnly {
		count := 0
		for _, line := range lines {
			match := matcher(line)
			if opts.invert {
				match = !match
			}
			if match {
				count++
			}
		}
		fmt.Println(count)
		return
	}

	for i, line := range lines {
		if matchIdx[i] {
			if opts.showLine {
				fmt.Printf("%d:%s\n", i+1, line)
			} else {
				fmt.Println(line)
			}
		}
	}
}

func main() {
	opts := parseFlags()

	var r io.Reader
	if opts.file != "" {
		f, err := os.Open(opts.file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error opening file: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		r = f
	} else {
		r = os.Stdin
	}

	grep(r, opts)
}
