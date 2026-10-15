package iplookup

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/url"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/jsonlimit"
	"github.com/ghodss/yaml"
	"golang.org/x/net/html"
)

const maxComplexInventoryBody = 1 << 20

func readJSONInventory(ctx context.Context, reader io.Reader, emit func(string)) error {
	decoder := json.NewDecoder(jsonlimit.NewReader(reader, jsonlimit.Limits{Depth: 64, StringBytes: maxProxyBody}))
	var walk func(string, int) error
	walk = func(key string, depth int) error {
		if depth > 64 {
			return &CodeError{"IP_PROXY_LIMIT"}
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		token, err := decoder.Token()
		if err != nil {
			return &CodeError{"IP_PROXY_PARSE"}
		}
		switch value := token.(type) {
		case json.Delim:
			if value == '{' {
				for decoder.More() {
					k, err := decoder.Token()
					if err != nil {
						return &CodeError{"IP_PROXY_PARSE"}
					}
					name, ok := k.(string)
					if !ok {
						return &CodeError{"IP_PROXY_PARSE"}
					}
					if err := walk(name, depth+1); err != nil {
						return err
					}
				}
			} else if value == '[' {
				for decoder.More() {
					if err := walk("", depth+1); err != nil {
						return err
					}
				}
			} else {
				return &CodeError{"IP_PROXY_PARSE"}
			}
			if _, err := decoder.Token(); err != nil {
				return &CodeError{"IP_PROXY_PARSE"}
			}
		case string:
			if key == "server" || key == "address" || key == "add" {
				if host := proxyInventoryEntry(value); host != "" {
					emit(host)
				}
			}
		}
		return nil
	}
	if err := walk("", 0); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return &CodeError{"IP_PROXY_PARSE"}
	}
	return nil
}

func readHTMLInventory(ctx context.Context, reader io.Reader, emit func(string)) error {
	tokenizer := html.NewTokenizer(reader)
	tokenizer.SetMaxBuf(maxComplexInventoryBody)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		kind := tokenizer.Next()
		if kind == html.ErrorToken {
			if tokenizer.Err() == io.EOF {
				return nil
			}
			return &CodeError{"IP_PROXY_PARSE"}
		}
		if err := tokenizer.Err(); err != nil && err != io.EOF {
			return &CodeError{"IP_PROXY_PARSE"}
		}
		token := tokenizer.Token()
		add := func(raw string) {
			if host := proxyInventoryEntry(raw); host != "" {
				emit(host)
			}
		}
		if kind == html.TextToken {
			add(token.Data)
		}
		for _, attribute := range token.Attr {
			if attribute.Key == "href" {
				if link, err := url.Parse(attribute.Val); err == nil {
					add(link.Query().Get("ip"))
				}
			}
		}
	}
}

func readClashInventory(ctx context.Context, reader io.Reader, limited *io.LimitedReader, emit func(string)) error {
	scanner := bufio.NewScanner(reader)
	scanner.Split(inventoryLineSplit(limited))
	scanner.Buffer(make([]byte, 64<<10), maxComplexInventoryBody+1)
	var record strings.Builder
	indent := -1
	finished := false
	flush := func() error {
		if record.Len() == 0 {
			return nil
		}
		var value any
		err := yaml.Unmarshal([]byte("proxies:\n"+record.String()), &value)
		if err == nil {
			walkProxyConfig(value, "", func(raw string) {
				if host := proxyInventoryEntry(raw); host != "" {
					emit(host)
				}
			})
		}
		record.Reset()
		if err != nil {
			return &CodeError{"IP_PROXY_PARSE"}
		}
		return nil
	}
	for scanner.Scan() {
		if scanner.Err() != nil {
			return &CodeError{"IP_PROXY_PARSE"}
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if finished {
			continue
		}
		if trimmed == "proxies:" || trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "proxies:") {
			var value any
			if yaml.Unmarshal([]byte(line), &value) != nil {
				return &CodeError{"IP_PROXY_PARSE"}
			}
			walkProxyConfig(value, "", func(raw string) {
				if host := proxyInventoryEntry(raw); host != "" {
					emit(host)
				}
			})
			finished = true
			continue
		}
		leading := len(line) - len(strings.TrimLeft(line, " \t"))
		if strings.HasPrefix(trimmed, "- ") {
			if indent < 0 {
				indent = leading
			}
			if leading == indent {
				if err := flush(); err != nil {
					return err
				}
			}
		} else if indent >= 0 && leading < indent {
			if err := flush(); err != nil {
				return err
			}
			finished = true
			continue
		}
		if record.Len()+len(line)+10 > maxComplexInventoryBody {
			return &CodeError{"IP_PROXY_LIMIT"}
		}
		record.WriteString(line)
		record.WriteByte('\n')
	}
	if err := scanner.Err(); err != nil {
		return &CodeError{"IP_PROXY_LIMIT"}
	}
	return flush()
}
