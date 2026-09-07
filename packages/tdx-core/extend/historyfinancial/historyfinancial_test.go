package historyfinancial

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"math"
	"testing"
)

func makeDAT() []byte {
	const (
		headerSize = 20
		itemSize   = 11
		recordSize = 8
	)
	data := make([]byte, headerSize+itemSize+recordSize)
	binary.LittleEndian.PutUint16(data[0:2], 1)
	binary.LittleEndian.PutUint32(data[2:6], 20260822)
	binary.LittleEndian.PutUint16(data[6:8], 1)
	binary.LittleEndian.PutUint32(data[12:16], recordSize)
	copy(data[headerSize:headerSize+6], "600519")
	binary.LittleEndian.PutUint32(data[headerSize+7:headerSize+11], headerSize+itemSize)
	binary.LittleEndian.PutUint32(data[headerSize+itemSize:headerSize+itemSize+4], math.Float32bits(1.25))
	binary.LittleEndian.PutUint32(data[headerSize+itemSize+4:], math.Float32bits(-2.5))
	return data
}

func TestParseDAT(t *testing.T) {
	dataset, err := Parse(makeDAT())
	if err != nil {
		t.Fatal(err)
	}
	if dataset.ReportDate != 20260822 || dataset.FieldCount != 2 || len(dataset.Records) != 1 {
		t.Fatalf("unexpected dataset: %#v", dataset)
	}
	record := dataset.Find("600519")
	if record == nil || len(record.Values) != 2 || record.Values[0] != 1.25 || record.Values[1] != -2.5 {
		t.Fatalf("unexpected record: %#v", record)
	}
}

func TestParseListAndZip(t *testing.T) {
	files, err := ParseList([]byte("gpcw20260822.zip,abc,123\r\n"))
	if err != nil || len(files) != 1 || files[0].Filesize != 123 {
		t.Fatalf("unexpected list: %#v %v", files, err)
	}
	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)
	w, err := zw.Create("gpcw.dat")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = w.Write(makeDAT()); err != nil {
		t.Fatal(err)
	}
	if err = zw.Close(); err != nil {
		t.Fatal(err)
	}
	dataset, err := ParseZip(archive.Bytes())
	if err != nil || dataset.Find("600519") == nil {
		t.Fatalf("unexpected zip parse: %#v %v", dataset, err)
	}
}
