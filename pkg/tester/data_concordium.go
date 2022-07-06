package tester

import (
	"context"
	"encoding/hex"
	"log"
	"sync"

	"github.com/anaskhan96/base58check"
	"github.com/coinbase/rosetta-sdk-go/syncer"
	"github.com/coinbase/rosetta-sdk-go/types"
)

// concordiumHelper is a wrapper around a sync.Helper that monkey patches fetched blocks
// by rewriting all account addresses into a "normalized" form (the zero'th alias).
// This is necessary for DataTester to do correct accounting.
type concordiumHelper struct {
	helper    syncer.Helper
	cacheLock sync.RWMutex
	cache     map[string]string
}

func newConcordiumHelper(h syncer.Helper) *concordiumHelper {
	return &concordiumHelper{
		helper: h,
		cache:  make(map[string]string),
	}
}

func (h *concordiumHelper) NetworkStatus(ctx context.Context, network *types.NetworkIdentifier) (*types.NetworkStatusResponse, error) {
	return h.helper.NetworkStatus(ctx, network)
}

func (h *concordiumHelper) Block(ctx context.Context, network *types.NetworkIdentifier, block *types.PartialBlockIdentifier) (*types.Block, error) {
	b, err := h.helper.Block(ctx, network, block)
	if err == nil {
		// Patch all account references in fetched block.
		for _, tx := range b.Transactions {
			for _, op := range tx.Operations {
				h.normalizeAddress(op.Account)
			}
		}
	}
	return b, err
}

func (h *concordiumHelper) normalizeAddress(account *types.AccountIdentifier) {
	if account != nil {
		h.cacheLock.RLock()
		a, ok := h.cache[account.Address]
		h.cacheLock.RUnlock()
		if !ok {
			a = addressToAliasZero(account.Address)
			h.cacheLock.Lock()
			h.cache[account.Address] = a
			h.cacheLock.Unlock()
		}
		account.Address = a
	}
}

func addressToAliasZero(addr string) string {
	// Lots of redundant encoding/decoding back and forth due to the design choices of the base58check lib.
	decodedStr, err := base58check.Decode(addr)
	if err != nil {
		// Non-Base58Check address is likely virtual (see https://github.com/Concordium/concordium-rosetta/#identifiers);
		// return unchanged.
		return addr
	}
	decodedBytes, err := hex.DecodeString(decodedStr)
	if err != nil {
		// Never happens as it just inverts the hex encode steps done in base58check.Decode.
		log.Fatal(err)
	}
	versionBytes, addrBytes := decodedBytes[0:1], decodedBytes[1:]
	versionStr := hex.EncodeToString(versionBytes)

	// Zero out last 3 bytes.
	if len(addrBytes) == 32 {
		addrBytes[29] = 0
		addrBytes[30] = 0
		addrBytes[31] = 0
	}

	addr0, err := base58check.Encode(versionStr, hex.EncodeToString(addrBytes))
	if err != nil {
		// Never happens as base58check.Encode just inverts the hex encode steps above.
		log.Fatal(err)
	}
	return addr0
}
