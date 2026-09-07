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

var (

	// Hosts 所有服务器地址(2024-11-30测试通过)
	Hosts = func() []string {
		lenSH := len(SHHosts)
		lenBJ := len(BJHosts)
		lenGZ := len(GZHosts)
		lenWH := len(WHHosts)

		ls := make([]string, lenSH+lenBJ+lenGZ+lenWH)
		copy(ls[:lenSH], SHHosts)
		copy(ls[lenSH:lenSH+lenBJ], BJHosts)
		copy(ls[lenSH+lenBJ:lenSH+lenBJ+lenGZ], GZHosts)
		copy(ls[lenSH+lenBJ+lenGZ:lenSH+lenBJ+lenGZ+lenWH], WHHosts)
		return ls
	}()

	// SHHosts 上海服务器地址
	SHHosts = []string{
		"124.71.187.122",  //华为
		"122.51.120.217",  //腾讯
		"111.229.247.189", //腾讯
		"122.51.232.182",  //腾讯
		"118.25.98.114",   //腾讯
		"124.70.199.56",   //华为
		"121.36.225.169",  //华为
		"123.60.70.228",   //华为
		"123.60.73.44",    //华为
		"124.70.133.119",  //华为
		"124.71.187.72",   //华为
		"123.60.84.66",    //华为
		"124.223.163.242", //腾讯云
		"150.158.160.2",   //腾讯云
		"101.35.121.35",   //腾讯云
		"111.231.113.208", //腾讯云
	}

	// BJHosts 北京服务器地址
	BJHosts = []string{
		"62.234.50.143",   //腾讯云
		"81.70.151.186",   //腾讯云
		"101.42.240.54",   //腾讯云
		"101.43.159.194",  //腾讯云
		"120.53.8.251",    //腾讯云
		"152.136.191.169", //腾讯云
		"49.232.15.141",   //腾讯云
		"82.156.174.84",   //腾讯云
		"101.42.164.241",  //腾讯云

	}

	// GZHosts 广州服务器地址,客户端上可能显示深圳
	GZHosts = []string{
		"124.71.9.153",    //华为
		"116.205.163.254", //华为
		"116.205.171.132", //华为
		"116.205.183.150", //华为
		"111.230.186.52",  //腾讯
		"110.41.2.72",     //华为
		"110.41.147.114",  //华为,这个客户端显示深圳线路1,IP查询是广州的
		"101.33.225.16",   //腾讯云
		"175.178.112.197", //腾讯云
		"175.178.128.227", //腾讯云
		"43.139.95.83",    //腾讯云
		"159.75.29.111",   //腾讯云
		"43.139.18.171",   //腾讯云
		"81.71.32.47",     //腾讯云
		"129.204.230.128", //腾讯云
	}

	// WHHosts 武汉服务器地址
	WHHosts = []string{
		"119.97.185.59", //电信
	}
)

// SortHosts 通过tcp(ping不可用)连接速度的方式筛选排序可用的地址
func SortHosts(timeout ...time.Duration) []string {

	//超时时间
	_timeout := conv.Default(time.Second, timeout...)

	//至少需要一个
	chMustOne := safe.NewCloser()

	mu := sync.Mutex{}
	ls := []string(nil)
	for _, host := range Hosts {
		go func(host string) {
			addr := host
			if !strings.Contains(addr, ":") {
				addr += ":7709"
			}

			now := time.Now()
			c, err := net.Dial("tcp", addr)
			if err != nil {
				logs.Err(err)
				return
			}
			c.Close()

			logs.Debugf("Host: %-15s  Speed: %s\n", host, time.Since(now))
			mu.Lock()
			ls = append(ls, host)
			mu.Unlock()
			chMustOne.Close()
		}(host)
	}

	<-time.After(_timeout)
	<-chMustOne.Done()

	Hosts = ls

	return ls
}
