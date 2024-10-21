// Copyright (c) 2021 The utreexo developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/utreexo/utreexo"
)

type testData struct {
	name           string
	height         int32
	leavesPerBlock []LeafData

	size        int
	sizeCompact int
}

func getTestDatas() []testData {
	return []testData{mainNetBlock104773, testNetBlock383}
}

var mainNetBlock104773 = testData{
	name:   "Mainnet block 104773",
	height: 104773,
	leavesPerBlock: []LeafData{
		{
			BlockHash: *newHashFromStr("000000000002bc1ddaae8ef976adf1c36db878b5f0711ec58c92ec0e4724277b"),
			OutPoint: OutPoint{
				Hash:  *newHashFromStr("43263e398303de72f5b8f5dd690c88cd87c31ec7c73cc98a567a4b73521428ea"),
				Index: 0,
			},
			Amount:     29865000000,
			PkScript:   hexToBytes("76a9147ac5cfe778bc4e65d8fa86f80caeb47b1f6303a988ac"),
			Height:     104766,
			IsCoinBase: false,
		},
		{
			BlockHash: *newHashFromStr("0000000000021ecac6ea6e14d61821b3ddcb8f4563c796957394e4181c261b4d"),
			OutPoint: OutPoint{
				Hash:  *newHashFromStr("76c131357f1efc87434b3de49f9cf2660acaad5f360205ba390cb8726c01c948"),
				Index: 0,
			},
			Amount:     2586000000,
			PkScript:   hexToBytes("76a914f303158d2894dbe996e9dc1f26798796716c9bf588ac"),
			Height:     104768,
			IsCoinBase: false,
		},
	},
	size:        214,
	sizeCompact: 80,
}

var testNetBlock383 = testData{
	name:   "Testnet block 383",
	height: 383,
	leavesPerBlock: []LeafData{
		{
			BlockHash: *newHashFromStr("00000000ff41b51f43141f3fd198016cead8c92355f7064849c4507f9e8914f8"),
			OutPoint: OutPoint{
				Hash:  *newHashFromStr("58102e32e848fbd68c29480de00d653a88a6de077c46d8f6c37488290f2b4d43"),
				Index: 0,
			},
			Amount:     5000000000,
			PkScript:   hexToBytes("210263ee71bdafe3250552cf9fb0c1734072758fff5c7b9f0b1a045ee91461fdeb87ac"),
			Height:     151,
			IsCoinBase: true,
		},
		{
			BlockHash: *newHashFromStr("000000004a0cd08dbda8e47cbab13205ba9ae2f3e4b157c6b2539446db44aae9"),
			OutPoint: OutPoint{
				Hash:  *newHashFromStr("013e22e413cdf3e80eca36c058f0a31ac00ebcfbf547fa6a5688b5626d1739e7"),
				Index: 0,
			},
			Amount:     5000000000,
			PkScript:   hexToBytes("2102fac1c1962818c784ed4be71611986fdb06c19577d410f4447aa9c8e705983609ac"),
			Height:     241,
			IsCoinBase: true,
		},
		{
			BlockHash: *newHashFromStr("000000001a4c2c64beded987790ab0c00675b4bc467cd3574ad455b1397c967c"),
			OutPoint: OutPoint{
				Hash:  *newHashFromStr("7e621eeb02874ab039a8566fd36f4591e65eca65313875221842c53de6907d6c"),
				Index: 0,
			},
			Amount:     4989000000,
			PkScript:   hexToBytes("76a914944a7d4b3a8d3a5ecf19dfdfd8dcc18c6f1487dd88ac"),
			Height:     381,
			IsCoinBase: false,
		},
		{
			BlockHash: *newHashFromStr("0000000092907b867c2871a75a70de6d5e39c697eac57555a3896c19321c75b8"),
			OutPoint: OutPoint{
				Hash:  *newHashFromStr("6a2ea57b544fce1e36eafec6543486e3d49f66295ddc11f3ec2276295bf8eeaa"),
				Index: 0,
			},
			Amount:     5000000000,
			PkScript:   hexToBytes("2103aba3696c249664d96c9fe7e09d31010071189c00995d9573026aeb57ee18e142ac"),
			Height:     237,
			IsCoinBase: true,
		},
	},
	size:        456,
	sizeCompact: 188,
}

func checkUDEqual(ud, checkUData *UData, isCompact bool, name string) error {
	for i := range ud.ProofHashes {
		if ud.ProofHashes[i] != checkUData.ProofHashes[i] {
			return fmt.Errorf("%s: UData.AccProof Target mismatch. expect %v, got %v",
				name, ud.ProofHashes[i], checkUData.ProofHashes[i])
		}
	}

	if len(ud.LeafDatas) != len(checkUData.LeafDatas) {
		return fmt.Errorf("%s: LeafData length mismatch. expect %v, got %v",
			name, len(ud.LeafDatas), len(checkUData.LeafDatas))
	}

	for i := range ud.LeafDatas {
		leaf := ud.LeafDatas[i]
		checkLeaf := checkUData.LeafDatas[i]

		if !isCompact {
			if leaf.BlockHash != checkLeaf.BlockHash {
				return fmt.Errorf("%s: LeafData blockhash mismatch. expect %v, got %v",
					name, hex.EncodeToString(leaf.BlockHash[:]),
					hex.EncodeToString(checkLeaf.BlockHash[:]))
			}
			if leaf.OutPoint.Hash != checkLeaf.OutPoint.Hash {
				return fmt.Errorf("%s: LeafData outpoint hash mismatch. expect %v, got %v",
					name, hex.EncodeToString(leaf.OutPoint.Hash[:]),
					hex.EncodeToString(checkLeaf.OutPoint.Hash[:]))
			}
			if leaf.OutPoint.Index != checkLeaf.OutPoint.Index {
				return fmt.Errorf("%s: LeafData outpoint index mismatch. expect %v, got %v",
					name, leaf.OutPoint.Index, checkLeaf.OutPoint.Index)
			}
		}

		// Only amount, hcb, and pkscript is serialized with the compact serialization.
		if leaf.Amount != checkLeaf.Amount {
			return fmt.Errorf("%s: LeafData amount mismatch. expect %v, got %v",
				name, leaf.Amount, checkLeaf.Amount)
		}

		if leaf.IsCoinBase != checkLeaf.IsCoinBase {
			return fmt.Errorf("%s: LeafData IsCoinBase mismatch. expect %v, got %v",
				name, leaf.IsCoinBase, checkLeaf.IsCoinBase)
		}

		if leaf.Height != checkLeaf.Height {
			return fmt.Errorf("%s: LeafData height mismatch. expect %v, got %v",
				name, leaf.Height, checkLeaf.Height)
		}

		if !bytes.Equal(leaf.PkScript[:], checkLeaf.PkScript[:]) {
			return fmt.Errorf("%s: LeafData pkscript mismatch. expect %x, got %x",
				name, leaf.PkScript, checkLeaf.PkScript)
		}
	}

	return nil
}

// HashesFromLeafDatas hashes the passed in leaf datas. Returns an error if a
// leaf data is compact as you can't generate the correct hash.
func HashesFromLeafDatas(leafDatas []LeafData) ([]utreexo.Hash, error) {
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

// generateUData creates a block proof, calling forest.ProveBatch with the leaf indexes
// to get a batched inclusion proof from the accumulator. It then adds on the leaf data,
// to create a block proof which both proves inclusion and gives all utxo data
// needed for transaction verification.
func generateUData(txIns []LeafData, pollard utreexo.Utreexo) (
	*UData, error) {

	ud := new(UData)
	ud.LeafDatas = txIns

	// Make a slice of hashes from the leafdatas.
	delHashes, err := HashesFromLeafDatas(ud.LeafDatas)
	if err != nil {
		return nil, err
	}

	// Generate the utreexo accumulator proof for all the inputs.
	proof, err := pollard.Prove(delHashes)
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

	ud.ProofHashes = proof.Proof

	return ud, nil
}

func TestUDataSerializeSize(t *testing.T) {
	t.Parallel()

	type test struct {
		name             string
		ud               UData
		size             int
		sizeCompact      int
		sizeCompactNoAcc int
	}

	testDatas := getTestDatas()
	tests := make([]test, 0, len(testDatas))

	for _, testData := range testDatas {
		// New forest object.
		p := utreexo.NewAccumulator()

		// Create hashes to add from the stxo data.
		addHashes := make([]utreexo.Leaf, 0, len(testData.leavesPerBlock))
		for i, ld := range testData.leavesPerBlock {
			addHashes = append(addHashes, utreexo.Leaf{
				Hash: ld.LeafHash(),
				// Just half and half.
				Remember: i%2 == 0,
			})
		}
		// Add to the accumulator.
		err := p.Modify(addHashes, nil, utreexo.Proof{})
		if err != nil {
			t.Fatal(err)
		}

		// Generate Proof.
		ud, err := generateUData(testData.leavesPerBlock, &p)
		if err != nil {
			t.Fatal(err)
		}

		// Append to the tests.
		tests = append(tests, test{
			name:        testData.name,
			ud:          *ud,
			size:        testData.size,
			sizeCompact: testData.sizeCompact,
		})
	}

	for _, test := range tests {
		gotSize := test.ud.SerializeSize()
		if gotSize != test.size {
			t.Fatalf("%s: UData serialize size fail. "+
				"expect %d, got %d", test.name,
				test.size, gotSize)
			continue
		}

		// Sanity check.  Actually serialize the data and compare against our hardcoded number.
		var buf bytes.Buffer
		err := test.ud.Serialize(&buf)
		if err != nil {
			t.Fatal(err)
		}
		if len(buf.Bytes()) != test.size {
			t.Errorf("%s: UData serialize size fail. "+
				"serialized %d, hardcoded %d", test.name,
				len(buf.Bytes()), test.size)
			continue
		}

		gotSize = test.ud.SerializeSizeCompact()
		if gotSize != test.sizeCompact {
			var buf bytes.Buffer
			err := test.ud.SerializeCompact(&buf)
			if err != nil {
				t.Fatal(err)
			}

			t.Errorf("%s: UData serialize size compact (false) fail. "+
				"expect %d, got %d", test.name,
				test.sizeCompact, gotSize)
			continue
		}

		// Sanity check.  Actually serialize the data and compare against our hardcoded number.
		buf.Reset()
		err = test.ud.SerializeCompact(&buf)
		if err != nil {
			t.Fatal(err)
		}
		if len(buf.Bytes()) != test.sizeCompact {
			t.Errorf("%s: UData serialize size compact(false) fail. "+
				"serialized %d, hardcoded %d", test.name,
				len(buf.Bytes()), test.sizeCompact)
			continue
		}

		// Sanity check.  Actually serialize the data and compare against our hardcoded number.
		buf.Reset()
		err = test.ud.SerializeCompact(&buf)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestUDataSerialize(t *testing.T) {
	t.Parallel()

	type test struct {
		name   string
		ud     UData
		before []byte
		after  []byte
	}

	testDatas := getTestDatas()
	tests := make([]test, 0, len(testDatas))

	for _, testData := range testDatas {
		// New forest object.
		p := utreexo.NewAccumulator()

		// Create hashes to add from the stxo data.
		addHashes := make([]utreexo.Leaf, 0, len(testData.leavesPerBlock))
		for i, ld := range testData.leavesPerBlock {
			add := utreexo.Leaf{Hash: ld.LeafHash(), Remember: i%2 == 0}
			addHashes = append(addHashes, add)
		}

		// Add to the accumulator.
		err := p.Modify(addHashes, nil, utreexo.Proof{})
		if err != nil {
			t.Fatal(err)
		}

		// Generate Proof.
		ud, err := generateUData(testData.leavesPerBlock, &p)
		if err != nil {
			t.Fatal(err)
		}

		// Append to the tests.
		tests = append(tests, test{name: testData.name, ud: *ud})
	}

	for _, test := range tests {
		// Serialize
		writer := &bytes.Buffer{}
		test.ud.Serialize(writer)
		test.before = writer.Bytes()

		// Deserialize
		checkUData := new(UData)
		checkUData.Deserialize(writer)

		err := checkUDEqual(&test.ud, checkUData, false, test.name)
		if err != nil {
			t.Error(err)
		}

		// Re-serialize
		afterWriter := &bytes.Buffer{}
		checkUData.Serialize(afterWriter)
		test.after = afterWriter.Bytes()

		// Check if before and after match.
		if !bytes.Equal(test.before, test.after) {
			t.Errorf("%s: UData serialize/deserialize fail. "+
				"Before len %d, after len %d", test.name,
				len(test.before), len(test.after))
		}
	}
}

func TestUDataSerializeCompact(t *testing.T) {
	t.Parallel()

	type test struct {
		name   string
		ud     UData
		before []byte
		after  []byte
	}

	testDatas := getTestDatas()
	tests := make([]test, 0, len(testDatas))

	for _, testData := range testDatas {
		// New forest object.
		p := utreexo.NewAccumulator()

		// Create hashes to add from the stxo data.
		addHashes := make([]utreexo.Leaf, 0, len(testData.leavesPerBlock))
		for i, ld := range testData.leavesPerBlock {
			addHashes = append(addHashes, utreexo.Leaf{
				Hash: ld.LeafHash(),
				// Just half and half.
				Remember: i%2 == 0,
			})
		}
		// Add to the accumulator.
		err := p.Modify(addHashes, nil, utreexo.Proof{})
		if err != nil {
			t.Fatal(err)
		}

		// Generate Proof.
		ud, err := generateUData(testData.leavesPerBlock, &p)
		if err != nil {
			t.Fatal(err)
		}

		// Append to the tests.
		tests = append(tests, test{
			name: testData.name,
			ud:   *ud,
		})
	}

	for _, test := range tests {
		// Serialize
		writer := &bytes.Buffer{}
		test.ud.SerializeCompact(writer)
		test.before = writer.Bytes()

		// Deserialize
		checkUData := new(UData)
		err := checkUData.DeserializeCompact(writer)
		if err != nil {
			t.Fatal(err)
		}

		err = checkUDEqual(&test.ud, checkUData, true, test.name)
		if err != nil {
			t.Error(err)
		}

		// Re-serialize
		afterWriter := &bytes.Buffer{}
		checkUData.SerializeCompact(afterWriter)
		test.after = afterWriter.Bytes()

		// Check if before and after match.
		if !bytes.Equal(test.before, test.after) {
			t.Errorf("%s: UData serialize/deserialize fail. "+
				"Before len %d, after len %d", test.name,
				len(test.before), len(test.after))
		}
	}
}
