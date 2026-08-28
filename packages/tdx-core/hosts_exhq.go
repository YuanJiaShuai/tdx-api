package tdx

import (
	"net"
	"strings"
	"sync"
	"time"

	"github.com/injoyai/base/safe"
	"github.com/injoyai/conv"
	"github.com/injoyai/logs"
)

// 扩展行情(TdxExHq)服务器,端口 7727,地址取自通达信 connect.cfg [DSHOST] 段。
// 编写方式参考主行情 hosts.go：按地域拆分子表,IP 不含端口(连接时补 :7727)。
const ExPort = "7727"

var (

	// ExHosts 扩展行情所有服务器地址(connect.cfg [DSHOST])
	// 广州段在前:2026-06 实测仅 116.205.143.214(广州双线1)完全可用,置首位使
	// range-dial 默认优先命中;其余多为连接超时/受限节点,可用 SortExHosts 重排。
	ExHosts = func() []string {
		lenGZ := len(ExGZHosts)
		lenSH := len(ExSHHosts)
		lenBJ := len(ExBJHosts)

		ls := make([]string, lenGZ+lenSH+lenBJ)
		copy(ls[:lenGZ], ExGZHosts)
		copy(ls[lenGZ:lenGZ+lenSH], ExSHHosts)
		copy(ls[lenGZ+lenSH:lenGZ+lenSH+lenBJ], ExBJHosts)
		return ls
	}()

	// ExSHHosts 扩展行情上海服务器地址
	ExSHHosts = []string{
		"175.24.47.69",   //扩展市场上海双线7
		"150.158.9.199",  //扩展市场上海双线1
		"150.158.20.127", //扩展市场上海双线2
		"49.235.119.116", //扩展市场上海双线3
		"49.234.13.160",  //扩展市场上海双线4
		"123.60.173.210", //扩展市场上海双线5
		"118.89.69.202",  //扩展市场上海双线6
	}

	// ExBJHosts 扩展行情北京服务器地址
	ExBJHosts = []string{
		"112.74.214.43",  //扩展市场北京双线1
		"120.25.218.6",   //扩展市场北京双线2
		"43.139.173.246", //扩展市场北京双线3
		"159.75.90.107",  //扩展市场北京双线4
		"106.52.170.195", //扩展市场北京双线5
		"139.9.191.175",  //扩展市场北京双线6
	}

	// ExGZHosts 扩展行情广州服务器地址
	ExGZHosts = []string{
		"116.205.143.214", //扩展市场广州双线1
		"124.71.223.19",   //扩展市场广州双线2
		"113.45.175.47",   //扩展市场广州双线4
	}
)

// SortExHosts 通过tcp连接速度筛选排序可用的扩展行情地址(同 SortHosts,端口 7727)
func SortExHosts(timeout ...time.Duration) []string {

	//超时时间
	_timeout := conv.Default(time.Second, timeout...)

	//至少需要一个
	chMustOne := safe.NewCloser()

	mu := sync.Mutex{}
	ls := []string(nil)
	for _, host := range ExHosts {
		go func(host string) {
			addr := host
			if !strings.Contains(addr, ":") {
				addr += ":" + ExPort
			}

			now := time.Now()
			c, err := net.Dial("tcp", addr)
			if err != nil {
				logs.Err(err)
				return
			}
			c.Close()

			logs.Debugf("ExHost: %-15s  Speed: %s\n", host, time.Since(now))
			mu.Lock()
			ls = append(ls, host)
			mu.Unlock()
			chMustOne.Close()
		}(host)
	}

	<-time.After(_timeout)
	<-chMustOne.Done()

	ExHosts = ls

	return ls
}
