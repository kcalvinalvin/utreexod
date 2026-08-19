// Copyright (c) 2026 The utreexod developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package ffldb

import (
	"bytes"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/utreexo/utreexod/database"
)

// TestProofRolloverDoesNotOverwriteExistingProof ensures proof storage never
// rolls to a different file number independently of its corresponding block.
// It also ensures returning to an earlier proof file cannot make a subsequent
// write overwrite a proof in a later file.
func TestProofRolloverDoesNotOverwriteExistingProof(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "ffldb-proof-rollover")
	idb, err := database.Create(dbType, dbPath, blockDataNet)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer idb.Close()

	blocks, err := loadBlocks(t, blockDataFile, blockDataNet)
	if err != nil {
		t.Fatalf("loadBlocks: %v", err)
	}

	// Keep all four blocks in block file 0 while setting a proof file limit
	// that would independently roll over after a single 60-byte proof.  Each
	// proof record has 12 bytes of framing, so the records below occupy 72,
	// 72, 22, and 72 bytes.  Proof partitioning must follow the block files
	// despite that nominal proof file limit.
	pdb := idb.(*db)
	pdb.proofStore.maxBlockFileSize = 100
	proofs := [][]byte{
		bytes.Repeat([]byte{0x11}, 60),
		bytes.Repeat([]byte{0x22}, 60),
		bytes.Repeat([]byte{0x33}, 10),
		bytes.Repeat([]byte{0x44}, 60),
	}

	err = idb.Update(func(tx database.Tx) error {
		for i, proof := range proofs {
			block := blocks[i+1]
			if err := tx.StoreBlock(block); err != nil {
				return err
			}
			if err := tx.StoreUtreexoProof(block.Hash(), proof); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("store blocks and proofs: %v", err)
	}

	err = idb.View(func(tx database.Tx) error {
		blockIdx := tx.Metadata().Bucket(blockIdxBucketName)
		if blockIdx == nil {
			return fmt.Errorf("block index bucket missing")
		}
		proofIdx := tx.Metadata().Bucket(proofIdxBucketName)
		if proofIdx == nil {
			return fmt.Errorf("proof index bucket missing")
		}

		for i, want := range proofs {
			hash := blocks[i+1].Hash()
			blockRow := blockIdx.Get(hash[:])
			if blockRow == nil {
				return fmt.Errorf("block index row %d missing", i)
			}
			proofRow := proofIdx.Get(hash[:])
			if proofRow == nil {
				return fmt.Errorf("proof index row %d missing", i)
			}

			blockLoc := deserializeBlockLoc(blockRow)
			proofLoc := deserializeBlockLoc(proofRow)
			if proofLoc.blockFileNum != blockLoc.blockFileNum {
				t.Errorf("proof %d stored in file %d, want corresponding block file %d",
					i, proofLoc.blockFileNum, blockLoc.blockFileNum)
			}

			got, err := tx.FetchUtreexoProof(hash)
			if err != nil {
				return err
			}
			if !bytes.Equal(got, want) {
				t.Errorf("proof %d was overwritten: got %x, want %x",
					i, got, want)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("fetch proofs: %v", err)
	}
}
