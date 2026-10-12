package main

import (
	"fmt"
	"os"

	"github.com/aniayoub/netlab/internal/config"
	"github.com/aniayoub/netlab/internal/lab"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: netlab up|down <config-path>")
		os.Exit(1)
	}

	configPath := os.Args[2]
	cfg, err := config.Parse(configPath)
	if err != nil {
		fmt.Println("Error parsing config:", err)
		os.Exit(1)
	}

	action := os.Args[1]
	switch action {
	case "up":
		if err := lab.Setup(cfg); err != nil {
			fmt.Println("Error running lab:", err)
			os.Exit(1)
		}
	case "down":
		if err := lab.Remove(cfg); err != nil {
			fmt.Println("Error running lab remove:", err)
			os.Exit(1)
		}
	default:
		fmt.Println("Usage: netlab up|down <config-path>")
		os.Exit(1)
	}
}
