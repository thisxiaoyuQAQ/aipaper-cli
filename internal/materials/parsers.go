package materials

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/references"
)

func formatBibTeXSummary(entries []references.BibTeXEntry) string {
	var b strings.Builder
	b.WriteString("# BibTeX References\n\n")
	for _, entry := range entries {
		title := entry.Fields["title"]
		if title == "" {
			title = entry.Key
		}
		fmt.Fprintf(&b, "- %s", title)
		if year := entry.Fields["year"]; year != "" {
			fmt.Fprintf(&b, " (%s)", year)
		}
		if author := entry.Fields["author"]; author != "" {
			fmt.Fprintf(&b, " - %s", author)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func parseCSVMaterial(data []byte) (string, map[string]any, error) {
	reader := csv.NewReader(bytes.NewReader(data))
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil {
		return "", nil, err
	}
	if len(rows) == 0 {
		return "# CSV Material\n\n(empty csv)\n", map[string]any{"rows": 0, "columns": 0}, nil
	}
	var b strings.Builder
	b.WriteString("# CSV Material\n\n")
	writeMarkdownRow(&b, rows[0])
	separator := make([]string, len(rows[0]))
	for i := range separator {
		separator[i] = "---"
	}
	writeMarkdownRow(&b, separator)
	for _, row := range rows[1:] {
		writeMarkdownRow(&b, row)
	}
	return b.String(), map[string]any{"rows": len(rows), "columns": len(rows[0]), "headers": rows[0]}, nil
}

func writeMarkdownRow(b *strings.Builder, row []string) {
	b.WriteString("|")
	for _, cell := range row {
		cell = strings.ReplaceAll(cell, "|", "\\|")
		cell = strings.Join(strings.Fields(cell), " ")
		b.WriteString(" ")
		b.WriteString(cell)
		b.WriteString(" |")
	}
	b.WriteString("\n")
}

func parseURLMaterial(data []byte) (string, map[string]any, error) {
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parsed, err := url.ParseRequestURI(line)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return "", nil, fmt.Errorf("invalid url material: %s", line)
		}
		text := fmt.Sprintf("# URL Material\n\nURL: %s\n", line)
		return text, map[string]any{"url": line, "scheme": parsed.Scheme, "host": parsed.Host}, nil
	}
	return "", nil, fmt.Errorf("url material is empty")
}

func extractDOCXText(path string) (string, error) {
	reader, err := zip.OpenReader(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	defer reader.Close()

	for _, file := range reader.File {
		if file.Name != "word/document.xml" {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return "", err
		}
		defer rc.Close()
		return docxXMLToText(rc)
	}
	return "", fmt.Errorf("word/document.xml not found")
}

func docxXMLToText(r io.Reader) (string, error) {
	decoder := xml.NewDecoder(r)
	var b strings.Builder
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		switch t := token.(type) {
		case xml.CharData:
			b.WriteString(string(t))
		case xml.EndElement:
			if t.Name.Local == "p" || t.Name.Local == "br" {
				b.WriteString("\n")
			}
		}
	}
	text := strings.TrimSpace(html.UnescapeString(b.String()))
	if text == "" {
		return "", fmt.Errorf("docx contains no text")
	}
	return text + "\n", nil
}
