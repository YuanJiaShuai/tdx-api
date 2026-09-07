// Package historyfinancial 读取和下载通达信历史专业财务数据。
package historyfinancial

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	listFile       = "tdxfin/gpcw.txt"
	stockHeaderLen = 20
	stockItemLen   = 11
)

// ListFilename is the report-file path containing the historical-financial index.
const ListFilename = listFile

// File describes one entry from tdxfin/gpcw.txt.
type File struct {
	Filename string `json:"filename"`
	Hash     string `json:"hash"`
	Filesize int64  `json:"filesize"`
}

// Record is one stock's historical financial record.
// Values preserves the original float32 fields in the DAT file without guessing
// field names that are not included in the file itself.
type Record struct {
	Code       string    `json:"code"`
	ReportDate uint32    `json:"reportDate"`
	Values     []float32 `json:"values"`
}

// Dataset is one parsed gpcw DAT file.
type Dataset struct {
	Version     int16    `json:"version"`
	ReportDate  uint32   `json:"reportDate"`
	RecordCount uint16   `json:"recordCount"`
	FieldCount  int      `json:"fieldCount"`
	Records     []Record `json:"records"`
}

// ParseList parses the UTF-8 text returned by tdxfin/gpcw.txt.
func ParseList(data []byte) ([]File, error) {
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	result := make([]File, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) < 3 {
			return nil, fmt.Errorf("invalid financial list line: %q", line)
		}
		size, err := strconv.ParseInt(strings.TrimSpace(parts[2]), 10, 64)
		if err != nil || size < 0 {
			return nil, fmt.Errorf("invalid financial file size in %q", line)
		}
		result = append(result, File{
			Filename: strings.TrimSpace(parts[0]),
			Hash:     strings.TrimSpace(parts[1]),
			Filesize: size,
		})
	}
	return result, nil
}

// Parse parses a raw gpcw DAT file.
func Parse(data []byte) (*Dataset, error) {
	if len(data) < stockHeaderLen {
		return nil, errors.New("financial DAT header is truncated")
	}
	version := int16(binary.LittleEndian.Uint16(data[0:2]))
	reportDate := binary.LittleEndian.Uint32(data[2:6])
	recordCount := binary.LittleEndian.Uint16(data[6:8])
	reportSize := binary.LittleEndian.Uint32(data[12:16])
	if reportSize == 0 || reportSize%4 != 0 {
		return nil, fmt.Errorf("invalid financial record size: %d", reportSize)
	}
	fieldCount := int(reportSize / 4)
	tableEnd := stockHeaderLen + int(recordCount)*stockItemLen
	if tableEnd > len(data) {
		return nil, errors.New("financial DAT stock table is truncated")
	}
	result := &Dataset{
		Version:     version,
		ReportDate:  reportDate,
		RecordCount: recordCount,
		FieldCount:  fieldCount,
		Records:     make([]Record, 0, recordCount),
	}
	for i := 0; i < int(recordCount); i++ {
		base := stockHeaderLen + i*stockItemLen
		code := strings.TrimRight(string(data[base:base+6]), "\x00")
		offset := binary.LittleEndian.Uint32(data[base+7 : base+11])
		end := uint64(offset) + uint64(reportSize)
		if end > uint64(len(data)) {
			return nil, fmt.Errorf("financial record %q points outside DAT: offset=%d size=%d", code, offset, reportSize)
		}
		values := make([]float32, fieldCount)
		for j := range values {
			start := int(offset) + j*4
			values[j] = mathFloat32(data[start : start+4])
		}
		result.Records = append(result.Records, Record{Code: code, ReportDate: reportDate, Values: values})
	}
	return result, nil
}

func mathFloat32(data []byte) float32 {
	return math.Float32frombits(binary.LittleEndian.Uint32(data))
}

// ParseZip parses the first .dat member in a gpcw zip archive.
func ParseZip(data []byte) (*Dataset, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("open financial zip: %w", err)
	}
	for _, member := range zr.File {
		if !strings.HasSuffix(strings.ToLower(member.Name), ".dat") {
			continue
		}
		r, err := member.Open()
		if err != nil {
			return nil, err
		}
		content, readErr := io.ReadAll(r)
		r.Close()
		if readErr != nil {
			return nil, readErr
		}
		return Parse(content)
	}
	return nil, errors.New("financial zip contains no DAT file")
}

// ParseFile parses either a raw .dat file or a .zip archive.
func ParseFile(filename string) (*Dataset, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(filepath.Ext(filename), ".zip") {
		return ParseZip(data)
	}
	return Parse(data)
}

// Find returns a record by six-digit code, or nil when absent.
func (d *Dataset) Find(code string) *Record {
	code = strings.TrimSpace(code)
	for i := range d.Records {
		if d.Records[i].Code == code {
			return &d.Records[i]
		}
	}
	return nil
}
