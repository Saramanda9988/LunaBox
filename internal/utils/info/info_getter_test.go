package info

import (
	"os"
	"testing"
)

const token = "7SeA8edOkADflUeBYpMnSsTidz36hR0PskAFyN8W"

// TestBangumiInfoGetter_FetchMetadata 测试 Bangumi ID 查询
// 需要设置环境变量 BANGUMI_TOKEN 才能运行
func TestBangumiInfoGetter_FetchMetadata(t *testing.T) {
	getter := NewBangumiInfoGetter()

	// 使用一个真实的 Bangumi subject ID 进行测试
	// 这里使用 243475 作为示例（可以替换为其他有效的 ID）
	testID := "243475"

	t.Run("成功获取游戏信息", func(t *testing.T) {
		game, err := getter.FetchMetadata(testID, token)
		if err != nil {
			t.Fatalf("获取元数据失败: %v", err)
		}

		if game.Name == "" {
			t.Error("游戏名称为空")
		}
		if game.SourceID != testID {
			t.Errorf("SourceID 不匹配: 期望 %s, 得到 %s", testID, game.SourceID)
		}

		// 打印结果以便查看
		t.Logf("游戏名称: %s", game.Name)
		t.Logf("封面URL: %s", game.CoverURL)
		t.Logf("开发商: %s", game.Company)
		t.Logf("简介长度: %d 字符", len(game.Summary))
	})

	t.Run("Token为空应返回错误", func(t *testing.T) {
		_, err := getter.FetchMetadata(testID, "")
		if err == nil {
			t.Error("期望返回错误，但没有错误")
		}
		if err.Error() != "bangumi API requires Bearer token" {
			t.Errorf("错误信息不匹配: %v", err)
		}
	})

	t.Run("无效ID应返回错误", func(t *testing.T) {
		_, err := getter.FetchMetadata("invalid_id", token)
		if err == nil {
			t.Error("期望返回错误，但没有错误")
		}
		t.Logf("预期的错误: %v", err)
	})
}

// TestBangumiInfoGetter_FetchMetadataByName 测试通过名称查询
func TestBangumiInfoGetter_FetchMetadataByName(t *testing.T) {
	token := os.Getenv("BANGUMI_TOKEN")
	if token == "" {
		t.Skip("跳过测试: 请设置环境变量 BANGUMI_TOKEN")
	}

	getter := NewBangumiInfoGetter()

	t.Run("应返回未实现错误", func(t *testing.T) {
		_, err := getter.FetchMetadataByName("测试游戏", token)
		if err == nil {
			t.Error("期望返回错误，但没有错误")
		}
		if err.Error() != "search by name is not implemented for Bangumi yet" {
			t.Errorf("错误信息不匹配: %v", err)
		}
	})
}

// TestVNDBInfoGetter_FetchMetadata 测试 VNDB ID 查询
func TestVNDBInfoGetter_FetchMetadata(t *testing.T) {
	getter := NewVNDBInfoGetter()

	// 使用一个真实的 VNDB ID 进行测试
	// 这里使用 v17 (Clannad) 作为示例
	testID := "v17"

	t.Run("成功获取游戏信息", func(t *testing.T) {
		game, err := getter.FetchMetadata(testID, "")
		if err != nil {
			t.Fatalf("获取元数据失败: %v", err)
		}

		if game.Name == "" {
			t.Error("游戏名称为空")
		}
		if game.SourceID != testID {
			t.Errorf("SourceID 不匹配: 期望 %s, 得到 %s", testID, game.SourceID)
		}

		// 打印结果以便查看
		t.Logf("游戏名称: %s", game.Name)
		t.Logf("封面URL: %s", game.CoverURL)
		t.Logf("开发商: %s", game.Company)
		t.Logf("简介长度: %d 字符", len(game.Summary))
	})

	t.Run("无效ID应返回错误或无结果", func(t *testing.T) {
		_, err := getter.FetchMetadata("v999999999", "")
		if err == nil {
			t.Error("期望返回错误，但没有错误")
		}
		t.Logf("预期的错误: %v", err)
	})
}

// TestVNDBInfoGetter_FetchMetadataByName 测试 VNDB 名称查询
func TestVNDBInfoGetter_FetchMetadataByName(t *testing.T) {
	getter := NewVNDBInfoGetter()

	t.Run("成功通过名称查询", func(t *testing.T) {
		game, err := getter.FetchMetadataByName("Clannad", "")
		if err != nil {
			t.Fatalf("获取元数据失败: %v", err)
		}

		if game.Name == "" {
			t.Error("游戏名称为空")
		}

		// 打印结果以便查看
		t.Logf("游戏名称: %s", game.Name)
		t.Logf("Source ID: %s", game.SourceID)
		t.Logf("开发商: %s", game.Company)
	})

	t.Run("不存在的游戏名称应返回错误", func(t *testing.T) {
		_, err := getter.FetchMetadataByName("这个游戏绝对不存在的超级长名字12345", "")
		if err == nil {
			t.Error("期望返回错误，但没有错误")
		}
		t.Logf("预期的错误: %v", err)
	})
}

// TestGetterInterface 测试两个实现都遵循 Getter 接口
func TestGetterInterface(t *testing.T) {
	var _ Getter = (*BangumiInfoGetter)(nil)
	var _ Getter = (*VNDBInfoGetter)(nil)
	t.Log("两个 InfoGetter 都正确实现了 Getter 接口")
}
