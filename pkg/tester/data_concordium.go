package tester

import (
	"context"
	"github.com/btcsuite/btcd/btcutil/base58"
	"github.com/coinbase/rosetta-sdk-go/syncer"
	"github.com/coinbase/rosetta-sdk-go/types"
)

// concordiumHelper is a wrapper around a sync.Helper that monkey patches fetched blocks
// by rewriting all account addresses into a "normalized" form (the zero'th alias).
// This is necessary for DataTester to do correct accounting.
type concordiumHelper struct {
	helper syncer.Helper
}

func newConcordiumHelper(h syncer.Helper) *concordiumHelper {
	return &concordiumHelper{
		helper: h,
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
		account.Address = addressToAliasZero(account.Address)
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
