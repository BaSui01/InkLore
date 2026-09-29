package files

import (
	pe "InkLore/internal/platform/errors"
	"InkLore/internal/platform/identity"
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
)

// ByteRange is inclusive. ParseRange supports exactly one RFC bytes range.
type ByteRange struct{ Start, End int64 }

func ParseRange(value string, size int64) (ByteRange, error) {
	bad := func() (ByteRange, error) { return ByteRange{}, pe.Contract(416, "platform.range_invalid", nil) }
	if size < 0 {
		return bad()
	}
	if value == "" {
		return ByteRange{0, size - 1}, nil
	}
	if !strings.HasPrefix(value, "bytes=") || strings.ContainsAny(value, ", \t\r\n") || size == 0 {
		return bad()
	}
	a, b, ok := strings.Cut(strings.TrimPrefix(value, "bytes="), "-")
	if !ok || (a == "" && b == "") {
		return bad()
	}
	parse := func(s string) (int64, error) {
		for _, r := range s {
			if r < '0' || r > '9' {
				return 0, fmt.Errorf("invalid range")
			}
		}
		return strconv.ParseInt(s, 10, 64)
	}
	if a == "" {
		n, e := parse(b)
		if e != nil || n <= 0 {
			return bad()
		}
		if n > size {
			n = size
		}
		return ByteRange{size - n, size - 1}, nil
	}
	start, e := parse(a)
	if e != nil || start < 0 || start >= size {
		return bad()
	}
	end := size - 1
	if b != "" {
		end, e = parse(b)
		if e != nil || end < start {
			return bad()
		}
		if end >= size {
			end = size - 1
		}
	}
	return ByteRange{start, end}, nil
}

// Download authorizes before evaluating ETag or Range. Errors before writing can
// be rendered by the composition root as the normal JSON envelope.
func (s *Service) Download(ctx context.Context, p identity.Principal, id string, w http.ResponseWriter, r *http.Request) error {
	opened, e := s.Open(ctx, p, id)
	if e != nil {
		return e
	}
	d := opened.Descriptor
	etag := "\"" + d.SHA256 + "\""
	match := false
	for _, candidate := range strings.Split(r.Header.Get("If-None-Match"), ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" || candidate == etag || candidate == "W/"+etag {
			match = true
		}
	}
	if match {
		w.Header().Set("ETag", etag)
		w.Header().Set("Cache-Control", "private, no-cache")
		w.WriteHeader(http.StatusNotModified)
		return nil
	}
	rangeHeader := r.Header.Get("Range")
	if ir := r.Header.Get("If-Range"); ir != "" && ir != etag {
		rangeHeader = ""
	}
	br, e := ParseRange(rangeHeader, d.SizeBytes)
	if e != nil {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", d.SizeBytes))
		return e
	}
	h := w.Header()
	h.Set("ETag", etag)
	h.Set("Accept-Ranges", "bytes")
	h.Set("Content-Type", d.MediaType)
	h.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": d.Filename}))
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Cache-Control", "private, no-cache")
	status := http.StatusOK
	n := d.SizeBytes
	if rangeHeader != "" {
		status = http.StatusPartialContent
		n = br.End - br.Start + 1
		h.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", br.Start, br.End, d.SizeBytes))
	}
	h.Set("Content-Length", strconv.FormatInt(n, 10))
	if _, e = opened.Reader.Seek(br.Start, io.SeekStart); e != nil {
		return e
	}
	w.WriteHeader(status)
	_, e = io.CopyN(w, opened.Reader, n)
	return e
}
