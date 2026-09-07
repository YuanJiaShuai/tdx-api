package protocol

import "testing"

func TestParseSpBlock(t *testing.T) {
	// ASCII 名称: GBK 解码对 ASCII 为恒等, 便于离线构造。CRLF 分行, 混入空行/短行以验证过滤。
	raw := []byte("#IDX2000\r\n0000011\r\n0000014\r\n1600000\r\n2920001\r\n" +
		"\r\n#IDX500\r\n0000009\r\n123\r\nabcdefg\r\n1600519\r\n")

	blocks := ParseSpBlock(raw)
	if len(blocks) != 2 {
		t.Fatalf("板块数 = %d, 期望 2", len(blocks))
	}

	if blocks[0].Name != "IDX2000" {
		t.Errorf("blocks[0].Name = %q, 期望 IDX2000", blocks[0].Name)
	}
	if got := len(blocks[0].Codes); got != 4 {
		t.Errorf("IDX2000 成分数 = %d, 期望 4", got)
	}
	if blocks[0].Codes[0] != "0000011" || blocks[0].Codes[3] != "2920001" {
		t.Errorf("IDX2000 成分内容异常: %v", blocks[0].Codes)
	}

	// "123"(短) 与 "abcdefg"(非数字) 应被过滤, 只剩 2 条。
	if got := len(blocks[1].Codes); got != 2 {
		t.Errorf("IDX500 成分数 = %d, 期望 2(过滤短行/非数字行)", got)
	}
	if b := SpBlockByName(blocks, "IDX500"); b == nil || b.Codes[1] != "1600519" {
		t.Errorf("SpBlockByName 结果异常: %+v", b)
	}
	if SpBlockByName(blocks, "不存在") != nil {
		t.Error("SpBlockByName 未命中应返回 nil")
	}
}
