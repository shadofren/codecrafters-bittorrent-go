package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
)

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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
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

func readMessage(conn net.Conn) (*PeerMessage, error) {
	header := make([]byte, 5)
	_, err := conn.Read(header)
	must(err)
	length := binary.BigEndian.Uint32(header[:4])
	payload := make([]byte, int(length-1))
	io.ReadFull(conn, payload)
	return &PeerMessage{
		Type:    header[4],
		Payload: payload,
	}, nil
}

func writeMessage(conn net.Conn, message *PeerMessage) error {
	if message == nil {
		return fmt.Errorf("message cannot be %v", message)
	}
	if len(message.Payload) > int(^uint32(0)) {
		return fmt.Errorf("payload length exceeds maximum allowed by uint32")
	}
	buf := make([]byte, 5+len(message.Payload))
	binary.BigEndian.PutUint32(buf[:4], uint32(len(message.Payload)+1))
	buf[4] = message.Type
	copy(buf[5:], message.Payload)
	_, err := conn.Write(buf)
	return err
}
