package sparklecoin

import (
	"blockbook/bchain"
	"blockbook/bchain/coins/btc"
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"github.com/juju/errors"
)

// SparklecoinRPC is an interface to JSON-RPC Sparklecoind service.
type SparklecoinRPC struct {
	*btc.BitcoinRPC
}

// NewSparklecoinRPC returns new SparklecoinRPC instance.
func NewSparklecoinRPC(config json.RawMessage, pushHandler func(bchain.NotificationType)) (bchain.BlockChain, error) {
	b, err := btc.NewBitcoinRPC(config, pushHandler)
	if err != nil {
		return nil, err
	}

	s := &SparklecoinRPC{
		b.(*btc.BitcoinRPC),
	}
	s.RPCMarshaler = btc.JSONMarshalerV1{}

	return s, nil
}

// Initialize initializes SparklecoinRPC instance.
func (b *SparklecoinRPC) Initialize() error {
	chainName, err := b.GetChainInfoAndInitializeMempool(b)
	if err != nil {
		return err
	}

	glog.Info("Chain name ", chainName)
	params := GetChainParams(chainName)

	// always create parser
	b.Parser = NewSparklecoinParser(params, b.ChainConfig)

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
		Difficulty      float64     `json:"difficulty"`
                Testnet bool `json:"testnet"`
        } `json:"result"`
}

// GetChainInfo returns information about the connected backend
func (b *SparklecoinRPC) GetChainInfo() (*bchain.ChainInfo, error) {
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
                Difficulty:      fmt.Sprintf("PoW: %v, PoS: 1.0", resI.Result.Difficulty),
                Timeoffset:      resI.Result.TimeOffset,
                Version:         resI.Result.Version,
                ProtocolVersion: string(resI.Result.ProtocolVersion),
        }
        if resI.Result.Testnet {
                rv.Chain = "testnet"
        } else {
                rv.Chain = "livenet"
        }
        return rv, nil
}

// GetBlock returns block with given hash.
func (b *SparklecoinRPC) GetBlock(hash string, height uint32) (*bchain.Block, error) {
        var err error
        if hash == "" && height > 0 {
                hash, err = b.GetBlockHash(height)
                if err != nil {
                        return nil, err
                }
        }

        glog.V(1).Info("rpc: getblock (verbosity=1) ", hash)

        res := btc.ResGetBlockThin{}
        req := btc.CmdGetBlock{Method: "getblock"}
        req.Params.BlockHash = hash
        req.Params.Verbosity = 1
        err = b.Call(&req, &res)

        if err != nil {
                return nil, errors.Annotatef(err, "hash %v", hash)
        }
        if res.Error != nil {
                return nil, errors.Annotatef(res.Error, "hash %v", hash)
        }

        txs := make([]bchain.Tx, 0, len(res.Result.Txids))
        for _, txid := range res.Result.Txids {
                tx, err := b.GetTransaction(txid)
                if err != nil {
                        if isInvalidTx(err) {
                                glog.Errorf("rpc: getblock: skipping transanction in block %s due error: %s", hash, err)
                                continue
                        }
                        return nil, err
                }
                txs = append(txs, *tx)
        }
        block := &bchain.Block{
                BlockHeader: res.Result.BlockHeader,
                Txs:         txs,
        }
        return block, nil
}

func isInvalidTx(err error) bool {
        switch e1 := err.(type) {
        case *errors.Err:
                switch e2 := e1.Cause().(type) {
                case *bchain.RPCError:
                        if e2.Code == -5 { // "No information available about transaction"
                                return true
                        }
                }
        }

        return false
}

func isErrBlockNotFound(err *bchain.RPCError) bool {
	return err.Message == "Block not found" ||
		err.Message == "Block height out of range"
}

type CmdGetBlock struct {
	Method string `json:"method"`
	Params struct {
		BlockHash string `json:"blockhash"`
		Verbosity int    `json:"verbosity"`
	} `json:"params"`
}

// GetBlockInfo returns extended header (more info than in bchain.BlockHeader) with a list of txids
/*
func (b *SparklecoinRPC) GetBlockInfo(hash string) (*bchain.BlockInfo, error) {
	glog.V(1).Info("rpc: getblock (txinfo=false) ", hash)

	res := btc.ResGetBlockInfo{}
	req := CmdGetBlock{Method: "getblock"}
	req.Params.BlockHash = hash
	req.Params.Verbosity = 1
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
*/
// GetBlockHash returns hash of block in best-block-chain at given height.
func (b *SparklecoinRPC) GetBlockHash(height uint32) (string, error) {
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

// GetTransactionForMempool returns a transaction by the transaction ID.
// It could be optimized for mempool, i.e. without block time and confirmations
func (b *SparklecoinRPC) GetTransactionForMempool(txid string) (*bchain.Tx, error) {
	return b.GetTransaction(txid)
}

// GetTransaction returns a transaction by the transaction ID.
func (b *SparklecoinRPC) GetTransaction(txid string) (*bchain.Tx, error) {
	r, err := b.GetTransactionSpecific(txid)
	if err != nil {
		return nil, err
	}
	tx, err := b.Parser.ParseTxFromJson(r)
	if err != nil {
		return nil, errors.Annotatef(err, "txid %v", txid)
	}
	for i := range tx.Vout {
		if tx.Vout[i].ScriptPubKey.Addresses == nil {
			tx.Vout[i].ScriptPubKey.Addresses = []string{}
		}
	}
	return tx, nil
}

// GetTransactionSpecific returns json as returned by backend, with all coin specific data
func (b *SparklecoinRPC) GetTransactionSpecific(txid string) (json.RawMessage, error) {
	r, err := b.BitcoinRPC.GetTransactionSpecific(txid)
	if err != nil {
		return r, err
	}
	// Sparklecoind getrawtransaction returns multiple "time" fields with different values.
	// We need to ensure the latter is removed.
	result, err := removeDuplicateJSONKeys(r)
	if err != nil {
		return nil, errors.Annotatef(err, "txid %v", txid)
	}
	return result, nil
}
