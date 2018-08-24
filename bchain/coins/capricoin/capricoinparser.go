package capricoin

import (
	"blockbook/bchain/coins/btc"
	"github.com/jakm/btcutil/chaincfg"
	"github.com/btcsuite/btcd/wire"
	"blockbook/bchain"
)

const (
	MainnetMagic wire.BitcoinNet = 0xa3a2a0a1
)

var (
	MainNetParams chaincfg.Params
)

func init() {
	MainNetParams = chaincfg.MainNetParams
	MainNetParams.Net = MainnetMagic
	MainNetParams.PubKeyHashAddrID = []byte{28}
	MainNetParams.ScriptHashAddrID = []byte{35}

	err := chaincfg.Register(&MainNetParams)
	if err != nil {
		panic(err)
	}
}

type CapricoinParser struct {
	*btc.BitcoinParser
}

func NewCapricoinParser(params *chaincfg.Params, c *btc.Configuration) *CapricoinParser {
	return &CapricoinParser{BitcoinParser: btc.NewBitcoinParser(params, c)}
}

func GetChainParams(chain string) *chaincfg.Params {
	return &MainNetParams
}

func (p *CapricoinParser) PackTx(tx *bchain.Tx, height uint32, blockTime int64) ([]byte, error) {
	return p.BitcoinParser.BaseParser.PackTx(tx, height, blockTime)
}

func (p *CapricoinParser) UnpackTx(buf []byte) (*bchain.Tx, uint32, error) {
	return p.BitcoinParser.BaseParser.UnpackTx(buf)
}