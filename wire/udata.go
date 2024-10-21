// Copyright (c) 2021 The utreexo developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"fmt"
	"io"

	"github.com/utreexo/utreexo"
	"github.com/utreexo/utreexod/chaincfg/chainhash"
)

// UData contains data sent over the wire from other utreexo peers to prove the
// existence and validity of all inputs for a Bitcoin block.  With this data,
// a full node may only keep the utreexo roots and still be able to fully validate a block.
type UData struct {
	// ProofHashes are the hashes needed to hash up to the merkle roots.
	ProofHashes []utreexo.Hash

	// LeafDatas are the tx validation data for every input.
	LeafDatas []LeafData
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

// SerializeUtxoDataSize returns the number of bytes it would take to serialize the
// utxo data size.
func (ud *UData) SerializeUtxoDataSize() int {
	size := VarIntSerializeSize(uint64(len(ud.LeafDatas)))
	for _, l := range ud.LeafDatas {
		size += l.SerializeSize()
	}

	return size
}

// SerializedHashSizes returns how many bytes it would take to serialize the
// hashes in the udata.
func SerializedHashSizes(hashes []utreexo.Hash) int {
	size := VarIntSerializeSize(uint64(len(hashes)))
	size += chainhash.HashSize * len(hashes)
	return size
}

// SerializeSize returns the number of bytes it would take to serialize the
// UData.
func (ud *UData) SerializeSize() int {
	// Leaf data size.
	return SerializedHashSizes(ud.ProofHashes) + ud.SerializeUtxoDataSize()
}

// -----------------------------------------------------------------------------
// UData serialization includes all the data that is needed for a utreexo node to
// verify a block or a tx with only the utreexo roots.
//
// The serialized format is:
// [<accumulator proof><leaf datas>]
//
// Accumulator proof serialization follows the batchproof serialization found
// in wire/batchproof.go.
//
// LeafData serialization can be found in wire/leaf.go.
//
// All together, the serialization looks like so:
//
// Field                    Type       Size
// accumulator proof        []byte     variable
// leaf datas               []byte     variable
//
// -----------------------------------------------------------------------------

// Serialize encodes the UData to w using the UData serialization format.
func (ud *UData) Serialize(w io.Writer) error {
	err := WriteVarInt(w, 0, uint64(len(ud.ProofHashes)))
	if err != nil {
		return err
	}

	for _, h := range ud.ProofHashes {
		_, err = w.Write(h[:])
		if err != nil {
			return err
		}
	}

	// Write the size of the leaf datas.
	err = WriteVarInt(w, 0, uint64(len(ud.LeafDatas)))
	if err != nil {
		return err
	}

	// Write the actual leaf datas.
	for _, ld := range ud.LeafDatas {
		err = ld.Serialize(w)
		if err != nil {
			return err
		}
	}

	return nil
}

// Deserialize encodes the UData to w using the UData serialization format.
func (ud *UData) Deserialize(r io.Reader) error {
	proofCount, err := ReadVarInt(r, 0)
	if err != nil {
		return err
	}

	proofs := make([]utreexo.Hash, proofCount)
	for i := range proofs {
		_, err = io.ReadFull(r, proofs[i][:])
		if err != nil {
			return err
		}
	}

	udCount, err := ReadVarInt(r, 0)
	if err != nil {
		return err
	}

	ud.LeafDatas = make([]LeafData, udCount)
	for i := range ud.LeafDatas {
		err = ud.LeafDatas[i].Deserialize(r)
		if err != nil {
			str := fmt.Sprintf("Stxos[%d], err:%s\n",
				i, err.Error())
			returnErr := messageError("Deserialize stxos", str)
			return returnErr
		}
	}

	return nil
}

// -----------------------------------------------------------------------------
// UData compact serialization includes only the data that is missing for a
// utreexo node to verify a block or a tx with only the utreexo roots.  The
// compact serialization leaves out data that is able to be fetched locally
// by a node.
//
// The serialized format is:
// [<accumulator proof><leaf datas>]
//
// Accumulator proof serialization follows the batchproof serialization found
// in wire/batchproof.go.
//
// Compact LeafData serialization can be found in wire/leaf.go.
//
// All together, the serialization looks like so:
//
// Field                    Type       Size
// accumulator proof        []byte     variable
// leaf datas               []byte     variable
//
// -----------------------------------------------------------------------------

// SerializeUxtoDataSizeCompact returns the number of bytes it would take to serialize
// the utxo data and the remember idx data with the compact serialization format.
func (ud *UData) SerializeUxtoDataSizeCompact() int {
	var size int

	// Explicitly serialize the count for the leaf datas.
	size += VarIntSerializeSize(uint64(len(ud.LeafDatas)))
	for _, l := range ud.LeafDatas {
		size += l.SerializeSizeCompact()
	}

	return size
}

// SerializeSizeCompact returns the number of bytes it would take to serialize the
// UData using the compact UData serialization format.
func (ud *UData) SerializeSizeCompact() int {
	// Accumulator hash proof size + leaf data size
	return SerializedHashSizes(ud.ProofHashes) + ud.SerializeUxtoDataSizeCompact()
}

// SerializeCompact encodes the UData to w using the compact UData
// serialization format.  It follows the normal UData serialization format with
// the exception that compact leaf data serialization is used.  Everything else
// remains the same.
func (ud *UData) SerializeCompact(w io.Writer) error {
	err := WriteVarInt(w, 0, uint64(len(ud.ProofHashes)))
	if err != nil {
		return err
	}

	for _, h := range ud.ProofHashes {
		_, err = w.Write(h[:])
		if err != nil {
			return err
		}
	}

	err = WriteVarInt(w, 0, uint64(len(ud.LeafDatas)))
	if err != nil {
		return err
	}

	// Write all the leafDatas.
	for _, ld := range ud.LeafDatas {
		err = ld.SerializeCompact(w)
		if err != nil {
			return err
		}
	}

	return nil
}

// DeserializeCompact decodes the UData from r using the compact UData
// serialization format.
//
// NOTE if deserializing for a transaction, a non zero txInCount MUST be passed
// in as a correct txCount is critical for deserializing correctly.  When
// deserializing a block, txInCount does not matter.
func (ud *UData) DeserializeCompact(r io.Reader) error {
	proofCount, err := ReadVarInt(r, 0)
	if err != nil {
		return err
	}

	proofs := make([]utreexo.Hash, proofCount)
	for i := range proofs {
		_, err = io.ReadFull(r, proofs[i][:])
		if err != nil {
			return err
		}
	}

	// Grab the count for the udatas
	udCount, err := ReadVarInt(r, 0)
	if err != nil {
		return err
	}
	ud.LeafDatas = make([]LeafData, udCount)

	for i := range ud.LeafDatas {
		err = ud.LeafDatas[i].DeserializeCompact(r)
		if err != nil {
			str := fmt.Sprintf("LeafDatas[%d], err:%s\n",
				i, err.Error())
			returnErr := messageError("Deserialize leaf datas", str)
			return returnErr
		}
	}

	return nil
}
