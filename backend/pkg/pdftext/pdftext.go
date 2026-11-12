// Package pdftext — общий разбор текстового слоя PDF через pdftotext (poppler). Им пользуются парсеры
// выписок-PDF (Приват, брокерский отчёт ICU): процесс и разбор координат слов живут в одном месте.
package pdftext

import (
	"bytes"
	"fmt"
	"html"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// Run запускает pdftotext поверх сырого PDF: Run(raw, "-layout"), Run(raw, "-bbox"), Run(raw, "-l", "1", "-layout").
func Run(raw []byte, args ...string) (string, error) {
	cmd := exec.Command("pdftotext", append(args, "-", "-")...)
	cmd.Stdin = bytes.NewReader(raw)
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("pdftotext %s failed: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return out.String(), nil
}

// Word — слово страницы с координатами (pt, начало координат — левый верхний угол).
type Word struct {
	X0, Y0, X1, Y1 float64
	Text           string
}

// YC — центр слова по вертикали.
func (w Word) YC() float64 { return (w.Y0 + w.Y1) / 2 }

// XC — центр слова по горизонтали.
func (w Word) XC() float64 { return (w.X0 + w.X1) / 2 }

var bboxTokenRe = regexp.MustCompile(`<page [^>]*>|<word xMin="([\d.]+)" yMin="([\d.]+)" xMax="([\d.]+)" yMax="([\d.]+)">([^<]*)</word>`)

// ParseBBox разбирает вывод `pdftotext -bbox`: слова по страницам.
func ParseBBox(s string) [][]Word {
	var pages [][]Word
	for _, m := range bboxTokenRe.FindAllStringSubmatch(s, -1) {
		if strings.HasPrefix(m[0], "<page") {
			pages = append(pages, nil)
			continue
		}
		if len(pages) == 0 {
			continue
		}
		x0, _ := strconv.ParseFloat(m[1], 64)
		y0, _ := strconv.ParseFloat(m[2], 64)
		x1, _ := strconv.ParseFloat(m[3], 64)
		y1, _ := strconv.ParseFloat(m[4], 64)
		pages[len(pages)-1] = append(pages[len(pages)-1], Word{x0, y0, x1, y1, html.UnescapeString(m[5])})
	}
	return pages
}
