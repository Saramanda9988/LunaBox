package locales_test

import (
	"go/scanner"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"
)

// 界面代码里不应该有写死的中文、日文：文字都要来自 ui/locales（见 docs/i18n.md）。确实不该翻译的
// （游戏数据、给开发者看的文字）在同一行写注释 i18n:keep 并说明原因。测试数据（_test.go、envtest）不检查。
func TestNoHardcodedText(t *testing.T) {
	root := ".."
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "envtest" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		lines := strings.Split(string(src), "\n")
		fset := token.NewFileSet()
		file := fset.AddFile(path, -1, len(src))
		var s scanner.Scanner
		s.Init(file, src, nil, 0)
		for {
			pos, tok, lit := s.Scan()
			if tok == token.EOF {
				break
			}
			if (tok != token.STRING && tok != token.CHAR) || !hasCJK(lit) {
				continue
			}
			p := fset.Position(pos)
			if strings.Contains(lines[p.Line-1], "i18n:keep") {
				continue
			}
			rel, _ := filepath.Rel(root, path)
			t.Errorf("ui/%s:%d: 写死的文字 %s，改用 ui/locales 的函数（docs/i18n.md）", filepath.ToSlash(rel), p.Line, lit)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func hasCJK(s string) bool {
	for _, r := range s {
		if unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana) || (r >= 0xff01 && r <= 0xff5e) || (r >= 0x3000 && r <= 0x303f) {
			return true
		}
	}
	return false
}
