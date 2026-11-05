package importpath

import (
	"regexp"
	"strings"
)

// 成对的括号：内容多是日期、品牌、版本、汉化组等，与搜索无关。
// 不含「」『』，它们常是标题本身的一部分。
var bracketPairPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\[[^\[\]]*\]`),
	regexp.MustCompile(`［[^［］]*］`),
	regexp.MustCompile(`【[^【】]*】`),
	regexp.MustCompile(`〔[^〔〕]*〕`),
	regexp.MustCompile(`\([^()]*\)`),
	regexp.MustCompile(`（[^（）]*）`),
	regexp.MustCompile(`\{[^{}]*\}`),
}

// 版本说明，可出现在名称任意位置，可带前后的 -、~ 包裹（如 "-Limited Edition-"）。
var editionPattern = regexp.MustCompile(`(?i)[-~～]?\s*(?:` +
	`\b(?:limited|deluxe|complete|special|standard|regular|premium|collector'?s|digital|perfect|full|trial|first\s+press|download|package|dl)\s+(?:edition|version|ver\.?)\b` +
	`|初回限定版|初回版|限定版|通常版|豪華版|豪华版|特装版|特裝版|完全版|廉価版|新装版|普及版|典藏版|完整版|体験版|體驗版|` +
	`DL版|ダウンロード版|パッケージ版|汉化硬盘版|硬盘版|免安装版|免安装|绿色版|整合版|` +
	`简体中文版|繁体中文版|簡體中文版|繁體中文版|中文版|汉化版|漢化版|官方中文|官中|简中|繁中|汉化|漢化|精翻|机翻|機翻` +
	`)\s*[-~～]?`)

// 只在名称首尾剥离的噪声词：日期、版本号、作品编号、语言与发布形式等。
var edgeNoisePatterns = []*regexp.Regexp{
	regexp.MustCompile(`^\d{6}$|^\d{8}$`),                             // 110624、20110624
	regexp.MustCompile(`^\d{2,4}[-./]\d{1,2}[-./]\d{1,2}$`),           // 2011-06-24、11.06.24
	regexp.MustCompile(`(?i)^v(?:er)?\.?\d+(?:\.\d+)*[a-z]?$`),        // v1.02、ver1.0、Ver.2
	regexp.MustCompile(`^\d+(?:\.\d+)+$`),                             // 1.02（"Ver 1.02" 拆开后的版本号）
	regexp.MustCompile(`(?i)^(?:rj|vj|bj|re|rg)\d{5,}$|^(?i:d)_\d+$`), // DLsite、FANZA 作品编号
	regexp.MustCompile(`(?i)^(?:ver|version|ch|chs|cht|cn|sc|tc|jp|jpn|eng|dl|file|files|iso|mdf|mds|rip|repack|zip|rar|7z)$`),
}

const searchNameSeparators = " \t-~～_.,，、・+&|"

// CleanSearchName 把游戏文件夹名清洗成用于搜索元数据的名称，例如
// "[110624][Navel] 俺たちに翼はない R -Limited Edition- (nrg)" → "俺たちに翼はない R"。
// 去掉括号内容、版本说明，以及首尾的日期、版本号、语言等噪声词；清洗后为空时返回原名。
func CleanSearchName(name string) string {
	original := strings.TrimSpace(name)
	if original == "" {
		return ""
	}

	stripped, inner := stripBrackets(original)
	if cleaned := cleanBareName(stripped); cleaned != "" {
		return cleaned
	}

	// 名称整个在括号里（如 "【俺たちに翼はない】汉化版"）：取括号里最长的有效内容。
	best := ""
	for _, part := range inner {
		if cleaned := cleanBareName(part); len([]rune(cleaned)) > len([]rune(best)) {
			best = cleaned
		}
	}
	if best != "" {
		return best
	}
	return original
}

// stripBrackets 由内向外去掉成对括号，返回剩余文本与各括号内的文本。
func stripBrackets(s string) (string, []string) {
	var inner []string
	for changed := true; changed; {
		changed = false
		for _, re := range bracketPairPatterns {
			s = re.ReplaceAllStringFunc(s, func(m string) string {
				changed = true
				r := []rune(m)
				inner = append(inner, string(r[1:len(r)-1]))
				return " "
			})
		}
	}
	return s, inner
}

func cleanBareName(s string) string {
	s = strings.ReplaceAll(s, "_", " ")
	s = editionPattern.ReplaceAllString(s, " ")

	tokens := strings.Fields(s)
	for len(tokens) > 0 && isEdgeNoise(tokens[0]) {
		tokens = tokens[1:]
	}
	for len(tokens) > 0 && isEdgeNoise(tokens[len(tokens)-1]) {
		tokens = tokens[:len(tokens)-1]
	}
	return strings.Trim(strings.Join(tokens, " "), searchNameSeparators)
}

func isEdgeNoise(token string) bool {
	token = strings.Trim(token, searchNameSeparators)
	if token == "" {
		return true
	}
	for _, re := range edgeNoisePatterns {
		if re.MatchString(token) {
			return true
		}
	}
	return false
}
