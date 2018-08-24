package capricoin

import (
	"blockbook/bchain"
	"blockbook/bchain/coins/btc"
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"github.com/juju/errors"
)

// CapricoinRPC is an interface to JSON-RPC Capricoind service.
type CapricoinRPC struct {
	*btc.BitcoinRPC
}

// NewCapricoinRPC returns new CapricoinRPC instance.
func NewCapricoinRPC(config json.RawMessage, pushHandler func(bchain.NotificationType)) (bchain.BlockChain, error) {
	b, err := btc.NewBitcoinRPC(config, pushHandler)
	if err != nil {
		return nil, err
	}

	s := &CapricoinRPC{
		b.(*btc.BitcoinRPC),
	}
	s.RPCMarshaler = btc.JSONMarshalerV1{}

	return s, nil
}

// Initialize initializes CapricoinRPC instance.
func (b *CapricoinRPC) Initialize() error {
	chainName, err := b.GetChainInfoAndInitializeMempool(b)
	if err != nil {
		return err
	}

	params := GetChainParams(chainName)

	// always create parser
	b.Parser = NewCapricoinParser(params, b.ChainConfig)

	// parameters for getInfo request
	if params.Net == MainnetMagic {
		b.Testnet = false
		b.Network = "livenet"
	} else {
		b.Testnet = true
		b.Network = "testnet"
	}

	glog.Info("rpc: block chain ", params.Name)
	return nil
}

type ResGetInfo struct {
	Error  *bchain.RPCError `json:"error"`
	Result struct {
		Version         string      `json:"version"`
		ProtocolVersion json.Number `json:"protocolversion"`
		Blocks          int         `json:"blocks"`
		TimeOffset      float64     `json:"timeoffset"`
		Difficulty      struct {
			ProofOfWork  json.Number `json:"proof-of-work"`
			ProofOfStake json.Number `json:"proof-of-stake"`
		} `json:"difficulty"`
		Testnet bool `json:"testnet"`
	} `json:"result"`
}

// GetChainInfo returns information about the connected backend
func (b *CapricoinRPC) GetChainInfo() (*bchain.ChainInfo, error) {
	glog.V(1).Info("rpc: getinfo")

	resI := ResGetInfo{}
	err := b.Call(&btc.CmdGetBlockChainInfo{Method: "getinfo"}, &resI)
	if err != nil {
		return nil, err
	}
	if resI.Error != nil {
		return nil, resI.Error
	}

	glog.V(1).Info("rpc: getbestblockhash")
	resBbh := btc.ResGetBestBlockHash{}
	err = b.Call(&btc.CmdGetBestBlockHash{Method: "getbestblockhash"}, &resBbh)
	if err != nil {
		return nil, err
	}

	rv := &bchain.ChainInfo{
		Bestblockhash:   resBbh.Result,
		Blocks:          resI.Result.Blocks,
		Difficulty:      fmt.Sprintf("PoW: %v, PoS: %v", resI.Result.Difficulty.ProofOfWork, resI.Result.Difficulty.ProofOfStake),
		Timeoffset:      resI.Result.TimeOffset,
		Version:         resI.Result.Version,
		ProtocolVersion: string(resI.Result.ProtocolVersion),
	}
	if resI.Result.Testnet {
		rv.Chain = "testnet"
	} else {
		rv.Chain = "mainnet"
	}
	return rv, nil
}

// GetBlock returns block with given hash.
func (b *CapricoinRPC) GetBlock(hash string, height uint32) (*bchain.Block, error) {
	var err error
	if hash == "" && height > 0 {
		hash, err = b.GetBlockHash(height)

		if err != nil {
			return nil, err
		}
	}
	return b.GetBlockFull(hash)
}

func isErrBlockNotFound(err *bchain.RPCError) bool {
	return err.Message == "Block not found" ||
		err.Message == "Block number out of range."
}

type CmdGetBlock struct {
	Method string `json:"method"`
	Params struct {
		BlockHash string `json:"blockhash"`
		TxInfo bool      `json:"txinfo"`
	} `json:"params"`
}

// GetBlockInfo returns extended header (more info than in bchain.BlockHeader) with a list of txids
func (b *CapricoinRPC) GetBlockInfo(hash string) (*bchain.BlockInfo, error) {
	glog.V(1).Info("rpc: getblock (txinfo=false) ", hash)

	res := btc.ResGetBlockInfo{}
	req := CmdGetBlock{Method: "getblock"}
	req.Params.BlockHash = hash
	req.Params.TxInfo = false
	err := b.Call(&req, &res)

	if err != nil {
		return nil, errors.Annotatef(err, "hash %v", hash)
	}
	if res.Error != nil {
		if isErrBlockNotFound(res.Error) {
			return nil, bchain.ErrBlockNotFound
		}
		return nil, errors.Annotatef(res.Error, "hash %v", hash)
	}
	return &res.Result, nil
}

// GetBlockHash returns hash of block in best-block-chain at given height.
func (b *CapricoinRPC) GetBlockHash(height uint32) (string, error) {
	glog.V(1).Info("rpc: getblockhash ", height)

	res := btc.ResGetBlockHash{}
	req := btc.CmdGetBlockHash{Method: "getblockhash"}
	req.Params.Height = height
	err := b.Call(&req, &res)

	if err != nil {
		return "", errors.Annotatef(err, "height %v", height)
	}
	if res.Error != nil {
		if isErrBlockNotFound(res.Error) {
			return "", bchain.ErrBlockNotFound
		}
		return "", errors.Annotatef(res.Error, "height %v", height)
	}
	return res.Result, nil
}
