package domain

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strings"
	"time"
)

const MaxExportSizeBytes = 100 << 20 // 100 MiB

type ExportNovel struct {
	ID         string
	Title      string
	Summary    string
	AuthorName string
}

type ExportVolume struct {
	ID       string
	Title    string
	Position int64
}

type ExportChapter struct {
	ID       string
	VolumeID string
	Title    string
	Body     string
	Position int64
}

type RenderedExport struct {
	Filename  string
	MediaType string
	Payload   []byte
}

func RenderExport(format string, novel ExportNovel, volumes []ExportVolume, chapters []ExportChapter, now time.Time) (RenderedExport, error) {
	var payload []byte
	var mediaType string
	var ext string
	var err error

	safeTitle := sanitizeFilename(novel.Title)
	if safeTitle == "" {
		safeTitle = "novel"
	}

	switch format {
	case JobFormatTXT:
		payload, err = renderTXT(novel, volumes, chapters)
		mediaType = "text/plain; charset=utf-8"
		ext = ".txt"
	case JobFormatMarkdown:
		payload, err = renderMarkdown(novel, volumes, chapters)
		mediaType = "text/markdown; charset=utf-8"
		ext = ".md"
	case JobFormatDOCX:
		payload, err = renderDOCX(novel, volumes, chapters)
		mediaType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
		ext = ".docx"
	case JobFormatEPUB:
		payload, err = renderEPUB(novel, volumes, chapters, now)
		mediaType = "application/epub+zip"
		ext = ".epub"
	default:
		return RenderedExport{}, Invalid()
	}

	if err != nil {
		return RenderedExport{}, err
	}
	if len(payload) > MaxExportSizeBytes {
		return RenderedExport{}, ExportTooLarge()
	}
	return RenderedExport{Filename: safeTitle + ext, MediaType: mediaType, Payload: payload}, nil
}

func sanitizeFilename(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' {
			b.WriteRune('_')
		} else if r > 31 {
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

func renderTXT(novel ExportNovel, volumes []ExportVolume, chapters []ExportChapter) ([]byte, error) {
	var buf bytes.Buffer
	author := novel.AuthorName
	if author == "" {
		author = "佚名"
	}
	buf.WriteString("书名：" + novel.Title + "\n")
	buf.WriteString("作者：" + author + "\n")
	buf.WriteString("语言：zh-CN\n\n")

	volMap := make(map[string]ExportVolume, len(volumes))
	for _, v := range volumes {
		volMap[v.ID] = v
	}

	var lastVolID string
	for _, ch := range chapters {
		if ch.VolumeID != lastVolID {
			lastVolID = ch.VolumeID
			if v, ok := volMap[ch.VolumeID]; ok && strings.TrimSpace(v.Title) != "" {
				buf.WriteString(strings.TrimSpace(v.Title) + "\n\n")
			}
		}
		buf.WriteString(strings.TrimSpace(ch.Title) + "\n\n")
		body := strings.TrimSpace(ch.Body)
		if body != "" {
			buf.WriteString(body + "\n\n")
		}
	}
	return buf.Bytes(), nil
}

func renderMarkdown(novel ExportNovel, volumes []ExportVolume, chapters []ExportChapter) ([]byte, error) {
	var buf bytes.Buffer
	author := novel.AuthorName
	if author == "" {
		author = "佚名"
	}
	buf.WriteString("# " + novel.Title + "\n\n")
	buf.WriteString("- 作者：" + author + "\n")
	buf.WriteString("- 语言：zh-CN\n\n")

	volMap := make(map[string]ExportVolume, len(volumes))
	for _, v := range volumes {
		volMap[v.ID] = v
	}

	var lastVolID string
	for _, ch := range chapters {
		if ch.VolumeID != lastVolID {
			lastVolID = ch.VolumeID
			if v, ok := volMap[ch.VolumeID]; ok && strings.TrimSpace(v.Title) != "" {
				buf.WriteString("# " + strings.TrimSpace(v.Title) + "\n\n")
			}
		}
		buf.WriteString("## " + strings.TrimSpace(ch.Title) + "\n\n")
		body := ch.Body
		lines := strings.Split(body, "\n")
		for _, line := range lines {
			t := strings.TrimLeft(line, " \t")
			if strings.HasPrefix(t, "#") {
				buf.WriteString("\\" + line + "\n")
			} else {
				buf.WriteString(line + "\n")
			}
		}
		buf.WriteString("\n")
	}
	return buf.Bytes(), nil
}

func xmlEscape(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '"':
			b.WriteString("&quot;")
		case '\'':
			b.WriteString("&apos;")
		default:
			if r < 32 && r != '\n' && r != '\r' && r != '\t' {
				continue
			}
			b.WriteRune(r)
		}
	}
	return b.String()
}

func renderDOCX(novel ExportNovel, volumes []ExportVolume, chapters []ExportChapter) (payload []byte, err error) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	closed := false
	defer func() {
		if closed {
			return
		}
		if closeErr := w.Close(); err == nil {
			payload = nil
			err = closeErr
		}
	}()

	author := novel.AuthorName
	if author == "" {
		author = "佚名"
	}

	contentTypes := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>
  <Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/>
  <Override PartName="/docProps/app.xml" ContentType="application/vnd.openxmlformats-officedocument.extended-properties+xml"/>
</Types>`
	if err := writeZipFile(w, "[Content_Types].xml", []byte(contentTypes)); err != nil {
		return nil, err
	}

	rels := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/>
  <Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/extended-properties" Target="docProps/app.xml"/>
</Relationships>`
	if err := writeZipFile(w, "_rels/.rels", []byte(rels)); err != nil {
		return nil, err
	}

	core := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/" xmlns:dcmitype="http://purl.org/dc/dcmitype/" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <dc:title>%s</dc:title>
  <dc:creator>%s</dc:creator>
  <dc:language>zh-CN</dc:language>
</cp:coreProperties>`, xmlEscape(novel.Title), xmlEscape(author))
	if err := writeZipFile(w, "docProps/core.xml", []byte(core)); err != nil {
		return nil, err
	}

	app := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties">
  <Application>InkLore</Application>
</Properties>`
	if err := writeZipFile(w, "docProps/app.xml", []byte(app)); err != nil {
		return nil, err
	}

	docRels := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
</Relationships>`
	if err := writeZipFile(w, "word/_rels/document.xml.rels", []byte(docRels)); err != nil {
		return nil, err
	}

	styles := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:docDefaults>
    <w:rPrDefault>
      <w:rPr>
        <w:rFonts w:ascii="SimSun" w:eastAsia="SimSun" w:hAnsi="SimSun" w:cs="SimSun"/>
        <w:sz w:val="24"/>
        <w:szCs w:val="24"/>
      </w:rPr>
    </w:rPrDefault>
    <w:pPrDefault>
      <w:pPr>
        <w:spacing w:line="480" w:lineRule="exact"/>
      </w:pPr>
    </w:pPrDefault>
  </w:docDefaults>
  <w:style w:type="paragraph" w:styleId="Heading1">
    <w:name w:val="heading 1"/>
    <w:rPr>
      <w:rFonts w:ascii="SimHei" w:eastAsia="SimHei" w:hAnsi="SimHei" w:cs="SimHei"/>
      <w:b/>
      <w:sz w:val="32"/>
      <w:szCs w:val="32"/>
    </w:rPr>
  </w:style>
</w:styles>`
	if err := writeZipFile(w, "word/styles.xml", []byte(styles)); err != nil {
		return nil, err
	}

	var docB strings.Builder
	docB.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>`)

	docB.WriteString(fmt.Sprintf(`<w:p><w:pPr><w:pStyle w:val="Heading1"/></w:pPr><w:r><w:rPr><w:rFonts w:ascii="SimHei" w:eastAsia="SimHei" w:hAnsi="SimHei" w:cs="SimHei"/><w:b/><w:sz w:val="48"/><w:szCs w:val="48"/></w:rPr><w:t>%s</w:t></w:r></w:p>`, xmlEscape(novel.Title)))
	docB.WriteString(fmt.Sprintf(`<w:p><w:r><w:t>作者：%s</w:t></w:r></w:p>`, xmlEscape(author)))
	docB.WriteString(`<w:p><w:r><w:t>语言：zh-CN</w:t></w:r></w:p>`)

	volMap := make(map[string]ExportVolume, len(volumes))
	for _, v := range volumes {
		volMap[v.ID] = v
	}

	var lastVolID string
	for _, ch := range chapters {
		docB.WriteString(`<w:p><w:r><w:br w:type="page"/></w:r></w:p>`)
		if ch.VolumeID != lastVolID {
			lastVolID = ch.VolumeID
			if v, ok := volMap[ch.VolumeID]; ok && strings.TrimSpace(v.Title) != "" {
				docB.WriteString(fmt.Sprintf(`<w:p><w:pPr><w:pStyle w:val="Heading1"/></w:pPr><w:r><w:rPr><w:rFonts w:ascii="SimHei" w:eastAsia="SimHei" w:hAnsi="SimHei" w:cs="SimHei"/><w:b/><w:sz w:val="36"/><w:szCs w:val="36"/></w:rPr><w:t>%s</w:t></w:r></w:p>`, xmlEscape(v.Title)))
			}
		}
		docB.WriteString(fmt.Sprintf(`<w:p><w:pPr><w:pStyle w:val="Heading1"/></w:pPr><w:r><w:rPr><w:rFonts w:ascii="SimHei" w:eastAsia="SimHei" w:hAnsi="SimHei" w:cs="SimHei"/><w:b/><w:sz w:val="32"/><w:szCs w:val="32"/></w:rPr><w:t>%s</w:t></w:r></w:p>`, xmlEscape(ch.Title)))
		for _, para := range strings.Split(ch.Body, "\n") {
			if strings.TrimSpace(para) != "" {
				docB.WriteString(fmt.Sprintf(`<w:p><w:pPr><w:spacing w:line="480" w:lineRule="exact"/></w:pPr><w:r><w:rPr><w:rFonts w:ascii="SimSun" w:eastAsia="SimSun" w:hAnsi="SimSun" w:cs="SimSun"/><w:sz w:val="24"/><w:szCs w:val="24"/></w:rPr><w:t xml:space="preserve">%s</w:t></w:r></w:p>`, xmlEscape(para)))
			}
		}
	}
	docB.WriteString(`  </w:body></w:document>`)
	if err := writeZipFile(w, "word/document.xml", []byte(docB.String())); err != nil {
		return nil, err
	}
	if closeErr := w.Close(); closeErr != nil {
		return nil, closeErr
	}
	closed = true
	return buf.Bytes(), nil
}

func renderEPUB(novel ExportNovel, volumes []ExportVolume, chapters []ExportChapter, now time.Time) ([]byte, error) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	author := novel.AuthorName
	if author == "" {
		author = "佚名"
	}

	// EPUB standard requirement: mimetype must be first uncompressed entry
	mimeHdr := &zip.FileHeader{Name: "mimetype", Method: zip.Store}
	mimeW, err := w.CreateHeader(mimeHdr)
	if err != nil {
		return nil, err
	}
	if _, err := mimeW.Write([]byte("application/epub+zip")); err != nil {
		return nil, err
	}

	containerXML := `<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OEBPS/package.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>`
	if err := writeZipFile(w, "META-INF/container.xml", []byte(containerXML)); err != nil {
		return nil, err
	}

	modified := now.UTC().Format("2006-01-02T15:04:05Z")
	var manifestItems strings.Builder
	var spineItems strings.Builder
	var navItems strings.Builder
	var ncxItems strings.Builder

	manifestItems.WriteString(`    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>` + "\n")
	manifestItems.WriteString(`    <item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/>` + "\n")

	for i, ch := range chapters {
		fileID := fmt.Sprintf("chapter_%d", i+1)
		fileName := fmt.Sprintf("chapter_%d.xhtml", i+1)
		manifestItems.WriteString(fmt.Sprintf(`    <item id="%s" href="%s" media-type="application/xhtml+xml"/>`+"\n", fileID, fileName))
		spineItems.WriteString(fmt.Sprintf(`    <itemref idref="%s"/>`+"\n", fileID))
		navItems.WriteString(fmt.Sprintf(`      <li><a href="%s">%s</a></li>`+"\n", fileName, xmlEscape(ch.Title)))
		ncxItems.WriteString(fmt.Sprintf(`    <navPoint id="navPoint-%d" playOrder="%d"><navLabel><text>%s</text></navLabel><content src="%s"/></navPoint>`+"\n", i+1, i+1, xmlEscape(ch.Title), fileName))

		var chDoc strings.Builder
		chDoc.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="zh-CN">
<head><meta charset="utf-8"/><title>` + xmlEscape(ch.Title) + `</title></head>
<body><h1>` + xmlEscape(ch.Title) + `</h1>`)
		for _, para := range strings.Split(ch.Body, "\n") {
			if strings.TrimSpace(para) != "" {
				chDoc.WriteString(`<p>` + xmlEscape(para) + `</p>`)
			}
		}
		chDoc.WriteString(`</body></html>`)
		if err := writeZipFile(w, "OEBPS/"+fileName, []byte(chDoc.String())); err != nil {
			return nil, err
		}
	}

	opf := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="pub-id">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="pub-id">urn:uuid:%s</dc:identifier>
    <dc:title>%s</dc:title>
    <dc:creator>%s</dc:creator>
    <dc:language>zh-CN</dc:language>
    <meta property="dcterms:modified">%s</meta>
  </metadata>
  <manifest>
%s
  </manifest>
  <spine toc="ncx">
%s
  </spine>
</package>`, novel.ID, xmlEscape(novel.Title), xmlEscape(author), modified, manifestItems.String(), spineItems.String())
	if err := writeZipFile(w, "OEBPS/package.opf", []byte(opf)); err != nil {
		return nil, err
	}

	navXHTML := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="zh-CN">
<head><meta charset="utf-8"/><title>目录</title></head>
<body>
  <nav epub:type="toc" id="toc">
    <h1>目录</h1>
    <ol>
%s
    </ol>
  </nav>
</body>
</html>`, navItems.String())
	if err := writeZipFile(w, "OEBPS/nav.xhtml", []byte(navXHTML)); err != nil {
		return nil, err
	}

	tocNCX := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1">
  <head><meta name="dtb:uid" content="urn:uuid:%s"/></head>
  <docTitle><text>%s</text></docTitle>
  <navMap>
%s
  </navMap>
</ncx>`, novel.ID, xmlEscape(novel.Title), ncxItems.String())
	if err := writeZipFile(w, "OEBPS/toc.ncx", []byte(tocNCX)); err != nil {
		return nil, err
	}

	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeZipFile(w *zip.Writer, name string, content []byte) error {
	f, err := w.Create(name)
	if err != nil {
		return err
	}
	_, err = f.Write(content)
	return err
}
