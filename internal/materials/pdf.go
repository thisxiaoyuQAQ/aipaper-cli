package materials

import (
	"bytes"
	"compress/zlib"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"
	"strings"
)

func extractPDFText(data []byte) (string, error) {
	var parts []string
	streams := pdfStreams(data)
	for _, stream := range streams {
		text := extractPDFTextTokens(stream)
		if strings.TrimSpace(text) != "" {
			parts = append(parts, text)
		}
	}
	if len(parts) == 0 {
		text := extractPDFTextTokens(data)
		if strings.TrimSpace(text) != "" {
			parts = append(parts, text)
		}
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("pdf contains no extractable text")
	}
	return strings.Join(parts, "\n\n") + "\n", nil
}

func pdfStreams(data []byte) [][]byte {
	var streams [][]byte
	offset := 0
	for {
		idx := bytes.Index(data[offset:], []byte("stream"))
		if idx < 0 {
			break
		}
		streamStart := offset + idx + len("stream")
		if streamStart < len(data) && data[streamStart] == '\r' {
			streamStart++
		}
		if streamStart < len(data) && data[streamStart] == '\n' {
			streamStart++
		}
		endRel := bytes.Index(data[streamStart:], []byte("endstream"))
		if endRel < 0 {
			break
		}
		streamEnd := streamStart + endRel
		streamData := bytes.TrimRight(data[streamStart:streamEnd], "\r\n")
		dictStart := lastIndex(data[:offset+idx], []byte("<<"))
		dict := []byte{}
		if dictStart >= 0 {
			dict = data[dictStart : offset+idx]
		}
		if bytes.Contains(dict, []byte("/FlateDecode")) {
			if decoded, err := inflate(streamData); err == nil {
				streamData = decoded
			}
		}
		streams = append(streams, streamData)
		offset = streamEnd + len("endstream")
	}
	return streams
}

func inflate(data []byte) ([]byte, error) {
	reader, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

func extractPDFTextTokens(data []byte) string {
	var tokens []string
	for i := 0; i < len(data); i++ {
		switch data[i] {
		case '(':
			value, next := parsePDFLiteralString(data, i+1)
			if strings.TrimSpace(value) != "" {
				tokens = append(tokens, value)
			}
			i = next
		case '<':
			if i+1 < len(data) && data[i+1] == '<' {
				continue
			}
			value, next := parsePDFHexString(data, i+1)
			if strings.TrimSpace(value) != "" {
				tokens = append(tokens, value)
			}
			i = next
		}
	}
	return strings.Join(tokens, " ")
}

func parsePDFLiteralString(data []byte, i int) (string, int) {
	var b strings.Builder
	depth := 1
	escaped := false
	for i < len(data) {
		ch := data[i]
		if escaped {
			switch ch {
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			case 'b':
				b.WriteByte('\b')
			case 'f':
				b.WriteByte('\f')
			case '\\', '(', ')':
				b.WriteByte(ch)
			default:
				if ch >= '0' && ch <= '7' {
					octal := []byte{ch}
					j := i + 1
					for ; j < len(data) && len(octal) < 3 && data[j] >= '0' && data[j] <= '7'; j++ {
						octal = append(octal, data[j])
					}
					if parsed, err := strconv.ParseInt(string(octal), 8, 32); err == nil {
						b.WriteRune(rune(parsed))
					}
					i = j - 1
				} else {
					b.WriteByte(ch)
				}
			}
			escaped = false
			i++
			continue
		}
		switch ch {
		case '\\':
			escaped = true
		case '(':
			depth++
			b.WriteByte(ch)
		case ')':
			depth--
			if depth == 0 {
				return strings.Join(strings.Fields(b.String()), " "), i
			}
			b.WriteByte(ch)
		default:
			b.WriteByte(ch)
		}
		i++
	}
	return strings.Join(strings.Fields(b.String()), " "), i
}

func parsePDFHexString(data []byte, i int) (string, int) {
	start := i
	for i < len(data) && data[i] != '>' {
		i++
	}
	raw := string(data[start:i])
	raw = strings.Join(strings.Fields(raw), "")
	if len(raw)%2 == 1 {
		raw += "0"
	}
	decoded, err := hex.DecodeString(raw)
	if err != nil {
		return "", i
	}
	return strings.Join(strings.Fields(string(decoded)), " "), i
}

func lastIndex(data, sep []byte) int {
	for i := len(data) - len(sep); i >= 0; i-- {
		if bytes.Equal(data[i:i+len(sep)], sep) {
			return i
		}
	}
	return -1
}
