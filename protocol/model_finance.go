package protocol

import (
	"encoding/binary"
	"errors"
	"math"
)

// TypeFinance 财务信息（基本面：流通股本/总股本/行业/地域/股东户数/财务指标）。参考 pytdx GetFinanceInfo。
const TypeFinance uint16 = 0x0010

// FinanceInfo 单只标的财务/基本面信息。股本与资产类字段单位为「股/元」（已 ×10000）。
type FinanceInfo struct {
	Market             uint8
	Code               string
	LiuTongGuBen       float64 // 流通股本
	Province           uint16  // 地域码
	Industry           uint16  // 行业码
	UpdatedDate        uint32  // 更新日期
	IPODate            uint32  // 上市日期
	ZongGuBen          float64 // 总股本
	GuoJiaGu           float64 // 国家股
	FaQiRenFaRenGu     float64
	FaRenGu            float64
	BGu                float64
	HGu                float64
	ZhiGongGu          float64
	ZongZiChan         float64 // 总资产
	LiuDongZiChan      float64 // 流动资产
	GuDingZiChan       float64 // 固定资产
	WuXingZiChan       float64 // 无形资产
	GuDongRenShu       float64 // 股东户数
	LiuDongFuZhai      float64 // 流动负债
	ChangQiFuZhai      float64 // 长期负债
	ZiBenGongJiJin     float64 // 资本公积金
	JingZiChan         float64 // 净资产
	ZhuYingShouRu      float64 // 主营收入
	ZhuYingLiRun       float64 // 主营利润
	YingShouZhangKuan  float64 // 应收账款
	YingYeLiRun        float64 // 营业利润
	TouZiShouYi        float64 // 投资收益
	JingYingXianJinLiu float64 // 经营现金流
	ZongXianJinLiu     float64 // 总现金流
	CunHuo             float64 // 存货
	LiRunZongHe        float64 // 利润总额
	ShuiHouLiRun       float64 // 税后利润
	JingLiRun          float64 // 净利润
	WeiFenLiRun        float64 // 未分配利润
	BaoLiu1            float64
	BaoLiu2            float64
}

type finance struct{}

// MFinance 财务信息协议单例。
var MFinance finance

// Frame 构造财务信息请求帧。market: 0深 1沪 2京（= Exchange.Uint8()）。
func (finance) Frame(market uint8, code string) *Frame {
	data := make([]byte, 9) // 01 00 + market(1) + code(6)
	data[0] = 0x01
	data[1] = 0x00
	data[2] = market
	copy(data[3:9], code)
	return &Frame{Control: Control01, Type: TypeFinance, Data: data}
}

func f32(bs []byte) float64 { return float64(math.Float32frombits(binary.LittleEndian.Uint32(bs))) }

// Decode 解析财务信息响应。
func (finance) Decode(bs []byte) (*FinanceInfo, error) {
	if len(bs) < 145 {
		return nil, errors.New("finance 数据长度不足")
	}
	pos := 2 // 跳过 num
	fi := &FinanceInfo{
		Market: bs[pos],
		Code:   string(bs[pos+1 : pos+7]),
	}
	pos += 7
	const w = 10000.0
	fi.LiuTongGuBen = f32(bs[pos:pos+4]) * w
	fi.Province = binary.LittleEndian.Uint16(bs[pos+4 : pos+6])
	fi.Industry = binary.LittleEndian.Uint16(bs[pos+6 : pos+8])
	fi.UpdatedDate = binary.LittleEndian.Uint32(bs[pos+8 : pos+12])
	fi.IPODate = binary.LittleEndian.Uint32(bs[pos+12 : pos+16])
	pos += 16
	// 后续 30 个 float32。
	flt := func(i int) float64 { return f32(bs[pos+i*4 : pos+i*4+4]) }
	fi.ZongGuBen = flt(0) * w
	fi.GuoJiaGu = flt(1) * w
	fi.FaQiRenFaRenGu = flt(2) * w
	fi.FaRenGu = flt(3) * w
	fi.BGu = flt(4) * w
	fi.HGu = flt(5) * w
	fi.ZhiGongGu = flt(6) * w
	fi.ZongZiChan = flt(7) * w
	fi.LiuDongZiChan = flt(8) * w
	fi.GuDingZiChan = flt(9) * w
	fi.WuXingZiChan = flt(10) * w
	fi.GuDongRenShu = flt(11) // 股东户数不 ×10000
	fi.LiuDongFuZhai = flt(12) * w
	fi.ChangQiFuZhai = flt(13) * w
	fi.ZiBenGongJiJin = flt(14) * w
	fi.JingZiChan = flt(15) * w
	fi.ZhuYingShouRu = flt(16) * w
	fi.ZhuYingLiRun = flt(17) * w
	fi.YingShouZhangKuan = flt(18) * w
	fi.YingYeLiRun = flt(19) * w
	fi.TouZiShouYi = flt(20) * w
	fi.JingYingXianJinLiu = flt(21) * w
	fi.ZongXianJinLiu = flt(22) * w
	fi.CunHuo = flt(23) * w
	fi.LiRunZongHe = flt(24) * w
	fi.ShuiHouLiRun = flt(25) * w
	fi.JingLiRun = flt(26) * w
	fi.WeiFenLiRun = flt(27) * w
	fi.BaoLiu1 = flt(28)
	fi.BaoLiu2 = flt(29)
	return fi, nil
}
