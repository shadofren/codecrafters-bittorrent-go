package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	// bencode "github.com/jackpal/bencode-go" // Available if you need it!
)

type FileInfo struct {
	TrackerURL  string
	Length      int
	InfoHash    string
	PieceLength int
	PieceHashes []string
}

func (f *FileInfo) Print() {
	fmt.Printf("Tracker URL: %s\n", f.TrackerURL)
	fmt.Printf("Length: %d\n", f.Length)
	fmt.Printf("Info Hash: %x\n", f.InfoHash)
	fmt.Printf("Piece Length: %d\n", f.PieceLength)
	fmt.Println("Piece Hashes:")
	for _, piece := range f.PieceHashes {
		fmt.Printf("%x\n", piece)
	}
}

func NewFileInfo(data *OrderedMap) (*FileInfo, error) {
	fileInfo := FileInfo{}
	announce, _ := data.Get("announce")
	info, _ := data.Get("info")
	fileInfo.TrackerURL = announce.(string)
	if info, ok := info.(*OrderedMap); ok {
		length, _ := info.Get("length")
		fileInfo.Length = length.(int)
	}

	infoBytes, err := bencode(info)
	if err != nil {
		return nil, err
	}
	h := sha1.New()
	h.Write(infoBytes)
	fileInfo.InfoHash = string(h.Sum(nil))
	if info, ok := info.(*OrderedMap); ok {
		length, _ := info.Get("piece length")
		fileInfo.PieceLength = length.(int)
		pieceHashes, _ := info.Get("pieces")
		pieceHashesStr, _ := pieceHashes.(string)
		if len(pieceHashesStr)%20 != 0 {
			return nil, fmt.Errorf("hash length is not multiple of 20")
		}
		fileInfo.PieceHashes = make([]string, 0)
		for i := 0; i < len(pieceHashesStr); i += 20 {
			fileInfo.PieceHashes = append(fileInfo.PieceHashes, pieceHashesStr[i:i+20])
		}
	}
	return &fileInfo, nil
}

func main() {
	switch os.Args[1] {
	case "decode":
		bencodedValue := os.Args[2]
		decoded, err := decodeBencode(bytes.NewReader([]byte(bencodedValue)))
		if err != nil {
			fmt.Println(err)
			return
		}
		var jsonOutput []byte
		if mapData, ok := decoded.(*OrderedMap); ok {
			jsonOutput, _ = json.Marshal(mapData.GetMap())
		} else {
			jsonOutput, _ = json.Marshal(decoded)
		}
		fmt.Println(string(jsonOutput))
	case "info":
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
		if data, ok := data.(*OrderedMap); ok {
			fileInfo, err := NewFileInfo(data)
			if err != nil {
				fmt.Println(err)
				return
			}
			fileInfo.Print()
		}
	default:
		fmt.Println("Unknown command: " + os.Args[1])
		os.Exit(1)
	}
}

func bencode(data any) ([]byte, error) {
	encoded := make([]byte, 0)
	var err error
	switch d := data.(type) {
	case []any:
		encoded = append(encoded, 'l')
		for _, item := range d {
			value, _ := bencode(item)
			encoded = append(encoded, value...)
		}
		encoded = append(encoded, 'e')
	case *OrderedMap:
		encoded = append(encoded, 'd')
		for _, k := range d.Keys() {
			key, _ := bencode(k)
			encoded = append(encoded, key...)
			v, _ := d.Get(k)
			value, _ := bencode(v)
			encoded = append(encoded, value...)
		}
		encoded = append(encoded, 'e')
	case int:
		encoded = append(encoded, 'i')
		intStr := strconv.Itoa(d)
		encoded = append(encoded, []byte(intStr)...)
		encoded = append(encoded, 'e')
	case string:
		length := len(d)
		lengthStr := strconv.Itoa(length)
		encoded = append(encoded, []byte(lengthStr)...)
		encoded = append(encoded, ':')
		encoded = append(encoded, []byte(d)...)
	default:
		err = fmt.Errorf("not supported type")
	}
	return encoded, err
}

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
		data := NewOrderedMap()
		for {
			cur, _ := reader.ReadByte()
			if cur == 'e' {
				break
			}
			reader.UnreadByte()
			key, _ := decodeBencode(reader)
			value, _ := decodeBencode(reader)
			if keyStr, ok := key.(string); ok {
				data.Set(keyStr, value)
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
