package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
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

const (
	Choke = iota
	Unchoke
	Interested
	NotInterested
	Have
	Bitfield
	Request
	Piece
	Cancel
)

type FileInfo struct {
	TrackerURL  string
	Length      int
	InfoHash    string
	PieceLength int
	PieceHashes []string
	Peers       []string
}

type TrackerResponse struct {
	Interval int
	Peers    string
}

type Handshake struct {
	Length     uint8
	BitTorrent [19]byte
	Reserved   [8]byte
	InfoHash   [20]byte
	PeerId     [20]byte
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
	if decoded, ok := decoded.(*OrderedMap); ok {
		tracker := NewTrackerResponse(decoded)
		for i := 0; i < len(tracker.Peers); i += 6 {
			ip, port, _ := parseBytesToIPv4AndPort([]byte(tracker.Peers[i : i+6]))
			peer := fmt.Sprintf("%s:%d\n", ip, port)
			f.Peers = append(f.Peers, peer)
		}
	}
}

func NewTrackerResponse(data *OrderedMap) *TrackerResponse {
	interval, _ := data.Get("interval")
	peers, _ := data.Get("peers")
	response := TrackerResponse{}
	if peers, ok := peers.(string); ok {
		response.Peers = peers
	}
	if interval, ok := interval.(int); ok {
		response.Interval = interval
	}
	return &response
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
		fileInfo := readTorrentFile(os.Args[2])
		fileInfo.Info()
	case "peers":
		fileInfo := readTorrentFile(os.Args[2])
		fileInfo.GetPeers()
		for _, peer := range fileInfo.Peers {
			fmt.Println(peer)
		}
	case "handshake":
		fileInfo := readTorrentFile(os.Args[2])
		resp := sendHandshake(os.Args[3], fileInfo)
		fmt.Printf("Peer ID: %x\n", resp.PeerId)
	case "download_piece":
		fileInfo := readTorrentFile(os.Args[2])
		fileInfo.GetPeers()
		resp := sendHandshake(fileInfo.Peers[0], fileInfo)
    fmt.Printf("sending download_piece to peer %s, response %v\n", fileInfo.Peers[0], resp)
	default:
		fmt.Println("Unknown command: " + os.Args[1])
		os.Exit(1)
	}
}

func readTorrentFile(filename string) *FileInfo {
	fileContent, err := os.ReadFile(filename)
	must(err)
	data, err := decodeBencode(bytes.NewReader(fileContent))
	must(err)
	if data, ok := data.(*OrderedMap); ok {
		fileInfo, err := NewFileInfo(data)
		must(err)
		return fileInfo
	}
	return nil

}

func sendHandshake(peer string, torrent *FileInfo) *Handshake {
	bitTorrent := [19]byte{}
	copy(bitTorrent[:], []byte("BitTorrent protocol"))
	info, peerId := [20]byte{}, [20]byte{}
	copy(info[:], []byte(torrent.InfoHash))
	copy(peerId[:], []byte("00112233445566778899"))

	handshake := Handshake{
		Length:     19,
		BitTorrent: bitTorrent,
		Reserved:   [8]byte{},
		InfoHash:   info,
		PeerId:     peerId,
	}
	conn, err := net.Dial("tcp", peer)
	must(err)
	defer conn.Close()
	_, err = conn.Write(packHandShake(&handshake))
	must(err)
	buffer := make([]byte, 1024)
	n, err := conn.Read(buffer)
	must(err)
	response := buffer[:n]
	resp := unpackHandShake(response)
	return resp
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

func packHandShake(hs *Handshake) []byte {
	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, hs.Length)
	binary.Write(&buf, binary.BigEndian, hs.BitTorrent)
	binary.Write(&buf, binary.BigEndian, hs.Reserved)
	binary.Write(&buf, binary.BigEndian, hs.InfoHash)
	binary.Write(&buf, binary.BigEndian, hs.PeerId)
	return buf.Bytes()
}

func unpackHandShake(data []byte) *Handshake {
	reader := bytes.NewReader(data)
	var hs Handshake

	binary.Read(reader, binary.BigEndian, &hs.Length)
	binary.Read(reader, binary.BigEndian, &hs.BitTorrent)
	binary.Read(reader, binary.BigEndian, &hs.Reserved)
	binary.Read(reader, binary.BigEndian, &hs.InfoHash)
	binary.Read(reader, binary.BigEndian, &hs.PeerId)
	return &hs
}
