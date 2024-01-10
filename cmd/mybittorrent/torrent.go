package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
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

type PeerMessage struct {
	Type    uint8
	Payload []byte
}

type RequestMessage struct {
	PieceIndex uint32
	Begin      uint32
	Length     uint32
}

func NewRequestMessage(pieceIndex, begin, length uint32) *PeerMessage {
	payload := make([]byte, 12)
	binary.BigEndian.PutUint32(payload, pieceIndex)
	binary.BigEndian.PutUint32(payload[4:], begin)
	binary.BigEndian.PutUint32(payload[8:], length)

	return &PeerMessage{
		Type:    Request,
		Payload: payload,
	}
}

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
	if decoded, ok := decoded.(*OrderedMap); ok {
		tracker := NewTrackerResponse(decoded)
		for i := 0; i < len(tracker.Peers); i += 6 {
			ip, port, _ := parseBytesToIPv4AndPort([]byte(tracker.Peers[i : i+6]))
			peer := fmt.Sprintf("%s:%d", ip, port)
			f.Peers = append(f.Peers, peer)
		}
	}
}

func ReadTorrentFile(filename string) *FileInfo {
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

type Handshake struct {
	Length     uint8
	BitTorrent [19]byte
	Reserved   [8]byte
	InfoHash   [20]byte
	PeerId     [20]byte
}

func PackHandShake(hs *Handshake) []byte {
	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, hs.Length)
	binary.Write(&buf, binary.BigEndian, hs.BitTorrent)
	binary.Write(&buf, binary.BigEndian, hs.Reserved)
	binary.Write(&buf, binary.BigEndian, hs.InfoHash)
	binary.Write(&buf, binary.BigEndian, hs.PeerId)
	return buf.Bytes()
}

func UnpackHandShake(data []byte) *Handshake {
	reader := bytes.NewReader(data)
	var hs Handshake

	binary.Read(reader, binary.BigEndian, &hs.Length)
	binary.Read(reader, binary.BigEndian, &hs.BitTorrent)
	binary.Read(reader, binary.BigEndian, &hs.Reserved)
	binary.Read(reader, binary.BigEndian, &hs.InfoHash)
	binary.Read(reader, binary.BigEndian, &hs.PeerId)
	return &hs
}

func SendHandShake(conn net.Conn, torrent *FileInfo) *Handshake {
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
	_, err := conn.Write(PackHandShake(&handshake))
	must(err)
	buffer := make([]byte, 1024)
	n, err := conn.Read(buffer)
	must(err)
	response := buffer[:n]
	resp := UnpackHandShake(response)
	return resp
}

type TrackerResponse struct {
	Interval int
	Peers    string
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

func DownloadPiece(conn net.Conn, output string, fileInfo *FileInfo, pieceId int) {
	if pieceId >= len(fileInfo.PieceHashes) {
		return
	}

	// bitfield message
	_, err := readMessage(conn)
	must(err)
	// send interest message
	err = writeMessage(conn, &PeerMessage{Type: Interested, Payload: []byte{}})
	must(err)
	/* Wait until you receive an unchoke message back */
	_, err = readMessage(conn)
	must(err)
	file, err := os.OpenFile(output, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	must(err)
	defer file.Close()

	numPiece := len(fileInfo.PieceHashes)
	pieceLength := fileInfo.PieceLength
	if pieceId+1 == numPiece {
		pieceLength = fileInfo.Length - (numPiece-1)*fileInfo.PieceLength // last piece length
	}

	for begin := 0; begin < pieceLength; begin += blockSize {
		length := uint32(min(pieceLength-begin, blockSize))
		request := NewRequestMessage(uint32(pieceId), uint32(begin), length)
		err = writeMessage(conn, request)
		must(err)
		pieceMessage, err := readMessage(conn)
		must(err)
		data := pieceMessage.Payload[8:] // skipping the index, begin uint32
		_, err = file.Write(data)
		must(err)
	}
}

