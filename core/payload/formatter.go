package payload

import (
	"fmt"
	"io"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Context struct {
	Host      string
	Port      int
	UserAgent string
}

type Formatter struct {
	mu      sync.Mutex
	rotate  map[string]int
	random  *rand.Rand
}

func NewFormatter() *Formatter {
	return &Formatter{
		rotate: make(map[string]int),
		random: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

var choicePattern = regexp.MustCompile(`(?i)\[(rotate|random)=([^\]]+)\]`)

func (f *Formatter) Format(input string, ctx Context) (string, error) {
	if ctx.Port < 1 || ctx.Port > 65535 {
		return "", fmt.Errorf("invalid destination port: %d", ctx.Port)
	}
	hostPort := ctx.Host + ":" + strconv.Itoa(ctx.Port)
	ua := ctx.UserAgent
	if ua == "" {
		ua = "Mozilla/5.0"
	}

	replacements := map[string]string{
		"[method]": "CONNECT",
		"[host]": ctx.Host,
		"[port]": strconv.Itoa(ctx.Port),
		"[host_port]": hostPort,
		"[protocol]": "HTTP/1.0",
		"[ssh]": hostPort,
		"[crlf]": "\r\n",
		"[cr]": "\r",
		"[lf]": "\n",
		"[lfcr]": "\n\r",
		"[ua]": ua,
	}
	out := input
	for key, value := range replacements {
		out = replaceFold(out, key, value)
	}
	out = strings.ReplaceAll(out, `\n`, "\n")
	out = strings.ReplaceAll(out, `\r`, "\r")

	f.mu.Lock()
	defer f.mu.Unlock()
	out = choicePattern.ReplaceAllStringFunc(out, func(token string) string {
		match := choicePattern.FindStringSubmatch(token)
		if len(match) != 3 {
			return token
		}
		choices := strings.Split(match[2], ";")
		if len(choices) == 0 {
			return token
		}
		for i := range choices {
			choices[i] = strings.TrimSpace(choices[i])
		}
		if strings.EqualFold(match[1], "rotate") {
			key := match[2]
			index := f.rotate[key] % len(choices)
			f.rotate[key]++
			return choices[index]
		}
		return choices[f.random.Intn(len(choices))]
	})
	return out, nil
}

func replaceFold(input, old, replacement string) string {
	for {
		index := strings.Index(strings.ToLower(input), strings.ToLower(old))
		if index < 0 {
			return input
		}
		input = input[:index] + replacement + input[index+len(old):]
	}
}

func WriteFragments(w io.Writer, payload string, mode string, delay time.Duration) error {
	parts := strings.Split(payload, "[split]")
	if mode == "delay_split" {
		parts = strings.Split(payload, "[delay_split]")
	}
	for i, part := range parts {
		if _, err := io.WriteString(w, part); err != nil {
			return err
		}
		if f, ok := w.(interface{ Flush() error }); ok {
			if err := f.Flush(); err != nil {
				return err
			}
		}
		if mode == "delay_split" && i < len(parts)-1 {
			time.Sleep(delay)
		}
	}
	return nil
}
