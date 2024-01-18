// Copyright (c) 2021 The utreexo developer
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package indexers

import (
	"bytes"
	"fmt"
	"sync"

	"github.com/utreexo/utreexo"
	"github.com/utreexo/utreexod/blockchain"
	"github.com/utreexo/utreexod/btcutil"
	"github.com/utreexo/utreexod/chaincfg"
	"github.com/utreexo/utreexod/chaincfg/chainhash"
	"github.com/utreexo/utreexod/database"
	"github.com/utreexo/utreexod/wire"
)

const (
	// utreexoProofIndexName is the human-readable name for the index.
	utreexoProofIndexName = "utreexo proof index"
)

var (
	// utreexoParentBucketKey is the name of the parent bucket of the
	// utreexo proof index.  It contains the buckets for the proof and
	// the undo data.
	utreexoParentBucketKey = []byte("utreexoparentindexkey")

	// utreexoProofIndexKey is the name of the utreexo proof data.  It
	// is included in the utreexoParentBucketKey and contains utreexo proof
	// data.
	utreexoProofIndexKey = []byte("utreexoproofindexkey")

	// utreexoStateKey is the name of the utreexo state data.  It is included
	// in the utreexoParentBucketKey and contains the utreexo state data.
	utreexoStateKey = []byte("utreexostatekey")

	// utreexoUndoKey is the name of the utreexo undo data bucket. It is included
	// in the utreexoParentBucketKey and contains the data necessary for disconnecting
	// blocks.
	utreexoUndoKey = []byte("utreexoundokey")
)

// Ensure the UtreexoProofIndex type implements the Indexer interface.
var _ Indexer = (*UtreexoProofIndex)(nil)

// Ensure the UtreexoProofIndex type implements the NeedsInputser interface.
var _ NeedsInputser = (*UtreexoProofIndex)(nil)

// UtreexoProofIndex implements a utreexo accumulator proof index for all the blocks.
type UtreexoProofIndex struct {
	db          database.DB
	chainParams *chaincfg.Params

	// If the node is a pruned node or not.
	pruned bool

	// The blockchain instance the index corresponds to.
	chain *blockchain.BlockChain

	// mtx protects concurrent access to utreexoView.
	mtx *sync.RWMutex

	// utreexoState represents the Bitcoin UTXO set as a utreexo accumulator.
	// It keeps all the elements of the forest in order to generate proofs.
	utreexoState *UtreexoState
}

// NeedsInputs signals that the index requires the referenced inputs in order
// to properly create the index.
//
// This implements the NeedsInputser interface.
func (idx *UtreexoProofIndex) NeedsInputs() bool {
	return true
}

// Init initializes the utreexo proof index. This is part of the Indexer
// interface.
func (idx *UtreexoProofIndex) Init() error {
	return nil // nothing to do
}

// Name returns the human-readable name of the index.
//
// This is part of the Indexer interface.
func (idx *UtreexoProofIndex) Name() string {
	return utreexoProofIndexName
}

// Key returns the database key to use for the index as a byte slice. This is
// part of the Indexer interface.
func (idx *UtreexoProofIndex) Key() []byte {
	return utreexoParentBucketKey
}

// Create is invoked when the indexer manager determines the index needs
// to be created for the first time.  It creates the bucket for the utreexo proof
// index.
//
// This is part of the Indexer interface.
func (idx *UtreexoProofIndex) Create(dbTx database.Tx) error {
	utreexoParentBucket, err := dbTx.Metadata().CreateBucket(utreexoParentBucketKey)
	if err != nil {
		return err
	}

	_, err = utreexoParentBucket.CreateBucket(utreexoProofIndexKey)
	if err != nil {
		return err
	}

	_, err = utreexoParentBucket.CreateBucket(utreexoStateKey)
	if err != nil {
		return err
	}

	return nil
}

// ConnectBlock is invoked by the index manager when a new block has been
// connected to the main chain.
//
// This is part of the Indexer interface.
func (idx *UtreexoProofIndex) ConnectBlock(dbTx database.Tx, block *btcutil.Block,
	stxos []blockchain.SpentTxOut) error {

	// Don't include genesis blocks.
	if block.Height() == 0 {
		log.Tracef("UtreexoProofIndex.ConnectBlock: Asked to connect genesis"+
			" block (height %d) Ignoring request and skipping block",
			block.Height())
		return nil
	}

	_, outCount, inskip, outskip := blockchain.DedupeBlock(block)
	dels, _, err := blockchain.BlockToDelLeaves(stxos, idx.chain, block, inskip, -1)
	if err != nil {
		return err
	}

	adds := blockchain.BlockToAddLeaves(block, outskip, nil, outCount)

	idx.mtx.RLock()
	ud, err := wire.GenerateUData(dels, idx.utreexoState.state)
	idx.mtx.RUnlock()
	if err != nil {
		return err
	}

	// Only store the proofs if the node is not pruned.
	if !idx.pruned {
		err = dbStoreUtreexoProof(dbTx, block.Hash(), ud)
		if err != nil {
			return err
		}
	}

	err = dbStoreUtreexoState(dbTx, block.Hash(), idx.utreexoState.state)
	if err != nil {
		return err
	}

	delHashes := make([]utreexo.Hash, len(ud.LeafDatas))
	for i := range delHashes {
		delHashes[i] = ud.LeafDatas[i].LeafHash()
	}

	err = dbStoreUndoData(dbTx,
		uint64(len(adds)), ud.AccProof.Targets, block.Hash(), delHashes)
	if err != nil {
		return err
	}

	idx.mtx.Lock()
	err = idx.utreexoState.state.Modify(adds, delHashes, ud.AccProof)
	idx.mtx.Unlock()
	if err != nil {
		return err
	}

	return nil
}

// DisconnectBlock is invoked by the index manager when a new block has been
// disconnected to the main chain.
//
// This is part of the Indexer interface.
func (idx *UtreexoProofIndex) DisconnectBlock(dbTx database.Tx, block *btcutil.Block,
	stxos []blockchain.SpentTxOut) error {

	state, err := dbFetchUtreexoState(dbTx, block.Hash())
	if err != nil {
		return err
	}

	numAdds, targets, delHashes, err := dbFetchUndoData(dbTx, block.Hash())
	if err != nil {
		return err
	}

	idx.mtx.Lock()
	err = idx.utreexoState.state.Undo(numAdds, utreexo.Proof{Targets: targets}, delHashes, state.Roots)
	idx.mtx.Unlock()
	if err != nil {
		return err
	}

	err = dbDeleteUtreexoProofEntry(dbTx, block.Hash())
	if err != nil {
		return err
	}

	err = dbDeleteUtreexoState(dbTx, block.Hash())
	if err != nil {
		return err
	}

	return dbDeleteUndoData(dbTx, block.Hash())
}

// FetchUtreexoProof returns the Utreexo proof data for the given block hash.
func (idx *UtreexoProofIndex) FetchUtreexoProof(hash *chainhash.Hash) (*wire.UData, error) {
	ud := new(wire.UData)
	err := idx.db.View(func(dbTx database.Tx) error {
		proofBytes, err := dbFetchUtreexoProofEntry(dbTx, hash)
		if err != nil {
			return err
		}
		r := bytes.NewReader(proofBytes)

		err = ud.DeserializeCompact(r, udataSerializeBool, 0)
		if err != nil {
			return err
		}

		return nil
	})

	return ud, err
}

// GetLeafHashPositions returns the positions of the passed in hashes.
func (idx *UtreexoProofIndex) GetLeafHashPositions(delHashes []utreexo.Hash) []uint64 {
	idx.mtx.RLock()
	defer idx.mtx.RUnlock()

	return idx.utreexoState.state.GetLeafPositions(delHashes)
}

// GenerateUDataPartial generates a utreexo data based on the current state of the accumulator.
// It leaves out the full proof hashes and only fetches the requested positions.
func (idx *UtreexoProofIndex) GenerateUDataPartial(dels []wire.LeafData, positions []uint64) (*wire.UData, error) {
	idx.mtx.RLock()
	defer idx.mtx.RUnlock()

	ud := new(wire.UData)
	ud.LeafDatas = dels

	// Get the positions of the targets of delHashes.
	delHashes, err := wire.HashesFromLeafDatas(ud.LeafDatas)
	if err != nil {
		return nil, err
	}

	hashes := make([]utreexo.Hash, len(positions))
	for i, pos := range positions {
		hashes[i] = idx.utreexoState.state.GetHash(pos)
	}

	targets := idx.utreexoState.state.GetLeafPositions(delHashes)
	ud.AccProof = utreexo.Proof{
		Targets: targets,
		Proof:   hashes,
	}

	return ud, nil
}

// GenerateUData generates utreexo data for the dels passed in.  Height passed in
// should either be of block height of where the deletions are happening or just
// the lastest block height for mempool tx proof generation.
func (idx *UtreexoProofIndex) GenerateUData(dels []wire.LeafData) (*wire.UData, error) {
	idx.mtx.RLock()
	ud, err := wire.GenerateUData(dels, idx.utreexoState.state)
	idx.mtx.RUnlock()
	if err != nil {
		return nil, err
	}

	return ud, nil
}

// ProveUtxos returns an accumulator proof of the outpoints passed in with
// respect to the UTXO state at chaintip.
//
// NOTE The accumulator state differs at every block height.  The caller must
// take into consideration that an accumulator proof at block X will not be valid
// at block height X+1.
//
// This function is safe for concurrent access.
func (idx *UtreexoProofIndex) ProveUtxos(utxos []*blockchain.UtxoEntry,
	outpoints *[]wire.OutPoint) (*blockchain.ChainTipProof, error) {

	// We'll turn the entries and outpoints into leaves that go in
	// the accumulator.
	leaves := make([]wire.LeafData, 0, len(utxos))
	for i, utxo := range utxos {
		if utxo == nil || utxo.IsSpent() {
			err := fmt.Errorf("Passed in utxo at index %d "+
				"is nil or is already spent", i)
			return nil, err
		}

		blockHash, err := idx.chain.BlockHashByHeight(utxo.BlockHeight())
		if err != nil {
			return nil, err
		}
		if blockHash == nil {
			err := fmt.Errorf("Couldn't find blockhash for height %d",
				utxo.BlockHeight())
			return nil, err
		}
		leaf := wire.LeafData{
			BlockHash:  *blockHash,
			OutPoint:   (*outpoints)[i],
			Amount:     utxo.Amount(),
			PkScript:   utxo.PkScript(),
			Height:     utxo.BlockHeight(),
			IsCoinBase: utxo.IsCoinBase(),
		}

		leaves = append(leaves, leaf)
	}

	// Now we'll turn those leaves into hashes.  These are the hashes that are
	// commited in the accumulator.
	hashes := make([]utreexo.Hash, 0, len(leaves))
	for _, leaf := range leaves {
		hashes = append(hashes, leaf.LeafHash())
	}

	// Get a read lock for the index.  This will prevent connectBlock from updating
	// the height and the utreexo state.
	idx.mtx.RLock()
	defer idx.mtx.RUnlock()

	// Prove the commited hashes.
	accProof, err := idx.utreexoState.state.Prove(hashes)
	if err != nil {
		return nil, err
	}

	// Grab the height and the blockhash the proof was generated at.
	snapshot := idx.chain.BestSnapshot()
	provedAtHash := snapshot.Hash

	proof := &blockchain.ChainTipProof{
		ProvedAtHash: &provedAtHash,
		AccProof:     &accProof,
		HashesProven: hashes,
	}

	return proof, nil
}

// VerifyAccProof verifies the given accumulator proof.  Returns an error if the
// verification failed.
func (idx *UtreexoProofIndex) VerifyAccProof(toProve []utreexo.Hash,
	proof *utreexo.Proof) error {
	return idx.utreexoState.state.Verify(toProve, *proof, false)
}

// SetChain sets the given chain as the chain to be used for blockhash fetching.
func (idx *UtreexoProofIndex) SetChain(chain *blockchain.BlockChain) {
	idx.chain = chain
}

// PruneBlock is invoked when an older block is deleted after it's been
// processed.
//
// This is part of the Indexer interface.
func (idx *UtreexoProofIndex) PruneBlock(dbTx database.Tx, blockHash *chainhash.Hash) error {
	//err := dbDeleteUtreexoProofEntry(dbTx, blockHash)
	//if err != nil {
	//	return err
	//}

	//err = dbDeleteUtreexoState(dbTx, blockHash)
	//if err != nil {
	//	return err
	//}

	return nil
}

// NewUtreexoProofIndex returns a new instance of an indexer that is used to create a
//
// It implements the Indexer interface which plugs into the IndexManager that in
// turn is used by the blockchain package.  This allows the index to be
// seamlessly maintained along with the chain.
func NewUtreexoProofIndex(db database.DB, dataDir string, chainParams *chaincfg.Params) (*UtreexoProofIndex, error) {
	idx := &UtreexoProofIndex{
		db:          db,
		chainParams: chainParams,
		mtx:         new(sync.RWMutex),
	}

	uState, err := InitUtreexoState(&UtreexoConfig{
		DataDir: dataDir,
		Name:    db.Type(),
		Params:  chainParams,
	})
	if err != nil {
		return nil, err
	}
	idx.utreexoState = uState
	//idx.pruned = pruned

	return idx, nil
}

// DropUtreexoProofIndex drops the address index from the provided database if it
// exists.
func DropUtreexoProofIndex(db database.DB, dataDir string, interrupt <-chan struct{}) error {
	err := dropIndex(db, utreexoParentBucketKey, utreexoProofIndexName, interrupt)
	if err != nil {
		return err
	}

	path := utreexoBasePath(&UtreexoConfig{DataDir: dataDir, Name: db.Type()})
	return deleteUtreexoState(path)
}

// Stores the utreexo proof in the database.
// TODO Use the compact serialization.
func dbStoreUtreexoProof(dbTx database.Tx, hash *chainhash.Hash, ud *wire.UData) error {
	// Pre-allocated the needed buffer.
	udSize := ud.SerializeSizeCompact(udataSerializeBool)
	buf := bytes.NewBuffer(make([]byte, 0, udSize))

	err := ud.SerializeCompact(buf, udataSerializeBool)
	if err != nil {
		return err
	}

	proofBucket := dbTx.Metadata().Bucket(utreexoParentBucketKey).Bucket(utreexoProofIndexKey)
	return proofBucket.Put(hash[:], buf.Bytes())
}

// Fetches the utreexo proof in the database as a byte slice. The returned byte slice
// is serialized using the compact serialization format.
func dbFetchUtreexoProofEntry(dbTx database.Tx, hash *chainhash.Hash) ([]byte, error) {
	proofBucket := dbTx.Metadata().Bucket(utreexoParentBucketKey).Bucket(utreexoProofIndexKey)
	return proofBucket.Get(hash[:]), nil
}

// Deletes the utreexo proof in the database.
func dbDeleteUtreexoProofEntry(dbTx database.Tx, hash *chainhash.Hash) error {
	idx := dbTx.Metadata().Bucket(utreexoParentBucketKey).Bucket(utreexoProofIndexKey)
	return idx.Delete(hash[:])
}

// Stores the utreexo state in the database.
func dbStoreUtreexoState(dbTx database.Tx, hash *chainhash.Hash, p *utreexo.Pollard) error {
	bytes, err := blockchain.SerializeUtreexoRoots(p.NumLeaves, p.GetRoots())
	if err != nil {
		return err
	}

	stateBucket := dbTx.Metadata().Bucket(utreexoParentBucketKey).Bucket(utreexoStateKey)
	return stateBucket.Put(hash[:], bytes)
}

// Fetches the utreexo state from the database.
func dbFetchUtreexoState(dbTx database.Tx, hash *chainhash.Hash) (utreexo.Stump, error) {
	stateBucket := dbTx.Metadata().Bucket(utreexoParentBucketKey).Bucket(utreexoStateKey)
	serialized := stateBucket.Get(hash[:])

	numLeaves, roots, err := blockchain.DeserializeUtreexoRoots(serialized)
	if err != nil {
		return utreexo.Stump{}, err
	}

	return utreexo.Stump{Roots: roots, NumLeaves: numLeaves}, nil
}

func dbStoreUndoData(dbTx database.Tx, numAdds uint64,
	targets []uint64, blockHash *chainhash.Hash, delHashes []utreexo.Hash) error {

	bytes, err := serializeUndoBlock(numAdds, targets, delHashes)
	if err != nil {
		return err
	}

	undoBucket := dbTx.Metadata().Bucket(utreexoParentBucketKey).Bucket(utreexoUndoKey)
	return undoBucket.Put(blockHash[:], bytes)
}

func dbFetchUndoData(dbTx database.Tx, blockHash *chainhash.Hash) (uint64, []uint64, []utreexo.Hash, error) {
	undoBucket := dbTx.Metadata().Bucket(utreexoParentBucketKey).Bucket(utreexoUndoKey)
	bytes := undoBucket.Get(blockHash[:])

	return deserializeUndoBlock(bytes)
}

func dbDeleteUndoData(dbTx database.Tx, blockHash *chainhash.Hash) error {
	undoBucket := dbTx.Metadata().Bucket(utreexoParentBucketKey).Bucket(utreexoUndoKey)
	return undoBucket.Delete(blockHash[:])
}

// Deletes the utreexo state in the database.
func dbDeleteUtreexoState(dbTx database.Tx, hash *chainhash.Hash) error {
	idx := dbTx.Metadata().Bucket(utreexoParentBucketKey).Bucket(utreexoStateKey)
	return idx.Delete(hash[:])
}

// UtreexoProofIndexInitialized returns true if the cfindex has been created previously.
func UtreexoProofIndexInitialized(db database.DB) bool {
	var exists bool
	db.View(func(dbTx database.Tx) error {
		bucket := dbTx.Metadata().Bucket(utreexoParentBucketKey)
		exists = bucket != nil
		return nil
	})

	return exists
}
