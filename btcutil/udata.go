// Copyright (c) 2024 The utreexo developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package btcutil

import (
	"encoding/hex"
	"fmt"

	"github.com/utreexo/utreexo"
	"github.com/utreexo/utreexod/wire"
)

// UData contains data needed to prove the existence and validity of all inputs
// for a Bitcoin block.  With this data, a full node may only keep the utreexo
// roots and still be able to fully validate a block.
type UData struct {
	// AccProof is the utreexo accumulator proof for all the inputs.
	AccProof utreexo.Proof

	// LeafDatas are the tx validation data for every input.
	LeafDatas []wire.LeafData
}

// StxosHashes returns the hash of all stxos in this UData.  The hashes returned
// here represent the hash commitments of the stxos.
func (ud *UData) StxoHashes() []utreexo.Hash {
	leafHashes := make([]utreexo.Hash, len(ud.LeafDatas))
	for i, stxo := range ud.LeafDatas {
		leafHashes[i] = stxo.LeafHash()
	}

	return leafHashes
}

// NewUData returns a UData initialized from the arguments passed in.
func NewUData(targets []uint64, hashes []utreexo.Hash, leaves []wire.LeafData) UData {
	return UData{
		AccProof: utreexo.Proof{
			Targets: targets,
			Proof:   hashes,
		},
		LeafDatas: leaves,
	}
}

// HashesFromLeafDatas hashes the passed in leaf datas. Returns an error if a
// leaf data is compact as you can't generate the correct hash.
func HashesFromLeafDatas(leafDatas []wire.LeafData) ([]utreexo.Hash, error) {
	// make slice of hashes from leafdata
	delHashes := make([]utreexo.Hash, 0, len(leafDatas))
	for _, ld := range leafDatas {
		// We can't calculate the correct hash if the leaf data is in
		// the compact state.
		if ld.IsCompact() {
			return nil, fmt.Errorf("leafdata is compact. Unable " +
				"to generate a leafhash")
		}

		delHashes = append(delHashes, ld.LeafHash())
	}

	return delHashes, nil
}

// GenerateUData creates a block proof, calling forest.ProveBatch with the leaf indexes
// to get a batched inclusion proof from the accumulator. It then adds on the leaf data,
// to create a block proof which both proves inclusion and gives all utxo data
// needed for transaction verification.
func GenerateUData(txIns []wire.LeafData, pollard utreexo.Utreexo) (
	*UData, error) {

	ud := new(UData)
	ud.LeafDatas = txIns

	// Make a slice of hashes from the leafdatas.
	delHashes, err := HashesFromLeafDatas(ud.LeafDatas)
	if err != nil {
		return nil, err
	}

	// Generate the utreexo accumulator proof for all the inputs.
	ud.AccProof, err = pollard.Prove(delHashes)
	if err != nil {
		// Find out which exact one is causing the error.
		for i, delHash := range delHashes {
			_, err = pollard.Prove([]utreexo.Hash{delHash})
			if err != nil {
				ld := ud.LeafDatas[i]
				return nil,
					fmt.Errorf("LeafData hash %s couldn't be proven. "+
						"BlockHash %s, Outpoint %s, height %v, "+
						"IsCoinbase %v, Amount %v, PkScript %s. "+
						"err: %s",
						hex.EncodeToString(delHash[:]),
						ld.BlockHash.String(), ld.OutPoint.String(),
						ld.Height, ld.IsCoinBase, ld.Amount,
						hex.EncodeToString(ld.PkScript), err.Error())
			}
		}
		return nil, err
	}

	return ud, nil
}
