package workbench

import (
	"errors"

	"github.com/injoyai/tdx"
)

var codeModelProvider func() ([]*tdx.CodeModel, error)

func SetCodeModelProvider(provider func() ([]*tdx.CodeModel, error)) {
	codeModelProvider = provider
}

func getAllCodeModels() ([]*tdx.CodeModel, error) {
	if codeModelProvider == nil {
		return nil, errors.New("代码列表提供器未初始化")
	}
	return codeModelProvider()
}
