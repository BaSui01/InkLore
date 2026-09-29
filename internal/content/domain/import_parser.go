package domain

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"path"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	MaxImportSizeBytes       = 20 << 20  // 20 MiB
	MaxDocxUncompressedBytes = 100 << 20 // 100 MiB
	MaxDocxEntries           = 2000
	MaxDocxRatio             = 100
	MaxChapterCount          = 2000
	MaxTitleRunes            = 120
	MaxBodyRunes             = 200000
)

type ParsedChapter struct {
	Title    string
	Body     string
	Warnings []string
}

type rawChapter struct {
	title    string
	lines    []string
	inferred bool
}

var chapterHeadingRegex = regexp.MustCompile(`^[ \t]*(第[0-9一二三四五六七八九十百千万零0-9]+[章回节集卷篇部][ \t]*[^\r\n]*)$`)

func ParseImport(format string, data []byte) ([]ParsedChapter, error) {
	if len(data) > MaxImportSizeBytes {
		return nil, Invalid()
	}
	switch format {
	case JobFormatTXT:
		return parseTXT(data)
	case JobFormatDOCX:
		return parseDOCX(data)
	default:
		return nil, Invalid()
	}
}

func parseTXT(data []byte) ([]ParsedChapter, error) {
	if len(data) >= 2 {
		if (data[0] == 0xff && data[1] == 0xfe) || (data[0] == 0xfe && data[1] == 0xff) {
			return nil, UnsupportedEncoding()
		}
	}
	if bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}) {
		data = data[3:]
	}
	if !utf8.Valid(data) || bytes.IndexByte(data, 0) != -1 {
		return nil, UnsupportedEncoding()
	}
	text := string(data)
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	lines := strings.Split(text, "\n")
	var rawList []rawChapter
	var current *rawChapter

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if chapterHeadingRegex.MatchString(trimmed) {
			if current != nil {
				rawList = append(rawList, *current)
			}
			current = &rawChapter{title: trimmed, lines: []string{}}
		} else if current != nil {
			current.lines = append(current.lines, line)
		} else {
			if trimmed != "" {
				current = &rawChapter{title: "引子", lines: []string{line}, inferred: true}
			}
		}
	}
	if current != nil {
		rawList = append(rawList, *current)
	}

	if len(rawList) == 0 {
		rawList = append(rawList, rawChapter{title: "第一章", lines: lines, inferred: true})
	}

	return finalizeChapters(rawList)
}

func finalizeChapters(rawList []rawChapter) ([]ParsedChapter, error) {
	if len(rawList) > MaxChapterCount {
		return nil, Invalid()
	}
	out := make([]ParsedChapter, 0, len(rawList))
	for _, rc := range rawList {
		title := strings.TrimSpace(rc.title)
		titleRunes := utf8.RuneCountInString(title)
		if titleRunes < 1 || titleRunes > MaxTitleRunes {
			return nil, Invalid()
		}
		body := strings.Trim(strings.Join(rc.lines, "\n"), "\n")
		bodyRunes := utf8.RuneCountInString(body)
		if bodyRunes > MaxBodyRunes {
			return nil, Invalid()
		}
		var warnings []string
		if rc.inferred {
			warnings = append(warnings, "heading_inferred")
		}
		if Words(body) == 0 {
			warnings = append(warnings, "empty_chapter")
		}
		out = append(out, ParsedChapter{Title: title, Body: body, Warnings: warnings})
	}
	return out, nil
}

func parseDOCX(data []byte) ([]ParsedChapter, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil || len(r.File) > MaxDocxEntries {
		return nil, UnsupportedDocument()
	}

	var totalUncompressed uint64
	var docFile *zip.File

	for _, f := range r.File {
		name := f.Name
		clean := path.Clean(name)
		if strings.Contains(name, "\\") || strings.Contains(name, ":") || strings.HasPrefix(name, "/") ||
			clean == ".." || strings.HasPrefix(clean, "../") {
			return nil, UnsupportedDocument()
		}
		lower := strings.ToLower(name)
		if strings.Contains(lower, "vbaproject") || strings.HasSuffix(lower, ".bin") || strings.HasSuffix(lower, ".vba") {
			return nil, UnsupportedDocument()
		}
		if strings.HasPrefix(lower, "word/media/") || strings.Contains(lower, "oleobject") || strings.Contains(lower, "embeddings") {
			return nil, UnsupportedDocument()
		}
		if f.UncompressedSize64 > 0 && f.CompressedSize64 > 0 {
			ratio := f.UncompressedSize64 / f.CompressedSize64
			if ratio > MaxDocxRatio {
				return nil, UnsupportedDocument()
			}
		}
		totalUncompressed += f.UncompressedSize64
		if totalUncompressed > uint64(MaxDocxUncompressedBytes) {
			return nil, UnsupportedDocument()
		}
		if name == "word/document.xml" {
			docFile = f
		}
	}
	if docFile == nil {
		return nil, UnsupportedDocument()
	}

	rc, err := docFile.Open()
	if err != nil {
		return nil, UnsupportedDocument()
	}
	defer func() { _ = rc.Close() }()

	paragraphs, err := extractDocxParagraphs(rc)
	if err != nil {
		return nil, err
	}

	var rawList []rawChapter
	var current *rawChapter

	for _, p := range paragraphs {
		trimmed := strings.TrimSpace(p.text)
		if p.isHeading || chapterHeadingRegex.MatchString(trimmed) {
			if current != nil {
				rawList = append(rawList, *current)
			}
			current = &rawChapter{title: trimmed, lines: []string{}}
		} else if current != nil {
			current.lines = append(current.lines, p.text)
		} else {
			if trimmed != "" {
				current = &rawChapter{title: "引子", lines: []string{p.text}, inferred: true}
			}
		}
	}
	if current != nil {
		rawList = append(rawList, *current)
	}
	if len(rawList) == 0 {
		allLines := make([]string, len(paragraphs))
		for i, p := range paragraphs {
			allLines[i] = p.text
		}
		rawList = append(rawList, rawChapter{title: "第一章", lines: allLines, inferred: true})
	}

	return finalizeChapters(rawList)
}

type docxPara struct {
	text      string
	isHeading bool
}

func extractDocxParagraphs(r io.Reader) ([]docxPara, error) {
	dec := xml.NewDecoder(r)
	var paras []docxPara

	var inT bool
	var inTc bool
	var isHeading bool
	var currentText strings.Builder
	var rowCells []string
	var cellText strings.Builder

	for {
		token, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, UnsupportedDocument()
		}
		switch se := token.(type) {
		case xml.StartElement:
			switch se.Name.Local {
			case "p":
				isHeading = false
				currentText.Reset()
			case "pStyle":
				for _, a := range se.Attr {
					if a.Name.Local == "val" {
						v := strings.ToLower(a.Value)
						if v == "heading1" || v == "heading 1" || v == "1" || strings.HasPrefix(v, "heading1") {
							isHeading = true
						}
					}
				}
			case "t":
				inT = true
			case "tr":
				rowCells = nil
			case "tc":
				inTc = true
				cellText.Reset()
			}
		case xml.CharData:
			if inT {
				currentText.Write(se)
				if inTc {
					cellText.Write(se)
				}
			}
		case xml.EndElement:
			switch se.Name.Local {
			case "t":
				inT = false
			case "p":
				if !inTc {
					paras = append(paras, docxPara{text: currentText.String(), isHeading: isHeading})
				}
			case "tc":
				inTc = false
				rowCells = append(rowCells, cellText.String())
			case "tr":
				if len(rowCells) > 0 {
					paras = append(paras, docxPara{text: strings.Join(rowCells, "\t"), isHeading: false})
				}
			}
		}
	}
	return paras, nil
}
