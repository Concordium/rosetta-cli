package tester

import (
	"context"
	"sync"

	"github.com/btcsuite/btcd/btcutil/base58"
	"github.com/coinbase/rosetta-sdk-go/syncer"
	"github.com/coinbase/rosetta-sdk-go/types"
)

// concordiumHelper is a wrapper around a sync.Helper that monkey patches fetched blocks
// by rewriting all account addresses into a "normalized" form (the zero'th alias).
// This is necessary for DataTester to do correct accounting.
type concordiumHelper struct {
	// helper is the wrapped syncer.Helper instance.
	helper syncer.Helper
	// cache is a map from the zero-alias to the canonical address (assumed by being the first alias observed - probably the account's creation).
	// We must use the canonical/non-aliased address instead of just the zero-alias to support networks such as mainnet
	// which started on a protocol version that didn't support aliases.
	cache     map[string]string
	cacheLock sync.RWMutex
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
		a0 := addressToAliasZero(account.Address)
		h.cacheLock.RLock()
		a, ok := h.cache[a0]
		h.cacheLock.RUnlock()
		if ok {
			account.Address = a
		} else {
			// NOTE: Due to parallelism, it isn't guaranteed that we see the blocks in strict sequential order.
			// But the only case we have a problem is when using a non-canonical alias when querying blocks from a protocol version that doesn't support aliases;
			// before the protocol update we never see any aliases.
			// So this can only fail if an account was created just before the protocol update took effect and was then referenced with an alias just afterwards (i.e. with in the window of parallelism).
			// Then we could get unlucky and process the latter block first, then fail when querying the balance before the update afterwards.
			// That situation seems very unlikely.
			h.cacheLock.Lock()
			h.cache[a0] = account.Address // first time we see the address; assume that it's the canonical variant.
			h.cacheLock.Unlock()
		}
	}
}

func addressToAliasZero(addr string) string {
	addrBytes, versionByte, err := base58.CheckDecode(addr)
	if err != nil || len(addrBytes) != 32 {
		// Non-Base58Check address is likely virtual (see https://github.com/Concordium/concordium-rosetta/#identifiers);
		// return unchanged.
		return addr
	}
	// Zero out last 3 bytes and re-encode.
	addrBytes[29] = 0
	addrBytes[30] = 0
	addrBytes[31] = 0
	return base58.CheckEncode(addrBytes, versionByte)
}
