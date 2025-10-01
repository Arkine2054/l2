package main

import (
	"fmt"
	"github.com/beevik/ntp"
	"os"
)

func main() {
	currentTime, err := ntp.Time("0.pool.ntp.org")
	if err != nil {
		fmt.Fprintf(os.Stderr, "ntp.Time: %s\n", err)
		os.Exit(1)
	}
	fmt.Println("CurrentTime:", currentTime)
}
