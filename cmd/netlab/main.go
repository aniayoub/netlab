package main

import (
	"fmt"
	"os"

	"github.com/aniayoub/netlab/internal/lab"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: netlab up|down")
		os.Exit(1)
	}

	action := os.Args[1]
	switch action {
	case "up":
		if err := lab.Setup(); err != nil {
			fmt.Println("Error running lab:", err)
			os.Exit(1)
		}
	case "down":
		if err := lab.Remove(); err != nil {
			fmt.Println("Error running lab remove:", err)
			os.Exit(1)
		}
	default:
		fmt.Println("Usage: netlab up|down")
		os.Exit(1)
	}
}
