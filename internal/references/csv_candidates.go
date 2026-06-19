package references

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
)

func CandidatesFromCSV(r io.Reader, startIndex int) ([]contracts.ReferenceCandidate, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) < 2 {
		return nil, nil
	}
	columns := mapCSVHeaders(rows[0])
	titleIdx, ok := firstCSVColumn(columns, "title")
	if !ok {
		return nil, nil
	}
	candidates := make([]contracts.ReferenceCandidate, 0, len(rows)-1)
	for _, row := range rows[1:] {
		title := csvCell(row, titleIdx)
		if strings.TrimSpace(title) == "" {
			continue
		}
		urlValue := csvValue(row, columns, "url")
		doiValue := csvValue(row, columns, "doi")
		candidate := contracts.ReferenceCandidate{
			ID:           fmt.Sprintf("cand_%03d", startIndex+len(candidates)),
			Title:        title,
			Authors:      splitCSVAuthors(csvValue(row, columns, "authors")),
			Year:         parseCSVYear(csvValue(row, columns, "year")),
			Source:       "csv_export",
			DOI:          doiValue,
			URL:          urlValue,
			Abstract:     csvValue(row, columns, "abstract"),
			Venue:        csvValue(row, columns, "venue"),
			Status:       "pending",
			Reliability:  "user_export",
			Availability: availabilityFromURLAndDOI(urlValue, doiValue),
			AccessURL:    urlValue,
		}
		candidate.DedupeGroup = CandidateDedupeGroup(candidate)
		candidates = append(candidates, candidate)
	}
	return candidates, nil
}

func mapCSVHeaders(headers []string) map[string]int {
	out := map[string]int{}
	for i, header := range headers {
		key := canonicalCSVHeader(header)
		if key != "" {
			out[key] = i
		}
	}
	return out
}

func canonicalCSVHeader(header string) string {
	key := strings.ToLower(strings.TrimSpace(header))
	key = strings.ReplaceAll(key, " ", "")
	key = strings.ReplaceAll(key, "_", "")
	aliases := map[string]string{
		"title": "title", "题名": "title", "标题": "title", "篇名": "title", "文献标题": "title",
		"author": "authors", "authors": "authors", "作者": "authors", "责任者": "authors",
		"year": "year", "年份": "year", "年": "year", "发表时间": "year", "发表年份": "year", "出版年": "year",
		"source": "venue", "journal": "venue", "venue": "venue", "来源": "venue", "期刊": "venue", "刊名": "venue", "出处": "venue",
		"doi": "doi", "DOI": "doi",
		"url": "url", "link": "url", "链接": "url", "网址": "url", "URL": "url",
		"abstract": "abstract", "摘要": "abstract", "简介": "abstract",
	}
	return aliases[key]
}

func firstCSVColumn(columns map[string]int, keys ...string) (int, bool) {
	for _, key := range keys {
		idx, ok := columns[key]
		if ok {
			return idx, true
		}
	}
	return 0, false
}

func csvValue(row []string, columns map[string]int, key string) string {
	idx, ok := columns[key]
	if !ok {
		return ""
	}
	return csvCell(row, idx)
}

func csvCell(row []string, idx int) string {
	if idx < 0 || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}

func splitCSVAuthors(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ';' || r == '；' || r == ',' || r == '，'
	})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			out = append(out, strings.TrimSpace(part))
		}
	}
	return out
}

func parseCSVYear(value string) int {
	value = strings.TrimSpace(value)
	if len(value) >= 4 {
		value = value[:4]
	}
	year, _ := strconv.Atoi(value)
	return year
}
