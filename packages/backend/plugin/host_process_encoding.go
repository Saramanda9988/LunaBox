package plugin

import (
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
)

// decodeLegacy 用常见的东亚旧编码解码命令输出。
func decodeLegacy(data []byte, name string) (string, bool) {
	var enc encoding.Encoding
	switch name {
	case "gbk", "gb2312", "gb18030", "cp936":
		enc = simplifiedchinese.GB18030
	case "big5", "cp950":
		enc = traditionalchinese.Big5
	case "shift_jis", "shift-jis", "sjis", "cp932":
		enc = japanese.ShiftJIS
	case "euc-kr", "cp949":
		enc = korean.EUCKR
	default:
		return "", false
	}
	out, err := enc.NewDecoder().Bytes(data)
	if err != nil {
		return "", false
	}
	return string(out), true
}
