// Copyright 2021 Dolthub, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package message

import (
	"context"
	"encoding/binary"

	fb "github.com/google/flatbuffers/go"

	"github.com/dolthub/dolt/go/gen/fb/serial"
	"github.com/dolthub/dolt/go/store/hash"
	"github.com/dolthub/dolt/go/store/pool"
)

const (
	// This constant is mirrored from serial.AddressMap.KeyOffsetsLength()
	// It is only as stable as the flatbuffers schema that defines it.
	addressMapKeyOffsetsVOffset = 6
)

var addressMapFileID = []byte(serial.AddressMapFileID)

type AddressMapSerializer struct {
	Pool pool.BuffPool
}

var _ Serializer = AddressMapSerializer{}

func (s AddressMapSerializer) Serialize(keys, addrs [][]byte, subtrees []uint64, level int) serial.Message {
	var (
		keyArr, keyOffs  fb.UOffsetT
		addrArr, cardArr fb.UOffsetT
	)

	keySz, addrSz, totalSz := estimateAddressMapSize(keys, addrs, subtrees)
	b := getFlatbufferBuilder(s.Pool, totalSz)

	// keys
	keyArr = writeItemBytes(b, keys, keySz)
	serial.AddressMapStartKeyOffsetsVector(b, len(keys)+1)
	keyOffs = writeItemOffsets(b, keys, keySz)

	// addresses
	addrArr = writeItemBytes(b, addrs, addrSz)

	// subtree cardinalities
	if level > 0 {
		cardArr = writeCountArray(b, subtrees)
	}

	serial.AddressMapStart(b)
	serial.AddressMapAddKeyItems(b, keyArr)
	serial.AddressMapAddKeyOffsets(b, keyOffs)
	serial.AddressMapAddAddressArray(b, addrArr)

	if level > 0 {
		serial.AddressMapAddSubtreeCounts(b, cardArr)
		serial.AddressMapAddTreeCount(b, sumSubtrees(subtrees))
	} else {
		serial.AddressMapAddTreeCount(b, uint64(len(keys)))
	}
	serial.AddressMapAddTreeLevel(b, uint8(level))

	return serial.FinishMessage(b, serial.AddressMapEnd(b), addressMapFileID)
}

func getAddressMapKeys(msg serial.Message) (keys ItemArray) {
	am := serial.GetRootAsAddressMap(msg, serial.MessagePrefixSz)
	keys.Items = am.KeyItemsBytes()
	keys.Offs = getAddressMapKeyOffsets(am)
	return
}

func getAddressMapValues(msg serial.Message) (values ItemArray) {
	am := serial.GetRootAsAddressMap(msg, serial.MessagePrefixSz)
	values.Items = am.AddressArrayBytes()
	values.Offs = offsetsForAddressArray(values.Items)
	return
}

func walkAddressMapAddresses(ctx context.Context, msg serial.Message, cb func(ctx context.Context, addr hash.Hash) error) error {
	am := serial.GetRootAsAddressMap(msg, serial.MessagePrefixSz)
	arr := am.AddressArrayBytes()
	for i := 0; i < len(arr)/hash.ByteLen; i++ {
		addr := hash.New(arr[i*addrSize : (i+1)*addrSize])
		if err := cb(ctx, addr); err != nil {
			return err
		}
	}
	return nil
}

func getAddressMapCount(msg serial.Message) uint16 {
	am := serial.GetRootAsAddressMap(msg, serial.MessagePrefixSz)
	return uint16(am.KeyOffsetsLength() - 1)
}

func getAddressMapTreeLevel(msg serial.Message) int {
	am := serial.GetRootAsAddressMap(msg, serial.MessagePrefixSz)
	return int(am.TreeLevel())
}

func getAddressMapTreeCount(msg serial.Message) int {
	am := serial.GetRootAsAddressMap(msg, serial.MessagePrefixSz)
	return int(am.TreeCount())
}

func getAddressMapSubtrees(msg serial.Message) []uint64 {
	counts := make([]uint64, getAddressMapCount(msg))
	am := serial.GetRootAsAddressMap(msg, serial.MessagePrefixSz)
	return decodeVarints(am.SubtreeCountsBytes(), counts)
}

func getAddressMapKeyOffsets(pm *serial.AddressMap) []byte {
	sz := pm.KeyOffsetsLength() * 2
	tab := pm.Table()
	vec := tab.Offset(addressMapKeyOffsetsVOffset)
	start := int(tab.Vector(fb.UOffsetT(vec)))
	stop := start + sz
	return tab.Bytes[start:stop]
}

func estimateAddressMapSize(keys, addresses [][]byte, subtrees []uint64) (keySz, addrSz, totalSz int) {
	assertTrue(len(keys) == len(addresses))
	for i := range keys {
		keySz += len(keys[i])
		addrSz += len(addresses[i])
	}
	totalSz += keySz + addrSz
	totalSz += len(keys) * uint16Size
	totalSz += len(subtrees) * binary.MaxVarintLen64
	totalSz += 8 + 1 + 1 + 1
	totalSz += 72
	totalSz += serial.MessagePrefixSz
	return
}
