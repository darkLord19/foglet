package util

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Confirm asks the user for yes/no confirmation
func Confirm(message string, defaultYes bool) bool {
	prompt := "[y/N]"
	if defaultYes {
		prompt = "[Y/n]"
	}

	fmt.Printf("%s %s: ", message, prompt)

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return defaultYes
	}

	response = strings.ToLower(strings.TrimSpace(response))

	if response == "" {
		return defaultYes
	}

	return response == "y" || response == "yes"
}

// AskString prompts the user for a string input
func AskString(prompt string, defaultValue string) string {
	if defaultValue != "" {
		fmt.Printf("%s [%s]: ", prompt, defaultValue)
	} else {
		fmt.Printf("%s: ", prompt)
	}

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return defaultValue
	}

	response = strings.TrimSpace(response)
	if response == "" {
		return defaultValue
	}

	return response
}

// SelectOption presents a menu and returns the selected option
func SelectOption(prompt string, options []string) (int, error) {
	fmt.Println(prompt)
	for i, opt := range options {
		fmt.Printf("[%d] %s\n", i+1, opt)
	}
	fmt.Print("\nSelect: ")

	var choice int
	_, err := fmt.Scanf("%d", &choice)
	if err != nil {
		return -1, err
	}

	if choice < 1 || choice > len(options) {
		return -1, fmt.Errorf("invalid choice")
	}

	return choice - 1, nil
}
