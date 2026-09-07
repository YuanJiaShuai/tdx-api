// Package reader provides Go equivalents of pytdx's local file readers.
package reader

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/injoyai/tdx/extend"
	"github.com/injoyai/tdx/protocol"
)

const (
	// Flat returns one row for every stock in every block.
	Flat = iota
	// Group returns one row for every block.
	Group
)

// BlockRow is the flat representation used by pytdx BlockReader.
type BlockRow struct {
	BlockName string `json:"blockname"`
	BlockType uint16 `json:"blockType"`
	CodeIndex int    `json:"codeIndex"`
	Code      string `json:"code"`
}

// BlockGroup is the grouped representation of a block file.
type BlockGroup struct {
	BlockName  string   `json:"blockname"`
	BlockType  uint16   `json:"blockType"`
	StockCount int      `json:"stockCount"`
	Codes      []string `json:"codes"`
}

// CustomBlockRow is one row from a blocknew.cfg custom-block directory.
type CustomBlockRow struct {
	BlockName string `json:"blockname"`
	BlockType string `json:"blockType"`
	CodeIndex int    `json:"codeIndex"`
	Code      string `json:"code"`
}

// CustomBlockGroup is one custom block and its member codes.
type CustomBlockGroup struct {
	BlockName  string   `json:"blockname"`
	BlockType  string   `json:"blockType"`
	StockCount int      `json:"stockCount"`
	Codes      []string `json:"codes"`
}

// ExDailyBar is one local extension-market daily record.
type ExDailyBar struct {
	Date          uint32  `json:"date"`
	Open          float32 `json:"open"`
	High          float32 `json:"high"`
	Low           float32 `json:"low"`
	Close         float32 `json:"close"`
	Amount        uint32  `json:"amount"`
	Volume        uint32  `json:"volume"`
	Settlement    float32 `json:"settlement"`
	HKStockAmount float32 `json:"hkStockAmount"`
}

// ReadDay reads a standard .day file through the existing protocol-aware reader.
func ReadDay(dir, code string) (protocol.Klines, error) {
	return extend.ReadDay(dir, code)
}

// ReadMinute1 reads a standard .lc1 file through the existing protocol-aware reader.
func ReadMinute1(dir, code string) (protocol.Klines, error) {
	return extend.ReadMinute1(dir, code)
}

// ReadMinute5 reads a standard .lc5 file through the existing protocol-aware reader.
func ReadMinute5(dir, code string) (protocol.Klines, error) {
	return extend.ReadMinute5(dir, code)
}

// ReadLegacyMinute reads the older standalone .5/.lc5-style file used by
// pytdx TdxMinBarReader, where OHLC values are uint32 price cents.
func ReadLegacyMinute(filename string) ([]protocol.Kline, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	const recordSize = 32
	if len(data)%recordSize != 0 {
		return nil, fmt.Errorf("legacy minute file has incomplete record: %s", filename)
	}
	result := make([]protocol.Kline, 0, len(data)/recordSize)
	for pos := 0; pos < len(data); pos += recordSize {
		r := data[pos : pos+recordSize]
		dateNum := binary.LittleEndian.Uint16(r[0:2])
		minute := binary.LittleEndian.Uint16(r[2:4])
		year := int(dateNum/2048) + 2004
		month := int(dateNum%2048) / 100
		day := int(dateNum%2048) % 100
		hour := int(minute) / 60
		min := int(minute) % 60
		result = append(result, protocol.Kline{
			Open:   protocol.Price(binary.LittleEndian.Uint32(r[4:8])) * 10,
			High:   protocol.Price(binary.LittleEndian.Uint32(r[8:12])) * 10,
			Low:    protocol.Price(binary.LittleEndian.Uint32(r[12:16])) * 10,
			Close:  protocol.Price(binary.LittleEndian.Uint32(r[16:20])) * 10,
			Amount: protocol.Price(int64(math.Float32frombits(binary.LittleEndian.Uint32(r[20:24])) * 1000)),
			Volume: int64(binary.LittleEndian.Uint32(r[24:28])),
			Time:   time.Date(year, time.Month(month), day, hour, min, 0, 0, time.Local),
		})
	}
	return result, nil
}

// ReadExDaily reads a local extension-market daily file such as ds/29#A1801.day.
func ReadExDaily(filename string) ([]ExDailyBar, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	const recordSize = 32
	if len(data)%recordSize != 0 {
		return nil, fmt.Errorf("extension daily file has incomplete record: %s", filename)
	}
	result := make([]ExDailyBar, 0, len(data)/recordSize)
	for pos := 0; pos < len(data); pos += recordSize {
		r := data[pos : pos+recordSize]
		amountBits := binary.LittleEndian.Uint32(r[20:24])
		result = append(result, ExDailyBar{
			Date:          binary.LittleEndian.Uint32(r[0:4]),
			Open:          math.Float32frombits(binary.LittleEndian.Uint32(r[4:8])),
			High:          math.Float32frombits(binary.LittleEndian.Uint32(r[8:12])),
			Low:           math.Float32frombits(binary.LittleEndian.Uint32(r[12:16])),
			Close:         math.Float32frombits(binary.LittleEndian.Uint32(r[16:20])),
			Amount:        amountBits,
			Volume:        binary.LittleEndian.Uint32(r[24:28]),
			Settlement:    math.Float32frombits(binary.LittleEndian.Uint32(r[28:32])),
			HKStockAmount: math.Float32frombits(amountBits),
		})
	}
	return result, nil
}

// ReadBlock reads a standard block_*.dat file in flat or grouped form.
func ReadBlock(filename string, resultType int) (any, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return ParseBlock(data, resultType)
}

// ParseBlock parses raw block_*.dat bytes in flat or grouped form.
func ParseBlock(data []byte, resultType int) (any, error) {
	blocks := protocol.ParseBlockFile(data)
	if blocks == nil && len(data) >= 386 {
		return nil, errors.New("invalid block file")
	}
	switch resultType {
	case Flat:
		result := make([]BlockRow, 0)
		for _, block := range blocks {
			for i, code := range block.Codes {
				result = append(result, BlockRow{BlockName: block.Name, BlockType: block.Type, CodeIndex: i, Code: code})
			}
		}
		return result, nil
	case Group:
		result := make([]BlockGroup, 0, len(blocks))
		for _, block := range blocks {
			codes := append([]string(nil), block.Codes...)
			result = append(result, BlockGroup{BlockName: block.Name, BlockType: block.Type, StockCount: len(codes), Codes: codes})
		}
		return result, nil
	default:
		return nil, fmt.Errorf("unknown block result type: %d", resultType)
	}
}

// ReadCustomerBlocks reads a TDX blocknew.cfg directory in flat or grouped form.
func ReadCustomerBlocks(dir string, resultType int) (any, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("custom block path is not a directory: %s", dir)
	}
	data, err := os.ReadFile(filepath.Join(dir, "blocknew.cfg"))
	if err != nil {
		return nil, err
	}
	groups := make([]CustomBlockGroup, 0)
	flat := make([]CustomBlockRow, 0)
	for pos := 0; pos+120 <= len(data); pos += 120 {
		name := decodeFixed(data[pos : pos+50])
		kind := decodeFixed(data[pos+50 : pos+120])
		if name == "" || kind == "" {
			continue
		}
		path := filepath.Join(dir, kind+".blk")
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		codes := make([]string, 0)
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			code := strings.TrimSpace(scanner.Text())
			if code == "" {
				continue
			}
			if len(code) > 0 {
				code = code[1:]
			}
			codes = append(codes, code)
		}
		closeErr := f.Close()
		if err := scanner.Err(); err != nil {
			return nil, err
		}
		if closeErr != nil {
			return nil, closeErr
		}
		if resultType == Flat {
			for i, code := range codes {
				flat = append(flat, CustomBlockRow{BlockName: name, BlockType: kind, CodeIndex: i, Code: code})
			}
		} else if resultType == Group {
			groups = append(groups, CustomBlockGroup{BlockName: name, BlockType: kind, StockCount: len(codes), Codes: codes})
		} else {
			return nil, fmt.Errorf("unknown block result type: %d", resultType)
		}
	}
	if resultType == Flat {
		return flat, nil
	}
	return groups, nil
}

func decodeFixed(data []byte) string {
	if i := strings.IndexByte(string(data), 0); i >= 0 {
		data = data[:i]
	}
	return string(protocol.UTF8ToGBK(data))
}
