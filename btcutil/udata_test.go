package btcutil

import (
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"

	"github.com/utreexo/utreexo"
	"github.com/utreexo/utreexod/chaincfg/chainhash"
	"github.com/utreexo/utreexod/wire"
)

func TestGenerateUData(t *testing.T) {
	t.Parallel()

	// Creates 15 leaves
	leafCount := 15

	rand := rand.New(rand.NewSource(0))
	leafDatas := make([]wire.LeafData, leafCount)
	for i := range leafDatas {
		// This creates a txo thats not spendable but it's ok for accumulator
		// testing.
		leafVal, ok := quick.Value(reflect.TypeOf(wire.LeafData{}), rand)
		if !ok {
			t.Fatal("Could not create LeafData")
		}
		ld := leafVal.Interface().(wire.LeafData)

		blockHashVal, ok := quick.Value(reflect.TypeOf(chainhash.Hash{}), rand)
		if !ok {
			t.Fatal("Could not create OutPoint")
		}
		bh := blockHashVal.Interface().(chainhash.Hash)
		ld.BlockHash = bh
		leafDatas[i] = ld
	}

	// Hash the leafData so that it can be added to the accumulator.
	addLeaves := make([]utreexo.Leaf, leafCount)
	for i := range addLeaves {
		addLeaves[i] = utreexo.Leaf{
			Hash: leafDatas[i].LeafHash(),
		}
	}

	p := utreexo.NewAccumulator()
	err := p.Modify(addLeaves, nil, utreexo.Proof{})
	if err != nil {
		t.Fatal(err)
	}

	delCount := 2
	firstDelIdx := 4
	secondDelIdx := 10

	delLeaves := make([]wire.LeafData, delCount)
	delLeaves[0] = leafDatas[firstDelIdx]
	delLeaves[1] = leafDatas[secondDelIdx]

	ud, err := GenerateUData(delLeaves, &p)
	if err != nil {
		t.Fatal(err)
	}

	delHashes := make([]utreexo.Hash, delCount)
	delHashes[0] = leafDatas[firstDelIdx].LeafHash()
	delHashes[1] = leafDatas[secondDelIdx].LeafHash()

	// Test if the UData actually validates
	err = p.Verify(delHashes, ud.AccProof, false)
	if err != nil {
		t.Errorf("Generated UData not verifiable")
	}

	// Use the udata.
	err = p.Modify(nil, delHashes, ud.AccProof)
	if err != nil {
		t.Fatal(err)
	}
}
