// Copyright (c) 2026 The utreexod developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/utreexo/utreexo"
	"github.com/utreexo/utreexod/btcutil"
	"github.com/utreexo/utreexod/chaincfg/chainhash"
	"github.com/utreexo/utreexod/database"
)

var errRegressionProofStore = errors.New("regression proof store failure")

// regressionProofStoreFailDB wraps write transactions so proof persistence can
// be failed without affecting any of the other database writes ProcessBlock
// performs before connecting a block.
type regressionProofStoreFailDB struct {
	database.DB
}

// Update wraps the transaction passed to the update callback.
func (db *regressionProofStoreFailDB) Update(fn func(database.Tx) error) error {
	return db.DB.Update(func(dbTx database.Tx) error {
		return fn(&regressionProofStoreFailTx{Tx: dbTx})
	})
}

// regressionProofStoreFailTx fails proof writes and forwards all other
// transaction operations to the underlying database transaction.
type regressionProofStoreFailTx struct {
	database.Tx
}

// StoreUtreexoProof simulates a proof persistence failure.
func (tx *regressionProofStoreFailTx) StoreUtreexoProof(*chainhash.Hash,
	[]byte) error {

	return errRegressionProofStore
}

// TestUtreexoViewRollbackOnProofStoreFailure ensures a block is not left
// applied to the in-memory accumulator when its proof cannot be persisted.
func TestUtreexoViewRollbackOnProofStoreFailure(t *testing.T) {
	chain, params, _, tearDown := countingUtreexoTestChain(t,
		"proof-store-rollback")
	defer tearDown()

	genesis := btcutil.NewBlock(params.GenesisBlock)
	genesis.SetHeight(0)
	proofState := newTestUtreexoProofState()
	block, _ := proofState.newBlock(t, chain, genesis, nil)

	rootsBefore := append([]utreexo.Hash(nil),
		chain.utreexoView.accumulator.GetRoots()...)
	leavesBefore := chain.utreexoView.NumLeaves()
	tipBefore := chain.bestChain.Tip().hash

	chain.db = &regressionProofStoreFailDB{DB: chain.db}
	mainChain, orphan, err := chain.ProcessBlock(block, BFNone)
	require.ErrorIs(t, err, errRegressionProofStore)
	require.False(t, mainChain, "failed block reported on main chain")
	require.False(t, orphan, "failed block reported as orphan")
	require.Equal(t, tipBefore, chain.bestChain.Tip().hash,
		"best-chain tip changed after proof store failure")
	require.True(t, chain.utreexoView.compareRoots(rootsBefore),
		"proof store failure left the block applied to the accumulator")
	require.Equal(t, leavesBefore, chain.utreexoView.NumLeaves(),
		"proof store failure changed the accumulator leaf count")
}
