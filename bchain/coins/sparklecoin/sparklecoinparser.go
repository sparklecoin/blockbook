package sparklecoin

import (
	"blockbook/bchain"
	"blockbook/bchain/coins/btc"
	"encoding/hex"
	"github.com/btcsuite/btcd/wire"
	"github.com/gogo/protobuf/proto"
	"github.com/jakm/btcutil/chaincfg"
	"github.com/juju/errors"
	"math/big"
)

const (
	MainnetMagic wire.BitcoinNet = 0xe6e8e9e5
	TestnetMagic wire.BitcoinNet = 0xcbf2c0ef
)

var (
	MainNetParams chaincfg.Params
	TestNetParams chaincfg.Params
)

func init() {
	MainNetParams = chaincfg.MainNetParams
	MainNetParams.Net = MainnetMagic
	MainNetParams.PubKeyHashAddrID = []byte{63}
	MainNetParams.ScriptHashAddrID = []byte{125}
	MainNetParams.Bech32HRPSegwit = "sprk"
	TestNetParams = chaincfg.TestNet3Params
	TestNetParams.Net = TestnetMagic
	TestNetParams.PubKeyHashAddrID = []byte{111}
	TestNetParams.ScriptHashAddrID = []byte{196}
	TestNetParams.Bech32HRPSegwit = "tpc"
}

type SparklecoinParser struct {
	*btc.BitcoinParser
}

func NewSparklecoinParser(params *chaincfg.Params, c *btc.Configuration) *SparklecoinParser {
	return &SparklecoinParser{BitcoinParser: btc.NewBitcoinParser(params, c)}
}

func GetChainParams(chain string) *chaincfg.Params {
	if !chaincfg.IsRegistered(&MainNetParams) {
		err := chaincfg.Register(&MainNetParams)
		if err == nil {
			err = chaincfg.Register(&TestNetParams)
		}
		if err != nil {
			panic(err)
		}
	}
	switch chain {
	case "testnet":
		return &TestNetParams
	default:
		return &MainNetParams
	}
}

// PackTx packs transaction to byte array using protobuf
func (p *SparklecoinParser) PackTx(tx *bchain.Tx, height uint32, blockTime int64) ([]byte, error) {
	var err error
	pti := make([]*ProtoTransaction_VinType, len(tx.Vin))
	for i, vi := range tx.Vin {
		hex, err := hex.DecodeString(vi.ScriptSig.Hex)
		if err != nil {
			return nil, errors.Annotatef(err, "Vin %v Hex %v", i, vi.ScriptSig.Hex)
		}
		itxid, err := p.PackTxid(vi.Txid)
		if err != nil {
			return nil, errors.Annotatef(err, "Vin %v Txid %v", i, vi.Txid)
		}
		pti[i] = &ProtoTransaction_VinType{
			Addresses:    vi.Addresses,
			Coinbase:     vi.Coinbase,
			ScriptSigHex: hex,
			Sequence:     vi.Sequence,
			Txid:         itxid,
			Vout:         vi.Vout,
		}
	}
	pto := make([]*ProtoTransaction_VoutType, len(tx.Vout))
	for i, vo := range tx.Vout {
		hex, err := hex.DecodeString(vo.ScriptPubKey.Hex)
		if err != nil {
			return nil, errors.Annotatef(err, "Vout %v Hex %v", i, vo.ScriptPubKey.Hex)
		}
		pto[i] = &ProtoTransaction_VoutType{
			Addresses:       vo.ScriptPubKey.Addresses,
			N:               vo.N,
			ScriptPubKeyHex: hex,
			ValueSat:        vo.ValueSat.Bytes(),
		}
	}
	pt := &ProtoTransaction{
		Blocktime: uint64(blockTime),
		Height:    height,
		Locktime:  tx.LockTime,
		Vin:       pti,
		Vout:      pto,
		Version:   tx.Version,
		Time:      uint64(tx.Time),
	}
	if pt.Hex, err = hex.DecodeString(tx.Hex); err != nil {
		return nil, errors.Annotatef(err, "Hex %v", tx.Hex)
	}
	if pt.Txid, err = p.PackTxid(tx.Txid); err != nil {
		return nil, errors.Annotatef(err, "Txid %v", tx.Txid)
	}
	return proto.Marshal(pt)
}

// UnpackTx unpacks transaction from protobuf byte array
func (p *SparklecoinParser) UnpackTx(buf []byte) (*bchain.Tx, uint32, error) {
	var pt ProtoTransaction
	err := proto.Unmarshal(buf, &pt)
	if err != nil {
		return nil, 0, err
	}
	txid, err := p.UnpackTxid(pt.Txid)
	if err != nil {
		return nil, 0, err
	}
	vin := make([]bchain.Vin, len(pt.Vin))
	for i, pti := range pt.Vin {
		itxid, err := p.UnpackTxid(pti.Txid)
		if err != nil {
			return nil, 0, err
		}
		vin[i] = bchain.Vin{
			Addresses: pti.Addresses,
			Coinbase:  pti.Coinbase,
			ScriptSig: bchain.ScriptSig{
				Hex: hex.EncodeToString(pti.ScriptSigHex),
			},
			Sequence: pti.Sequence,
			Txid:     itxid,
			Vout:     pti.Vout,
		}
	}
	vout := make([]bchain.Vout, len(pt.Vout))
	for i, pto := range pt.Vout {
		var vs big.Int
		vs.SetBytes(pto.ValueSat)
		vout[i] = bchain.Vout{
			N: pto.N,
			ScriptPubKey: bchain.ScriptPubKey{
				Addresses: pto.Addresses,
				Hex:       hex.EncodeToString(pto.ScriptPubKeyHex),
			},
			ValueSat: vs,
		}
	}
	tx := bchain.Tx{
		Blocktime: int64(pt.Blocktime),
		Hex:       hex.EncodeToString(pt.Hex),
		LockTime:  pt.Locktime,
		Time:      int64(pt.Time),
		Txid:      txid,
		Vin:       vin,
		Vout:      vout,
		Version:   pt.Version,
	}
	return &tx, pt.Height, nil
}
