package extend

import (
	"context"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/injoyai/logs"
	"github.com/injoyai/tdx"
	"github.com/injoyai/tdx/protocol"
	"xorm.io/core"
	"xorm.io/xorm"
)

const (
	// K线类型(补充新版 pull-kline.go 中已移除的历史类型)
	Minute5  = "5minute"
	Minute15 = "15minute"
	Minute30 = "30minute"
	Hour     = "hour"
	Week     = "week"
	Month    = "month"
	Quarter  = "quarter"
	Year     = "year"

	// K线类型对应的MySQL表名
	tableMinute   = "MinuteKline"
	table5Minute  = "Minute5Kline"
	table15Minute = "Minute15Kline"
	table30Minute = "Minute30Kline"
	tableHour     = "HourKline"
	tableDay      = "DayKline"
	tableWeek     = "WeekKline"
	tableMonth    = "MonthKline"
	tableQuarter  = "QuarterKline"
	tableYear     = "YearKline"
)

// KlineHandler 拉取K线的方法签名
type KlineHandler func(code string, f func(k *protocol.Kline) bool) (*protocol.KlineResp, error)

// NewKlineTable 构造K线表配置(表名 + 拉取方法)
func NewKlineTable(tableName string, handler func(c *tdx.Client) KlineHandler) *KlineTable {
	return &KlineTable{tableName: tableName, handler: handler}
}

// KlineTable K线表配置
type KlineTable struct {
	tableName string
	handler   func(c *tdx.Client) KlineHandler
}

// TableName 获取表名
func (this *KlineTable) TableName() string { return this.tableName }

// Handler 获取K线拉取方法
func (this *KlineTable) Handler(c *tdx.Client) KlineHandler { return this.handler(c) }

// KlineTableMap 所有K线类型对应的表配置
var KlineTableMap = map[string]*KlineTable{
	Minute:   NewKlineTable(tableMinute, func(c *tdx.Client) KlineHandler { return c.GetKlineMinuteUntil }),
	Minute5:  NewKlineTable(table5Minute, func(c *tdx.Client) KlineHandler { return c.GetKline5MinuteUntil }),
	Minute15: NewKlineTable(table15Minute, func(c *tdx.Client) KlineHandler { return c.GetKline15MinuteUntil }),
	Minute30: NewKlineTable(table30Minute, func(c *tdx.Client) KlineHandler { return c.GetKline30MinuteUntil }),
	Hour:     NewKlineTable(tableHour, func(c *tdx.Client) KlineHandler { return c.GetKlineHourUntil }),
	Day:      NewKlineTable(tableDay, func(c *tdx.Client) KlineHandler { return c.GetKlineDayUntil }),
	Week:     NewKlineTable(tableWeek, func(c *tdx.Client) KlineHandler { return c.GetKlineWeekUntil }),
	Month:    NewKlineTable(tableMonth, func(c *tdx.Client) KlineHandler { return c.GetKlineMonthUntil }),
	Quarter:  NewKlineTable(tableQuarter, func(c *tdx.Client) KlineHandler { return c.GetKlineQuarterUntil }),
	Year:     NewKlineTable(tableYear, func(c *tdx.Client) KlineHandler { return c.GetKlineYearUntil }),
}

// KlineMysql MySQL存储的K线结构(带代码字段,单表存多股票)
type KlineMysql struct {
	Code       string         `json:"code" xorm:"index"`     //代码
	Unix       int64          `json:"unix" xorm:"index"`     //时间戳
	Time       time.Time      `json:"time"`                  //时间
	Last       protocol.Price `json:"last"`                  //昨日收盘价
	Open       protocol.Price `json:"open"`                  //开盘价
	High       protocol.Price `json:"high"`                  //最高价
	Low        protocol.Price `json:"low"`                   //最低价
	Close      protocol.Price `json:"close"`                 //收盘价
	Order      int            `json:"order"`                 //成交单数
	Volume     int64          `json:"volume"`                //成交量(手)
	Amount     protocol.Price `json:"amount"`                //成交额
	UpCount    int            `json:"upCount"`               //上涨数量(指数有效)
	DownCount  int            `json:"downCount"`             //下跌数量(指数有效)
	Turnover   float64        `json:"turnover"`              //换手率
	FloatStock int64          `json:"floatStock"`            //流通股本
	TotalStock int64          `json:"totalStock"`            //总股本
	InDate     int64          `json:"inDate" xorm:"created"` //创建时间
}

// NewPullKlineMysql 拉取K线数据保存到MySQL(本地扩展)
// cfg.Dir 为MySQL DSN,cfg.Types 为需要拉取的K线类型,默认day
func NewPullKlineMysql(cfg PullKlineConfig) (*PullKlineMysql, error) {
	db, err := xorm.NewEngine("mysql", cfg.Dir)
	if err != nil {
		return nil, err
	}
	db.SetMapper(core.SameMapper{})
	types := cfg.Types
	if len(types) == 0 {
		types = []string{Day}
	}
	for _, v := range types {
		table := KlineTableMap[v]
		if table == nil {
			continue
		}
		if err = db.Table(table.TableName()).Sync2(new(KlineMysql)); err != nil {
			return nil, err
		}
	}
	return &PullKlineMysql{
		Config: cfg,
		Types:  types,
		DB:     db,
	}, nil
}

// PullKlineMysql MySQL版K线拉取器(本地扩展)
type PullKlineMysql struct {
	Config PullKlineConfig
	Types  []string
	DB     *xorm.Engine
}

// Name 任务名称
func (this *PullKlineMysql) Name() string {
	return "拉取k线数据"
}

// Run 执行拉取
func (this *PullKlineMysql) Run(ctx context.Context, m *tdx.Manage) error {
	goroutines := this.Config.Goroutines
	if goroutines <= 0 {
		goroutines = 10
	}
	sem := make(chan struct{}, goroutines)
	var wg sync.WaitGroup

	//1. 获取所有股票代码
	codes := this.Config.Codes
	if len(codes) == 0 {
		codes = m.Codes.GetStockCodes()
	}

	for _, code := range codes {
		select {
		case <-ctx.Done():
			wg.Wait()
			return ctx.Err()
		default:
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(code string) {
			defer wg.Done()
			defer func() { <-sem }()

			for _, typ := range this.Types {
				select {
				case <-ctx.Done():
					return
				default:
				}
				table := KlineTableMap[typ]
				if table == nil {
					continue
				}
				if err := this.pullOne(m, table, code); err != nil {
					logs.Err(err)
					return
				}
			}
		}(code)
	}
	wg.Wait()
	return nil
}

// pullOne 拉取单个代码单个类型的K线
func (this *PullKlineMysql) pullOne(m *tdx.Manage, table *KlineTable, code string) error {
	//2. 获取最后一条数据
	last := new(KlineMysql)
	if _, err := this.DB.Table(table.TableName()).Where("Code=?", code).Desc("Unix").Get(last); err != nil {
		return err
	}

	lastUnix := last.Unix
	if lastUnix == 0 {
		lastUnix = protocol.ExchangeEstablish.Unix()
	}
	if start := this.Config.StartAt.Unix(); start > lastUnix {
		lastUnix = start
	}

	//3. 从服务器获取增量数据
	insert := []*KlineMysql(nil)
	err := m.Do(func(c *tdx.Client) error {
		resp, err := table.Handler(c)(code, func(k *protocol.Kline) bool {
			return k.Time.Unix() <= lastUnix
		})
		if err != nil {
			return err
		}
		for _, v := range resp.List {
			insert = append(insert, &KlineMysql{
				Code:      code,
				Unix:      v.Time.Unix(),
				Time:      v.Time,
				Last:      v.Last,
				Open:      v.Open,
				High:      v.High,
				Low:       v.Low,
				Close:     v.Close,
				Order:     v.Order,
				Volume:    v.Volume,
				Amount:    v.Amount,
				UpCount:   v.UpCount,
				DownCount: v.DownCount,
			})
		}
		return nil
	})
	if err != nil {
		return err
	}

	//4. 插入数据库
	if len(insert) == 0 {
		return nil
	}
	if _, err := this.DB.Table(table.TableName()).Where("Code=? and Unix >= ?", code, insert[0].Unix).Delete(new(KlineMysql)); err != nil {
		return err
	}
	for _, v := range insert {
		if _, err := this.DB.Table(table.TableName()).Insert(v); err != nil {
			return err
		}
	}
	return nil
}
