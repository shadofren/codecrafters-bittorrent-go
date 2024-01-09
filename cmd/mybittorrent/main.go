package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
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
	Peers       []string
}

func (f *FileInfo) Info() {
	fmt.Printf("Tracker URL: %s\n", f.TrackerURL)
	fmt.Printf("Length: %d\n", f.Length)
	fmt.Printf("Info Hash: %x\n", f.InfoHash)
	fmt.Printf("Piece Length: %d\n", f.PieceLength)
	fmt.Println("Piece Hashes:")
	for _, piece := range f.PieceHashes {
		fmt.Printf("%x\n", piece)
	}
}

func (f *FileInfo) GetPeers() {
	// send GET request and print the peers
	params := url.Values{}
	params.Add("info_hash", f.InfoHash)
	params.Add("peer_id", "00112233445566778899")
	params.Add("port", "6881")
	params.Add("uploaded", "0")
	params.Add("downloaded", "0")
	params.Add("left", strconv.Itoa(f.Length))
	params.Add("compact", "1")

	fullURL := fmt.Sprintf("%s?%s", f.TrackerURL, params.Encode())
	resp, err := http.Get(fullURL)
	must(err)
	defer resp.Body.Close()
	body := make([]byte, 1024)
	size, _ := resp.Body.Read(body)
	decoded, err := decodeBencode(bytes.NewReader(body[:size]))
  must(err)
	if m, ok := decoded.(*OrderedMap); ok {
		peers, _ := m.Get("peers")
		if peers, ok := peers.(string); ok {
			for i := 0; i < len(peers); i += 6 {
				ip, port, _ := parseBytesToIPv4AndPort([]byte(peers[i : i+6]))
				fmt.Printf("%+v:%d\n", ip, port)
			}
		}
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
		must(err)
		var jsonOutput []byte
		if mapData, ok := decoded.(*OrderedMap); ok {
			jsonOutput, _ = json.Marshal(mapData.GetMap())
		} else {
			jsonOutput, _ = json.Marshal(decoded)
		}
		fmt.Println(string(jsonOutput))
	case "info":
		fileContent, err := os.ReadFile(os.Args[2])
		must(err)
		data, err := decodeBencode(bytes.NewReader(fileContent))
		must(err)
		if data, ok := data.(*OrderedMap); ok {
			fileInfo, err := NewFileInfo(data)
			must(err)
			fileInfo.Info()
		}
	case "peers":
		fileContent, err := os.ReadFile(os.Args[2])
		must(err)
		data, err := decodeBencode(bytes.NewReader(fileContent))
		must(err)
		if data, ok := data.(*OrderedMap); ok {
			fileInfo, err := NewFileInfo(data)
			must(err)
			fileInfo.GetPeers()
		}
	default:
		fmt.Println("Unknown command: " + os.Args[1])
		data := []byte{100, 56, 58, 99, 111, 109, 112, 108, 101, 116, 101, 105, 50, 101, 49, 48, 58, 100, 111, 119, 110, 108, 111, 97, 100, 101, 100, 105, 49, 101, 49, 48, 58, 105, 110, 99, 111, 109, 112, 108, 101, 116, 101, 105, 49, 101, 56, 58, 105, 110, 116, 101, 114, 118, 97, 108, 105, 49, 57, 50, 49, 101, 49, 50, 58, 109, 105, 110, 32, 105, 110, 116, 101, 114, 118, 97, 108, 105, 57, 54, 48, 101, 53, 58, 112, 101, 101, 114, 115, 49, 56, 58, 188, 119, 61, 177, 26, 225, 185, 107, 13, 235, 213, 14, 88, 99, 2, 101, 26, 225, 101}
		fmt.Println("size", len(data))
		decoded, err := decodeBencode(bytes.NewReader(data))
		must(err)
		if m, ok := decoded.(*OrderedMap); ok {
			peers, _ := m.Get("peers")
			fmt.Printf("type %T\n", peers)
			if peers, ok := peers.(string); ok {
				for i := 0; i < len(peers); i += 6 {
					ip, port, _ := parseBytesToIPv4AndPort([]byte(peers[i : i+6]))
					fmt.Printf("peer %+v\n", peers[i:i+6])
					fmt.Printf("ip %+v, port %d\n", ip, port)
				}
			}
		}
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

func must(err error) {
	if err != nil {
		fmt.Println(err)
		log.Fatalln(err)
	}
}

func parseBytesToIPv4AndPort(data []byte) (string, int, error) {
	if len(data) != 6 {
		return "", 0, fmt.Errorf("input data must be exactly 6 bytes")
	}

	ip := net.IP(data[:4])
	port := int(data[4])<<8 + int(data[5])

	return ip.String(), port, nil
}
