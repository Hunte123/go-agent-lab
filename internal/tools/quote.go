// Package tools 提供开箱即用的工具集：行情查询（新浪主/腾讯备）与
// 限定在 workdir 内的文件读写。文件工具做了路径约束，Agent 无法越界。
package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Hunte123/go-agent-lab/internal/agent"
	"golang.org/x/text/encoding/simplifiedchinese"
)

var hc = &http.Client{Timeout: 15 * time.Second}

func httpGetGBK(u string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Referer", "https://finance.sina.com.cn")
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := hc.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	out, err := simplifiedchinese.GBK.NewDecoder().Bytes(raw)
	return string(out), err
}

var kvRe = regexp.MustCompile(`="([^"]*)"`)

// fullCode 6位纯数字自动补市场前缀。
func fullCode(c string) string {
	c = strings.TrimSpace(c)
	if len(c) == 6 && isDigits(c) {
		if strings.HasPrefix(c, "6") {
			return "sh" + c
		}
		return "sz" + c
	}
	return c
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// GetQuote 注册实时行情工具（双数据源容灾）。
func GetQuote() agent.Tool {
	return agent.Tool{
		Name:        "get_quote",
		Description: "查询A股实时行情。code 支持 sh600519 / sz000001 / 纯6位数字；指数如 sz399001。盘前/停牌返回昨收。",
		Parameters: map[string]any{
			"code": map[string]any{"type": "string", "description": "股票代码，如 600519"},
		},
		Fn: func(args json.RawMessage) (string, error) {
			var in struct{ Code string `json:"code"` }
			if err := json.Unmarshal(args, &in); err != nil {
				return "", err
			}
			code := fullCode(in.Code)
			if out, ok := quoteSina(code); ok {
				return out, nil
			}
			if out, ok := quoteTencent(code); ok {
				return out, nil
			}
			return "", fmt.Errorf("未查到 %s", code)
		},
	}
}

func quoteSina(code string) (string, bool) {
	text, err := httpGetGBK("https://hq.sinajs.cn/list=" + code)
	if err != nil {
		return "", false
	}
	m := kvRe.FindStringSubmatch(text)
	if m == nil || m[1] == "" {
		return "", false
	}
	p := strings.Split(m[1], ",")
	if len(p) < 32 {
		return "", false
	}
	last, _ := strconv.ParseFloat(p[3], 64)
	prev, _ := strconv.ParseFloat(p[2], 64)
	if prev == 0 {
		return "", false
	}
	if last == 0 {
		return fmt.Sprintf("%s（%s）【盘前/停牌】昨收 %.2f", p[0], code, prev), true
	}
	return fmt.Sprintf("%s（%s）最新 %.2f（%+.2f%%）今开 %s 最高 %s 最低 %s 成交额 %.2f亿",
		p[0], code, last, (last-prev)/prev*100, p[1], p[4], p[5], parseFloat(p[9])/1e8), true
}

func quoteTencent(code string) (string, bool) {
	text, err := httpGetGBK("https://qt.gtimg.cn/q=" + code)
	if err != nil {
		return "", false
	}
	m := kvRe.FindStringSubmatch(text)
	if m == nil || len(m[1]) < 10 {
		return "", false
	}
	p := strings.Split(m[1], "~")
	if len(p) < 6 {
		return "", false
	}
	last, _ := strconv.ParseFloat(p[3], 64)
	prev, _ := strconv.ParseFloat(p[4], 64)
	if last == 0 || prev == 0 {
		return "", false
	}
	return fmt.Sprintf("%s（%s·腾讯）最新 %.2f（%+.2f%%）", p[1], code, last, (last-prev)/prev*100), true
}

func parseFloat(s string) float64 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return f
}
