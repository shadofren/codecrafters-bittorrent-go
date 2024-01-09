package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	// bencode "github.com/jackpal/bencode-go" // Available if you need it!
)

func decodeBencode(reader *bytes.Reader) (interface{}, error) {
	c, _ := reader.ReadByte()

	switch {
	case '0' <= c && c <= '9':
		reader.UnreadByte()
		lengthBytes, _ := readSlice(reader, ':')
		length, _ := strconv.Atoi(string(lengthBytes))
		value, err := readSliceSize(reader, length)
		return string(value), err
	case c == 'i':
		value, _ := readSlice(reader, 'e')
		number, err := strconv.Atoi(string(value))
		return number, err
	case c == 'l':
		data := make([]any, 0)
		for {
			cur, _ := reader.ReadByte()
			if cur == 'e' {
				break
			}
			reader.UnreadByte()
			value, _ := decodeBencode(reader)
			data = append(data, value)
		}
		return data, nil
	case c == 'd':
		data := make(map[string]any)
		for {
			cur, _ := reader.ReadByte()
			if cur == 'e' {
				break
			}
			reader.UnreadByte()
			key, _ := decodeBencode(reader)
			value, _ := decodeBencode(reader)
			if keyStr, ok := key.(string); ok {
				data[keyStr] = value
				// Type assertion succeeded, and strValue is now of type string
			} else {
				return "", fmt.Errorf("not a valid key %v", key)
			}
		}
		return data, nil
	default:
		return "", fmt.Errorf("unexpected character %v", c)
	}
}

func readSlice(reader *bytes.Reader, delim byte) ([]byte, error) {
	res := make([]byte, 0)
	for {
		c, _ := reader.ReadByte()
		if c == delim {
			break
		}
		res = append(res, c)
	}
	return res, nil
}

func readSliceSize(reader *bytes.Reader, size int) ([]byte, error) {
	res := make([]byte, 0)
	for i := 0; i < size; i++ {
		c, _ := reader.ReadByte()
		res = append(res, c)
	}
	return res, nil
}

func main() {
	command := os.Args[1]

	if command == "decode" {
		bencodedValue := os.Args[2]

		decoded, err := decodeBencode(bytes.NewReader([]byte(bencodedValue)))
		if err != nil {
			fmt.Println(err)
			return
		}

		jsonOutput, _ := json.Marshal(decoded)
		fmt.Println(string(jsonOutput))
	} else if command == "info" {
		fileContent, err := os.ReadFile(os.Args[2])
		if err != nil {
			fmt.Println(err)
			return
		}
		data, err := decodeBencode(bytes.NewReader(fileContent))
		if err != nil {
			fmt.Println(err)
			return
		}
		if data, ok := data.(map[string]any); ok {
			fmt.Printf("Tracker URL: %s\n", data["announce"])
			if info, ok := data["info"].(map[string]any); ok {
        fmt.Printf("Length: %d\n", info["length"])
        /* for k := range info { */
        /*   fmt.Printf("key %s\n", k) */
        /* } */
			}
		}

	} else {
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}
