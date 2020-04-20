// Copyright 2020 Coinbase, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package logger

import (
	"context"
	"fmt"
	"log"
	"os"
	"path"

	"github.com/coinbase/rosetta-validator/internal/storage"

	"github.com/coinbase/rosetta-sdk-go/types"
)

const (
	// blockStreamFile contains the stream of processed
	// blocks and whether they were added or removed.
	blockStreamFile = "blocks.txt"

	// transactionStreamFile contains the stream of processed
	// transactions and whether they were added or removed.
	transactionStreamFile = "transactions.txt"

	// balanceStreamFile contains the stream of processed
	// balance changes.
	balanceStreamFile = "balances.txt"

	// reconcileSuccessStreamFile contains the stream of processed
	// reconciliations.
	reconcileSuccessStreamFile = "successful_reconciliations.txt"
	reconcileFailureStreamFile = "failure_reconciliations.txt"

	// addEvent is printed in a stream
	// when an event is added.
	addEvent = "Add"

	// removeEvent is printed in a stream
	// when an event is orphaned.
	removeEvent = "Remove"

	// logFilePermissions specifies that the user can
	// read and write the file.
	logFilePermissions = 0600
)

// Logger contains all logic to record validator output
// and benchmark a Rosetta Server.
type Logger struct {
	logDir            string
	logTransactions   bool
	logBalances       bool
	logReconciliation bool
}

// NewLogger constructs a new Logger.
func NewLogger(
	logDir string,
	logTransactions bool,
	logBalances bool,
	logReconciliation bool,
) *Logger {
	return &Logger{
		logDir:            logDir,
		logTransactions:   logTransactions,
		logBalances:       logBalances,
		logReconciliation: logReconciliation,
	}
}

// BlockStream writes the next processed block to the end of the
// blockStreamFile output file.
func (l *Logger) BlockStream(
	ctx context.Context,
	block *types.Block,
	orphan bool,
) error {
	f, err := os.OpenFile(
		path.Join(l.logDir, blockStreamFile),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		logFilePermissions,
	)
	if err != nil {
		return err
	}
	defer f.Close()

	verb := addEvent
	if orphan {
		verb = removeEvent
	}

	_, err = f.WriteString(fmt.Sprintf(
		"%s Block %d:%s with Parent Block %d:%s\n",
		verb,
		block.BlockIdentifier.Index,
		block.BlockIdentifier.Hash,
		block.ParentBlockIdentifier.Index,
		block.ParentBlockIdentifier.Hash,
	))
	if err != nil {
		return err
	}

	return l.TransactionStream(ctx, block, verb)
}

// TransactionStream writes the next processed block's transactions
// to the end of the transactionStreamFile.
func (l *Logger) TransactionStream(
	ctx context.Context,
	block *types.Block,
	verb string,
) error {
	if !l.logTransactions {
		return nil
	}

	f, err := os.OpenFile(
		path.Join(l.logDir, transactionStreamFile),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		logFilePermissions,
	)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, tx := range block.Transactions {
		_, err = f.WriteString(fmt.Sprintf(
			"%s Transaction %s at Block %d:%s\n",
			verb,
			tx.TransactionIdentifier.Hash,
			block.BlockIdentifier.Index,
			block.BlockIdentifier.Hash,
		))
		if err != nil {
			return err
		}

		for _, op := range tx.Operations {
			amount := ""
			symbol := ""
			if op.Amount != nil {
				amount = op.Amount.Value
				symbol = op.Amount.Currency.Symbol
			}
			participant := ""
			if op.Account != nil {
				participant = op.Account.Address
			}

			networkIndex := op.OperationIdentifier.Index
			if op.OperationIdentifier.NetworkIndex != nil {
				networkIndex = *op.OperationIdentifier.NetworkIndex
			}

			_, err = f.WriteString(fmt.Sprintf(
				"TxOp %d(%d) %s %s %s %s %s\n",
				op.OperationIdentifier.Index,
				networkIndex,
				op.Type,
				participant,
				amount,
				symbol,
				op.Status,
			))
			if err != nil {
				return err
			}

			if op.Account != nil && op.Account.Metadata != nil {
				_, err = f.WriteString(fmt.Sprintf("Account Metadata: %+v\n", op.Account.Metadata))
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// BalanceStream writes a slice of storage.BalanceChanges
// to the balanceStreamFile.
func (l *Logger) BalanceStream(
	ctx context.Context,
	balanceChanges []*storage.BalanceChange,
) error {
	if !l.logBalances {
		return nil
	}

	f, err := os.OpenFile(
		path.Join(l.logDir, balanceStreamFile),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		logFilePermissions,
	)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, balanceChange := range balanceChanges {
		balanceLog := fmt.Sprintf(
			"Account: %s Change: %s%s Block: %d:%s",
			balanceChange.Account.Address,
			balanceChange.Difference,
			balanceChange.Currency.Symbol,
			balanceChange.Block.Index,
			balanceChange.Block.Hash,
		)

		if _, err := f.WriteString(fmt.Sprintf("%s\n", balanceLog)); err != nil {
			return err
		}
	}
	return nil
}

// ReconcileSuccessStream logs all reconciliation checks performed
// during syncing.
func (l *Logger) ReconcileSuccessStream(
	ctx context.Context,
	reconciliationType string,
	account *types.AccountIdentifier,
	balance *types.Amount,
	block *types.BlockIdentifier,
) error {
	if !l.logReconciliation {
		return nil
	}

	f, err := os.OpenFile(
		path.Join(l.logDir, reconcileSuccessStreamFile),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		logFilePermissions,
	)
	if err != nil {
		return err
	}
	defer f.Close()

	log.Printf(
		"%s Reconciled %+v at %d\n",
		reconciliationType,
		account,
		block.Index,
	)

	_, err = f.WriteString(fmt.Sprintf(
		"Type:%s Account: %s Currency: %s Balance: %s Block: %d:%s\n",
		reconciliationType,
		account.Address,
		balance.Currency.Symbol,
		balance.Value,
		block.Index,
		block.Hash,
	))
	if err != nil {
		return err
	}

	return nil
}

// ReconcileFailureStream logs all reconciliation checks performed
// during syncing.
func (l *Logger) ReconcileFailureStream(
	ctx context.Context,
	reconciliationType string,
	account *types.AccountIdentifier,
	currency *types.Currency,
	difference string,
	differenceDetail string,
	block *types.BlockIdentifier,
) error {
	if !l.logReconciliation {
		return nil
	}

	f, err := os.OpenFile(
		path.Join(l.logDir, reconcileFailureStreamFile),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		logFilePermissions,
	)
	if err != nil {
		return err
	}
	defer f.Close()

	log.Printf(
		"%s Reconciliation failed for %+v at %d by %s %s\n",
		reconciliationType,
		account,
		block.Index,
		difference,
		differenceDetail,
	)

	_, err = f.WriteString(fmt.Sprintf(
		"Type:%s Account: %+v Currency: %+v Block: %s:%d Difference%s:%s\n",
		reconciliationType,
		account,
		currency,
		block.Hash,
		block.Index,
		differenceDetail,
		difference,
	))
	if err != nil {
		return err
	}

	return nil
}
