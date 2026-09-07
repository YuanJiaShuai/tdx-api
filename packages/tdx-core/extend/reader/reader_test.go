package reader

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestReadExDaily(t *testing.T) {
	dir := t.TempDir()
	data := make([]byte, 32)
	binary.LittleEndian.PutUint32(data[0:4], 20260907)
	binary.LittleEndian.PutUint32(data[4:8], math.Float32bits(1.0))
	binary.LittleEndian.PutUint32(data[16:20], math.Float32bits(1.2))
	binary.LittleEndian.PutUint32(data[24:28], 88)
	binary.LittleEndian.PutUint32(data[28:32], math.Float32bits(1.1))
	path := filepath.Join(dir, "sample.day")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	rows, err := ReadExDaily(path)
	if err != nil || len(rows) != 1 || rows[0].Date != 20260907 || rows[0].Volume != 88 || rows[0].Close != 1.2 {
		t.Fatalf("unexpected rows: %#v %v", rows, err)
	}
}

func TestReadCustomerBlocks(t *testing.T) {
	dir := t.TempDir()
	cfg := make([]byte, 120)
	copy(cfg[0:50], "自选股")
	copy(cfg[50:120], "myblock")
	if err := os.WriteFile(filepath.Join(dir, "blocknew.cfg"), cfg, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "myblock.blk"), []byte("0sh600519\n1sz000001\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	value, err := ReadCustomerBlocks(dir, Group)
	if err != nil {
		t.Fatal(err)
	}
	groups := value.([]CustomBlockGroup)
	if len(groups) != 1 || len(groups[0].Codes) != 2 || groups[0].Codes[0] != "sh600519" || groups[0].BlockType != "myblock" {
		t.Fatalf("unexpected custom blocks: %#v", groups)
	}
}
