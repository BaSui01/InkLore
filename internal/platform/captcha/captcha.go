package captcha

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type CaptchaItem struct {
	Code      string
	ExpiresAt time.Time
}

type Service struct {
	mu    sync.RWMutex
	items map[string]CaptchaItem
	ttl   time.Duration
}

func NewService() *Service {
	return &Service{
		items: make(map[string]CaptchaItem),
		ttl:   3 * time.Minute,
	}
}

const captchaAlphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"

func (s *Service) Generate() (string, string, error) {
	s.cleanupExpired()

	code := make([]byte, 4)
	maxIdx := big.NewInt(int64(len(captchaAlphabet)))
	for i := 0; i < 4; i++ {
		idx, err := rand.Int(rand.Reader, maxIdx)
		if err != nil {
			return "", "", fmt.Errorf("generate captcha random char: %w", err)
		}
		code[i] = captchaAlphabet[idx.Int64()]
	}
	captchaCode := string(code)

	svg, err := renderSVG(captchaCode)
	if err != nil {
		return "", "", fmt.Errorf("render captcha svg: %w", err)
	}

	id := uuid.NewString()
	s.mu.Lock()
	s.items[id] = CaptchaItem{
		Code:      captchaCode,
		ExpiresAt: time.Now().Add(s.ttl),
	}
	s.mu.Unlock()

	return id, svg, nil
}

func (s *Service) Verify(id string, code string) bool {
	if id == "" || strings.TrimSpace(code) == "" {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	item, exists := s.items[id]
	if !exists {
		return false
	}

	delete(s.items, id)

	if time.Now().After(item.ExpiresAt) {
		return false
	}

	return strings.EqualFold(strings.TrimSpace(item.Code), strings.TrimSpace(code))
}

func (s *Service) cleanupExpired() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for id, item := range s.items {
		if now.After(item.ExpiresAt) {
			delete(s.items, id)
		}
	}
}

func renderSVG(code string) (string, error) {
	const width = 120
	const height = 42

	var b strings.Builder
	b.WriteString(fmt.Sprintf(`<svg width="100%%" height="100%%" viewBox="0 0 %d %d" xmlns="http://www.w3.org/2000/svg" style="background:transparent;border-radius:8px;user-select:none;display:block;">`, width, height))
	b.WriteString(`<defs>`)
	b.WriteString(`<style>`)
	b.WriteString(`.c-bg{fill:var(--bg-subtle,#eeeae1);}`)
	b.WriteString(`:root.dark .c-bg{fill:var(--bg-subtle,#273449);}`)
	b.WriteString(`.c-line{stroke:var(--border-control,#6b7280);opacity:0.35;}`)
	b.WriteString(`:root.dark .c-line{stroke:var(--border-control,#94a3b8);opacity:0.45;}`)
	b.WriteString(`.c-dot{fill:var(--text-muted,#526071);opacity:0.25;}`)
	b.WriteString(`:root.dark .c-dot{fill:var(--text-muted,#b5c0d0);opacity:0.3;}`)
	b.WriteString(`.c-txt-0{fill:#1d4ed8;}:root.dark .c-txt-0{fill:#93c5fd;}`)
	b.WriteString(`.c-txt-1{fill:#4338ca;}:root.dark .c-txt-1{fill:#a5b4fc;}`)
	b.WriteString(`.c-txt-2{fill:#15803d;}:root.dark .c-txt-2{fill:#86efac;}`)
	b.WriteString(`.c-txt-3{fill:#b45309;}:root.dark .c-txt-3{fill:#fde68a;}`)
	b.WriteString(`.c-txt-4{fill:#b91c1c;}:root.dark .c-txt-4{fill:#fca5a5;}`)
	b.WriteString(`.c-txt-5{fill:#6d28d9;}:root.dark .c-txt-5{fill:#c4b5fd;}`)
	b.WriteString(`</style>`)
	b.WriteString(`</defs>`)
	b.WriteString(fmt.Sprintf(`<rect width="%d" height="%d" rx="8" class="c-bg"/>`, width, height))

	for i := 0; i < 4; i++ {
		x1, y1 := randInt(width/3), randInt(height)
		cx1, cy1 := randInt(width), randInt(height)
		cx2, cy2 := randInt(width), randInt(height)
		x2, y2 := width-randInt(width/3), randInt(height)
		strokeWidth := 1 + randInt(2)
		b.WriteString(fmt.Sprintf(`<path d="M%d,%d C%d,%d %d,%d %d,%d" stroke-width="%d" fill="none" class="c-line"/>`, x1, y1, cx1, cy1, cx2, cy2, x2, y2, strokeWidth))
	}

	for i := 0; i < 24; i++ {
		cx := randInt(width)
		cy := randInt(height)
		r := 1 + randInt(2)
		b.WriteString(fmt.Sprintf(`<circle cx="%d" cy="%d" r="%d" class="c-dot"/>`, cx, cy, r))
	}

	xStep := (width - 24) / len(code)
	for i, ch := range code {
		x := 14 + i*xStep + randInt(4)
		y := 27 + randInt(6) - 3
		rotate := randInt(31) - 15
		clsIdx := (i + randInt(6)) % 6
		fontSize := 21 + randInt(4)
		b.WriteString(fmt.Sprintf(`<text x="%d" y="%d" font-family="'SF Mono', Menlo, Monaco, Consolas, monospace" font-size="%d" font-weight="bold" class="c-txt-%d" transform="rotate(%d %d %d)" text-anchor="middle">%c</text>`, x, y, fontSize, clsIdx, rotate, x, y, ch))
	}

	b.WriteString("</svg>")
	return b.String(), nil
}

func randInt(max int) int {
	if max <= 0 {
		return 0
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0
	}
	return int(n.Int64())
}

