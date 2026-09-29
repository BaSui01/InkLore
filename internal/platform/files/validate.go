package files

import (
	"InkLore/internal/platform/scope"
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"io"
	"io/fs"
	"mime"
	"path"
	"strings"
	"unicode"
	"unicode/utf8"
)

func PrepareUpload(r io.Reader, filename, mediaType, purpose string, novelID *string) (Prepared, error) {
	if purpose != "import" && purpose != "inspiration_source" {
		return Prepared{}, invalid()
	}
	return prepare(r, filename, mediaType, purpose, novelID, false)
}

// PrepareOutput is for trusted export applications, never multipart upload handlers.
func PrepareOutput(r io.Reader, filename, mediaType, purpose string, novelID *string) (Prepared, error) {
	if purpose != "export" && purpose != "audit_export" {
		return Prepared{}, invalid()
	}
	return prepare(r, filename, mediaType, purpose, novelID, true)
}
func prepare(r io.Reader, filename, mediaType, purpose string, novelID *string, output bool) (Prepared, error) {
	if r == nil || (novelID != nil && !scope.ValidID(*novelID)) {
		return Prepared{}, invalid()
	}
	filename = path.Base(strings.ReplaceAll(filename, "\\", "/"))
	filename = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, filename)
	if !utf8.ValidString(filename) || filename == "" || filename == "." || filename == "/" || utf8.RuneCountInString(filename) > 180 {
		return Prepared{}, invalid()
	}
	limit := InputLimit
	if output {
		limit = OutputLimit
	}
	b, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return Prepared{}, err
	}
	if int64(len(b)) > limit {
		return Prepared{}, tooLarge()
	}
	mt, params, err := mime.ParseMediaType(mediaType)
	if err != nil {
		return Prepared{}, badType()
	}
	ext := strings.ToLower(path.Ext(filename))
	switch ext {
	case ".txt":
		if mt != "text/plain" || !validText(b) {
			return Prepared{}, badType()
		}
		if c := strings.ToLower(params["charset"]); c != "" && c != "utf-8" && c != "utf-16" && c != "utf-16le" && c != "utf-16be" {
			return Prepared{}, badType()
		}
		c := strings.ToLower(params["charset"])
		le := len(b) >= 2 && b[0] == 255 && b[1] == 254
		be := len(b) >= 2 && b[0] == 254 && b[1] == 255
		if (strings.HasPrefix(c, "utf-16") && !le && !be) || (c == "utf-8" && (le || be)) || (c == "utf-16le" && !le) || (c == "utf-16be" && !be) {
			return Prepared{}, badType()
		}
	case ".docx":
		if mt != DOCX || validateDOCX(b) != nil {
			return Prepared{}, badType()
		}
	case ".json":
		if purpose != "import" && purpose != "export" {
			return Prepared{}, badType()
		}
		if mt != "application/json" || !utf8.Valid(b) || !json.Valid(b) {
			return Prepared{}, badType()
		}
	case ".ndjson":
		if !output || purpose != "audit_export" || mt != "application/x-ndjson" || !validNDJSON(b) {
			return Prepared{}, badType()
		}
	default:
		return Prepared{}, badType()
	}
	sum := sha256.Sum256(b)
	var novel *string
	if novelID != nil {
		n := *novelID
		novel = &n
	}
	return Prepared{descriptor: Descriptor{Filename: filename, MediaType: mt, Purpose: purpose, SizeBytes: int64(len(b)), SHA256: hex.EncodeToString(sum[:])}, payload: b, novelID: novel, output: output}, nil
}
func validText(b []byte) bool {
	if len(b) >= 2 && ((b[0] == 255 && b[1] == 254) || (b[0] == 254 && b[1] == 255)) {
		var order binary.ByteOrder = binary.LittleEndian
		if b[0] == 254 {
			order = binary.BigEndian
		}
		b = b[2:]
		if len(b)%2 != 0 {
			return false
		}
		for i := 0; i < len(b); i += 2 {
			u := order.Uint16(b[i:])
			if u == 0 {
				return false
			}
			if u >= 0xd800 && u <= 0xdbff {
				if i+3 >= len(b) {
					return false
				}
				v := order.Uint16(b[i+2:])
				if v < 0xdc00 || v > 0xdfff {
					return false
				}
				i += 2
			} else if u >= 0xdc00 && u <= 0xdfff {
				return false
			}
		}
		return true
	}
	if !utf8.Valid(b) {
		return false
	}
	for _, r := range string(b) {
		if r == 0 || (unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t') {
			return false
		}
	}
	return true
}
func validNDJSON(b []byte) bool {
	for _, line := range bytes.Split(b, []byte{'\n'}) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		if !json.Valid(line) {
			return false
		}
	}
	return utf8.Valid(b)
}
func validateDOCX(b []byte) error {
	if len(b) < 4 || !bytes.Equal(b[:4], []byte{'P', 'K', 3, 4}) {
		return badType()
	}
	z, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil || len(z.File) > 10000 {
		return badType()
	}
	var total uint64
	seen := map[string]bool{}
	document, types := false, false
	for _, f := range z.File {
		name := f.Name
		clean := path.Clean(name)
		if strings.Contains(name, "\\") || strings.Contains(name, ":") || strings.HasPrefix(name, "/") || clean == ".." || strings.HasPrefix(clean, "../") || strings.TrimSuffix(name, "/") != clean || seen[strings.ToLower(name)] || (f.Mode()&fs.ModeType != 0 && !f.FileInfo().IsDir()) {
			return badType()
		}
		seen[strings.ToLower(name)] = true
		if f.Flags&1 != 0 {
			return badType()
		}
		if f.FileInfo().IsDir() {
			continue
		}
		if f.CompressedSize64 > uint64(len(b)) || f.UncompressedSize64 > uint64(OutputLimit) || total > uint64(OutputLimit)-f.UncompressedSize64 {
			return badType()
		}
		total += f.UncompressedSize64
		if f.UncompressedSize64 > 0 && (f.CompressedSize64 == 0 || f.UncompressedSize64/f.CompressedSize64 > 100 || f.UncompressedSize64 > f.CompressedSize64*100) {
			return badType()
		}
		lower := strings.ToLower(name)
		if strings.Contains(lower, "vbaproject") || strings.HasSuffix(lower, ".bin") || strings.Contains(lower, "macros") {
			return badType()
		}
		rc, e := f.Open()
		if e != nil {
			return badType()
		}
		data, e := io.ReadAll(io.LimitReader(rc, OutputLimit+1))
		closeErr := rc.Close()
		if e != nil || closeErr != nil || uint64(len(data)) != f.UncompressedSize64 {
			return badType()
		}
		if name == "word/document.xml" {
			document = true
		}
		if name == "[Content_Types].xml" {
			types = true
		}
		if strings.HasSuffix(lower, ".xml") || strings.HasSuffix(lower, ".rels") {
			d := xml.NewDecoder(bytes.NewReader(data))
			root := false
			for {
				token, e := d.Token()
				if e == io.EOF {
					break
				}
				if e != nil {
					return badType()
				}
				switch t := token.(type) {
				case xml.Directive:
					return badType()
				case xml.StartElement:
					if !root {
						root = true
						if name == "word/document.xml" && (t.Name.Local != "document" || t.Name.Space != "http://schemas.openxmlformats.org/wordprocessingml/2006/main") {
							return badType()
						}
					}
					for _, a := range t.Attr {
						v := strings.ToLower(a.Value)
						if a.Name.Local == "TargetMode" && strings.EqualFold(a.Value, "External") {
							return badType()
						}
						if a.Name.Local == "ContentType" && (strings.Contains(v, "macroenabled") || strings.Contains(v, "vbaproject")) {
							return badType()
						}
						if strings.HasSuffix(lower, ".rels") && a.Name.Local == "Target" && (strings.Contains(v, ":") || strings.HasPrefix(v, "//")) {
							return badType()
						}
					}
				}
			}
			if !root {
				return badType()
			}
		}
	}
	if !document || !types {
		return badType()
	}
	return nil
}
