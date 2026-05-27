# 添加 DLsite 与 ErogameScape 元数据源方案

本文档说明如何在 LunaBox 中新增 DLsite 与 ErogameScape（批评空间）两个游戏元数据源。

当前项目已有统一的元数据抓取入口：

- 后端抓取实现：`internal/utils/metadata`
- 数据源枚举：`internal/common/enums/source_enum.go`
- 元数据分发：`internal/service/game_service.go`
- 用户可配置来源：`internal/appconf/config.go`
- 前端选择入口：添加游戏弹窗、批量导入弹窗、元数据设置页

建议不要照搬 vnite 的 scraper provider 架构，而是在 LunaBox 现有 `metadata.Getter` 接口上扩展。

## 目标

新增两个数据源：

- `dlsite`
- `erogamescape`

支持能力：

- 按 ID 抓取元数据
- 按名称搜索并返回最佳匹配
- 抓取封面 URL
- 抓取开发商/社团、简介、发售日、标签
- 写入 `games.source_type` 与 `games.source_id`
- 写入 `game_tags`

第一版建议只实现元数据、封面和 tags，不扩展背景图、logo、截图等媒体能力。

## 数据结构映射

LunaBox 当前核心模型是 `models.Game` 和 `metadata.TagItem`。

### Game 字段

| LunaBox 字段 | DLsite | ErogameScape |
|---|---|---|
| `Name` | 作品名 `#work_name` | `div#soft-title > span.bold` |
| `CoverURL` | 主图 `img_main` | `div#main_image img` |
| `Company` | `.maker_name a` | `tr#brand > td` |
| `Summary` | `[itemprop="description"]` | 优先空；可后续从 FANZA/Getchu 补充 |
| `Rating` | 第一版可为 `0` | 可解析评分后归一到 10 分 |
| `ReleaseDate` | 发售日表格项 | `tr#sellday > td` |
| `SourceType` | `enums.DLsite` | `enums.ErogameScape` |
| `SourceID` | `RJxxxxxx` / `RExxxxxx` / `VJxxxxxx` | 批评空间数字 ID |
| `CachedAt` | `time.Now()` | `time.Now()` |

### Tag 字段

| TagItem 字段 | 规则 |
---|---|
| `Name` | 标签原文 |
| `Source` | `dlsite` 或 `erogamescape` |
| `Weight` | 第一版可统一 `1.0`，或按顺序递减 |
| `IsSpoiler` | 第一版统一 `false` |

## 后端改动

### 1. 新增 source enum

修改：

`internal/common/enums/source_enum.go`

新增：

```go
DLsite        SourceType = "dlsite"
ErogameScape SourceType = "erogamescape"
```

并同步 `AllSourceTypes`：

```go
{DLsite, "DLSITE"},
{ErogameScape, "EROGAMESCAPE"},
```

完成后需要运行：

```powershell
wails generate module
```

这样前端 `wailsjs/go/models.ts` 会生成新的 `enums.SourceType` 成员。

### 2. 扩展配置白名单

修改：

`internal/appconf/config.go`

在 `allowedMetadataSourceSet` 中增加：

```go
string(enums2.DLsite):        {},
string(enums2.ErogameScape): {},
```

建议第一版不要加入 `defaultMetadataSources`。

原因：

- 两个源都是 HTML 抓取，稳定性弱于 API。
- 默认启用会增加全局搜索耗时。
- DLsite 和批评空间都偏成人向数据源，应让用户显式启用。

### 3. 新增 DLsite Getter

新增文件：

`internal/utils/metadata/metadata_dlsite.go`

建议结构：

```go
type DLsiteInfoGetter struct {
	client *http.Client
}

func NewDLsiteInfoGetter() *DLsiteInfoGetter {
	return &DLsiteInfoGetter{client: newMetadataClient()}
}

var _ Getter = (*DLsiteInfoGetter)(nil)

func (d DLsiteInfoGetter) FetchMetadata(id string, token string) (MetadataResult, error) {
	// normalize id -> fetch work page -> parse metadata
}

func (d DLsiteInfoGetter) FetchMetadataByName(name string, token string) (MetadataResult, error) {
	// search -> pick first/best result -> FetchMetadata(result.ID, "")
}
```

关键实现：

- ID 正则：`(?i)\b(RJ|RE|VJ)\d{4,}\b`
- `VJ` 使用 `https://www.dlsite.com/pro/work/=/product_id/{id}.html`
- `RJ/RE` 使用 `https://www.dlsite.com/maniax/work/=/product_id/{id}.html`
- 搜索使用 `https://www.dlsite.com/maniax/fsr/=/language/jp/keyword/{query}/`
- 请求头包含：

```http
User-Agent: LunaBox metadata user agent
Cookie: adultchecked=1; locale=ja
Accept-Language: ja,en;q=0.8
```

解析建议：

- 使用 `goquery` 解析 HTML。
- `#work_name` 取作品名。
- `.maker_name a` 取社团。
- `[itemprop="description"]` 取简介。
- `.main_genre a` 和作品形式字段取 tags。
- `data-src` 中包含 `_img_main` 或 `_img_smp` 的图片可作为封面候选。

如果不想引入依赖，也可以用 `golang.org/x/net/html` 自行遍历，但实现成本更高。

### 4. 新增 ErogameScape Getter

新增文件：

`internal/utils/metadata/metadata_erogamescape.go`

建议结构：

```go
type ErogameScapeInfoGetter struct {
	client *http.Client
}

func NewErogameScapeInfoGetter() *ErogameScapeInfoGetter {
	return &ErogameScapeInfoGetter{client: newMetadataClient()}
}

var _ Getter = (*ErogameScapeInfoGetter)(nil)

func (e ErogameScapeInfoGetter) FetchMetadata(id string, token string) (MetadataResult, error) {
	// normalize id -> fetch game.php -> parse metadata
}

func (e ErogameScapeInfoGetter) FetchMetadataByName(name string, token string) (MetadataResult, error) {
	// search kensaku.php -> pick first/best result -> FetchMetadata(result.ID, "")
}
```

关键 URL：

```text
https://erogamescape.org/~ap2/ero/toukei_kaiseki
```

搜索：

```text
/kensaku.php?category=game&word_category=name&mode=normal&word={query}
```

详情：

```text
/game.php?game={id}
```

解析建议：

- 搜索结果解析 `#result tr`。
- 表头根据日文列名定位：
  - `ゲーム名`
  - `ブランド名`
  - `発売日`
- 详情页解析：
  - 标题：`div#soft-title > span.bold`
  - 品牌：`tr#brand > td`
  - 发售日：`tr#sellday > td`
  - 封面：`div#main_image img`
  - 标签：`table#att_pov_table`

第一版可以不从 FANZA/Getchu 补简介。ErogameScape 自身适合提供结构化标签、品牌、评分和发售日。

### 5. 接入 GameService

修改：

`internal/service/game_service.go`

#### `fetchMetadataResultByRequest`

增加：

```go
case enums2.DLsite:
	normalizedID, ok := normalizeDLsiteID(sourceID)
	if !ok {
		return metadata.MetadataResult{}, fmt.Errorf("invalid DLsite ID format: %s", req.ID)
	}
	return s.fetchMetadataResultBySource(req.Source, normalizedID)

case enums2.ErogameScape:
	normalizedID, ok := normalizeErogameScapeID(sourceID)
	if !ok {
		return metadata.MetadataResult{}, fmt.Errorf("invalid ErogameScape ID format: %s", req.ID)
	}
	return s.fetchMetadataResultBySource(req.Source, normalizedID)
```

#### `fetchMetadataResultBySource`

增加：

```go
case enums2.DLsite:
	getter := metadata.NewDLsiteInfoGetter()
	return getter.FetchMetadata(sourceID, "")
case enums2.ErogameScape:
	getter := metadata.NewErogameScapeInfoGetter()
	return getter.FetchMetadata(sourceID, "")
```

#### `getConfiguredMetadataSearchSources`

增加：

```go
case enums2.DLsite:
	sources = append(sources, metadataSearchSource{
		source: enums2.DLsite,
		fetchByName: func(name string) (metadata.MetadataResult, error) {
			return metadata.NewDLsiteInfoGetter().FetchMetadataByName(name, "")
		},
	})
case enums2.ErogameScape:
	sources = append(sources, metadataSearchSource{
		source: enums2.ErogameScape,
		fetchByName: func(name string) (metadata.MetadataResult, error) {
			return metadata.NewErogameScapeInfoGetter().FetchMetadataByName(name, "")
		},
	})
```

#### `getConfiguredMetadataSources`

允许配置读取：

```go
case enums2.Bangumi, enums2.VNDB, enums2.Ymgal, enums2.Steam, enums2.DLsite, enums2.ErogameScape:
```

## 前端改动

### 1. 元数据设置页

修改：

`frontend/src/components/panel/MetadataSettingsPanel.tsx`

增加 source：

```ts
const DEFAULT_METADATA_SOURCES = [
  "bangumi",
  "vndb",
  "ymgal",
  "steam",
  "dlsite",
  "erogamescape",
];
```

如果后端不默认启用两者，前端默认值也应保持旧四项，只把两项加入合法集合和 `sourceItems`。

建议：

- `DEFAULT_METADATA_SOURCES` 仍保持旧四项。
- 单独增加 `VALID_METADATA_SOURCES`。

新增 UI 项：

```ts
{
  value: "dlsite",
  label: "DLsite",
  hint: t("settings.metadata.sourceHints.dlsite"),
  icon: "/dlsite-logo.png",
},
{
  value: "erogamescape",
  label: "ErogameScape",
  hint: t("settings.metadata.sourceHints.erogamescape"),
  icon: "/erogamescape-logo.png",
},
```

如暂时没有图标，可以先用文字或现有通用图标，避免为了图标阻塞功能。

### 2. 添加游戏弹窗

修改：

`frontend/src/components/modal/AddGameModal.tsx`

手动按 ID 搜索的数据源下拉增加：

```ts
{ value: enums.SourceType.DLSITE, label: "DLsite" },
{ value: enums.SourceType.EROGAMESCAPE, label: "ErogameScape" },
```

### 3. 批量导入弹窗

修改：

- `frontend/src/components/modal/BatchImportModal.tsx`
- `frontend/src/components/modal/DragDropImportModal.tsx`

手动选择项增加 DLsite / ErogameScape。

自动匹配优先级建议：

```ts
const priorityOrder = [
  enums.SourceType.BANGUMI,
  enums.SourceType.VNDB,
  enums.SourceType.YMGAL,
  enums.SourceType.DLSITE,
  enums.SourceType.EROGAMESCAPE,
  enums.SourceType.STEAM,
];
```

如果你的主要用户是日文成人向游戏，可以把 `DLSITE` 和 `EROGAMESCAPE` 提到 `YMGAL` 前后。

### 4. 文案

修改：

- `frontend/src/locales/zh-CN.json`
- `frontend/src/locales/en-US.json`
- `frontend/src/locales/ja-JP.json`

建议文案：

```json
{
  "settings": {
    "metadata": {
      "sourceHints": {
        "dlsite": "适合 DLsite 上架作品，支持 RJ/RE/VJ 作品 ID。",
        "erogamescape": "适合日文 Galgame 条目，可提供品牌、发售日、评分和标签。"
      }
    }
  }
}
```

## ID 规范化建议

### DLsite

支持输入：

- `RJ123456`
- `RE123456`
- `VJ123456`
- `https://www.dlsite.com/.../product_id/RJ123456.html`

输出统一大写。

### ErogameScape

支持输入：

- `12345`
- `game.php?game=12345`
- `https://erogamescape.org/~ap2/ero/toukei_kaiseki/game.php?game=12345`

输出纯数字字符串。

## 错误处理

建议错误信息：

- ID 为空：`metadata id is empty`
- DLsite ID 不合法：`invalid DLsite ID format: {id}`
- ErogameScape ID 不合法：`invalid ErogameScape ID format: {id}`
- 搜索无结果：`no results found`
- HTTP 非 200：包含 status 和部分 body，便于定位反爬或页面变化

抓取失败时应返回 error，不要返回空 `models.Game`，否则前端难以区分“没找到”和“解析失败”。

## 测试建议

至少添加 unit tests：

- DLsite ID 规范化
- ErogameScape ID 规范化
- 日期归一化
- HTML fixture 解析
- `getConfiguredMetadataSources` 能接受新 source

不建议在单元测试中直接请求真实网站。把真实 HTML 存成 fixture，测试 parser。

## 验证命令

后端：

```powershell
go test ./internal/utils/metadata ./internal/service/...
go build -tags dev
```

生成 Wails 绑定：

```powershell
wails generate module
```

前端：

```powershell
cd frontend
pnpm build
```

## 推荐实施顺序

1. 加 enum 和配置白名单。
2. 实现两个 getter 的 ID 规范化和 HTML parser。
3. 接入 `GameService`。
4. 跑后端测试和 `go build -tags dev`。
5. 跑 `wails generate module`。
6. 接前端 source 选项和文案。
7. 跑 `pnpm build`。

这样每一步都能独立验证，问题容易定位。
