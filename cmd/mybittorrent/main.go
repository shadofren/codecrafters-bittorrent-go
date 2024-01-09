package main

import (
	// Uncomment this line to pass the first stage
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode"
	// bencode "github.com/jackpal/bencode-go" // Available if you need it!
)

// Example:
// - 5:hello -> hello
// - 10:hello12345 -> hello12345
// - i52e -> 52
func decodeBencode(bencodedString string) (interface{}, error) {
	if unicode.IsDigit(rune(bencodedString[0])) {

		lengthStr, valueStr, found := strings.Cut(bencodedString, ":")
		if !found {
			return "", fmt.Errorf("not valid bencoded string")
		}

		length, err := strconv.Atoi(lengthStr)
		if err != nil {
			return "", err
		}

		return valueStr[:length], nil
	} else if bencodedString[0] == 'i' {
    // integer parsing
    // find the end
    i := strings.Index(bencodedString, "e")
    value, err := strconv.Atoi(bencodedString[1:i])
    if err != nil {
      return 0, err
    }
    return value, nil
	} else {
    return "", fmt.Errorf("not supported yet")
  }
}

func main() {
	command := os.Args[1]

	if command == "decode" {
		bencodedValue := os.Args[2]

		decoded, err := decodeBencode(bencodedValue)
		if err != nil {
			fmt.Println(err)
			return
		}

		jsonOutput, _ := json.Marshal(decoded)
		fmt.Println(string(jsonOutput))
	} else {
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}
