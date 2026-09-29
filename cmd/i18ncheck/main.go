// Command i18ncheck 校验前端多语言词条 key 集合是否完全一致。
//
// 用法：
//
//	go run ./cmd/i18ncheck frontend/src/shared/i18n/locales/zh-CN.ts frontend/src/shared/i18n/locales/en-US.ts
//
// 为什么要独立工具：i18n 缺失 key 在运行时不报错、类型检查也发现不了，
// 只表现为界面回退到原始 key 字符串，很容易在并行开发中悄悄漂移。
//
// 支持两种写法：
//
//	"common.success": "操作成功"        （点分平面 key）
//	common: { success: "操作成功" }     （嵌套对象，路径拼接为 common.success）
//
// 退出码：0 一致；1 存在差异；2 参数、读取或解析错误。
package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// 以 byte 常量代替 rune 字面量：避免源码里出现大量转义字符。
const (
	chTab         = byte(9)
	chLF          = byte(10)
	chCR          = byte(13)
	chSpace       = byte(32)
	chQuote       = byte(34)
	chSingleQuote = byte(39)
	chStar        = byte(42)
	chSlash       = byte(47)
	chColon       = byte(58)
	chBackslash   = byte(92)
	chOpenBrace   = byte(123)
	chCloseBrace  = byte(125)
)

func main() {
	if len(os.Args) < 3 {
		_, _ = fmt.Fprintln(os.Stderr, "用法: i18ncheck <基准文件> <对比文件> [更多文件...]")
		os.Exit(2)
	}

	basePath := os.Args[1]
	baseKeys, err := extractKeys(basePath)
	if err != nil {
		reportParseFailure(basePath, err)
		os.Exit(2)
	}

	failed := false
	for _, target := range os.Args[2:] {
		targetKeys, targetErr := extractKeys(target)
		if targetErr != nil {
			reportParseFailure(target, targetErr)
			os.Exit(2)
		}
		missing := difference(baseKeys, targetKeys)
		extra := difference(targetKeys, baseKeys)

		fmt.Printf("%s  基准 %d 条 / 对比 %d 条", target, len(baseKeys), len(targetKeys))
		fmt.Println()
		if len(missing) == 0 && len(extra) == 0 {
			fmt.Println("  一致")
			continue
		}
		failed = true
		for _, key := range missing {
			fmt.Printf("  缺失: %s", key)
			fmt.Println()
		}
		for _, key := range extra {
			fmt.Printf("  多余: %s", key)
			fmt.Println()
		}
	}

	if failed {
		fmt.Println()
		fmt.Println("结果：词条 key 集合不一致")
		os.Exit(1)
	}
	fmt.Println()
	fmt.Println("结果：全部一致")
}

func reportParseFailure(path string, err error) {
	_, _ = fmt.Fprint(os.Stderr, "i18ncheck: 解析失败 ")
	_, _ = fmt.Fprint(os.Stderr, path)
	_, _ = fmt.Fprint(os.Stderr, ": ")
	_, _ = fmt.Fprintln(os.Stderr, err)
}

func difference(left map[string]struct{}, right map[string]struct{}) []string {
	out := make([]string, 0)
	for key := range left {
		if _, ok := right[key]; !ok {
			out = append(out, key)
		}
	}
	sort.Strings(out)
	return out
}

// extractKeys 单遍扫描 TS 对象字面量，返回点分路径形式的 key 集合。
//
// 关键点：遇到冒号时**不能立刻判定为词条**——值可能是嵌套对象（命名空间）。
// 因此记录 lastKey 后进入 awaitingValue，由后续第一个非空白字符决定：
// 遇到 { 则是命名空间（压栈、不记 key），否则才是叶子词条。
func extractKeys(path string) (map[string]struct{}, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	text := strings.ReplaceAll(string(raw),
		string([]byte{chCR, chLF}),
		string([]byte{chLF}))

	keys := make(map[string]struct{})
	stack := make([]string, 0, 8)

	lastKey := ""
	awaitingValue := false
	inString := false
	activeQuote := byte(0)
	inLineComment := false
	inBlockComment := false
	escaped := false
	buffer := strings.Builder{}

	for i := 0; i < len(text); i++ {
		ch := text[i]

		if inLineComment {
			if ch == chLF {
				inLineComment = false
			}
			continue
		}
		if inBlockComment {
			if ch == chStar && i+1 < len(text) && text[i+1] == chSlash {
				inBlockComment = false
				i++
			}
			continue
		}
		if inString {
			if escaped {
				buffer.WriteByte(ch)
				escaped = false
				continue
			}
			if ch == chBackslash {
				escaped = true
				continue
			}
			if ch == activeQuote {
				inString = false
				candidate := buffer.String()
				buffer.Reset()
				if nextNonSpaceIsColon(text, i+1) {
					lastKey = candidate
				}
				continue
			}
			buffer.WriteByte(ch)
			continue
		}

		// 值的第一个非空白字符：决定是命名空间还是叶子词条。
		if awaitingValue {
			switch ch {
			case chSpace, chTab, chLF, chCR:
				continue
			case chOpenBrace:
				stack = append(stack, lastKey)
				awaitingValue = false
				continue
			default:
				if len(stack) > 0 && lastKey != "" {
					keys[joinPath(stack, lastKey)] = struct{}{}
				}
				awaitingValue = false
			}
		}

		switch {
		case ch == chSlash && i+1 < len(text) && text[i+1] == chSlash:
			inLineComment = true
			i++
		case ch == chSlash && i+1 < len(text) && text[i+1] == chStar:
			inBlockComment = true
			i++
		case ch == chQuote || ch == chSingleQuote:
			inString = true
			activeQuote = ch
			buffer.Reset()
		case isIdentifierStart(ch):
			start := i
			for i+1 < len(text) && isIdentifierPart(text[i+1]) {
				i++
			}
			if len(stack) > 0 && nextNonSpaceIsColon(text, i+1) {
				lastKey = text[start : i+1]
			}
		case ch == chCloseBrace:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		case ch == chOpenBrace:
			stack = append(stack, "")
		case ch == chColon:
			awaitingValue = true
		}
	}

	if len(keys) == 0 {
		return nil, fmt.Errorf("未解析到任何词条 key，解析器可能不支持该文件的语法")
	}
	return keys, nil
}

func nextNonSpaceIsColon(text string, from int) bool {
	for i := from; i < len(text); i++ {
		switch text[i] {
		case chSpace, chTab, chLF, chCR:
			continue
		case chColon:
			return true
		default:
			return false
		}
	}
	return false
}

func isIdentifierStart(ch byte) bool {
	return ch == byte(95) ||
		(ch >= byte(97) && ch <= byte(122)) ||
		(ch >= byte(65) && ch <= byte(90))
}

func isIdentifierPart(ch byte) bool {
	return isIdentifierStart(ch) || (ch >= byte(48) && ch <= byte(57))
}

func joinPath(stack []string, key string) string {
	parts := make([]string, 0, len(stack)+1)
	for _, item := range stack {
		if item != "" {
			parts = append(parts, item)
		}
	}
	parts = append(parts, key)
	return strings.Join(parts, ".")
}
