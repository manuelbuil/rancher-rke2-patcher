package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func promptYesNo(prompt string) (bool, error) {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print(prompt)
		input, err := reader.ReadString('\n')
		if err != nil {
			return false, fmt.Errorf("failed to read user input: %w", err)
		}

		normalized := strings.ToLower(strings.TrimSpace(input))
		switch normalized {
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		default:
			fmt.Println("please answer Yes or No")
		}
	}
}
