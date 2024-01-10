package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	// bencode "github.com/jackpal/bencode-go" // Available if you need it!
)

const blockSize int = 1 << 14 // 16 * 1024 bytes

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
		fileInfo := ReadTorrentFile(os.Args[2])
		fileInfo.Info()
	case "peers":
		fileInfo := ReadTorrentFile(os.Args[2])
		fileInfo.GetPeers()
		for _, peer := range fileInfo.Peers {
			fmt.Println(peer)
		}
	case "handshake":
		fileInfo := ReadTorrentFile(os.Args[2])
		conn, err := net.Dial("tcp", os.Args[3])
		must(err)
		defer conn.Close()
		resp := SendHandShake(conn, fileInfo)
		fmt.Printf("Peer ID: %x\n", resp.PeerId)
	case "download_piece":
		output, torrent, pieceIdStr := os.Args[3], os.Args[4], os.Args[5]
		fileInfo := ReadTorrentFile(torrent)
		fileInfo.GetPeers()
		pieceId, err := strconv.Atoi(pieceIdStr)
		must(err)
		DownloadPiece(output, fileInfo, pieceId)
    fmt.Printf("Piece %d downloaded to %s.\n", pieceId, output)
	case "download":
		output, torrent := os.Args[3], os.Args[4]
		fileInfo := ReadTorrentFile(torrent)
		fileInfo.GetPeers()
    _ = output
    Download(output, fileInfo)
    fmt.Printf("Downloaded %s to %s.\n", torrent, output)
	default:
		fmt.Println("Unknown command: " + os.Args[1])
		os.Exit(1)
	}
}
