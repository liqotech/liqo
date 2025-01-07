// Copyright 2019-2025 The Liqo Authors
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

package ipamcore

import (
	"fmt"
	"net/netip"
	"strings"

	"k8s.io/apimachinery/pkg/util/runtime"
)

// convertByteSliceToString converts a slice of bytes to a comma-separated string.
func convertByteSliceToString(byteSlice []byte) string {
	strSlice := make([]string, len(byteSlice))
	for i, b := range byteSlice {
		strSlice[i] = fmt.Sprintf("%d", b)
	}
	return strings.Join(strSlice, ".")
}

// setBit sets the bit at the given position to 1.
func setBit(b byte, position int) (byte, error) {
	if position > 7 || position < 0 {
		return b, fmt.Errorf("bit position out of range")
	}
	return b | (1 << (7 - position)), nil
}

func checkHostBitsZero(prefix netip.Prefix) error {
	if prefix.Masked().Addr().Compare(prefix.Addr()) != 0 {
		return fmt.Errorf("%s :host bits must be zero", prefix)
	}
	return nil
}

// splitNetworkPrefix splits a network prefix into two subnets.
// It increases the prefix length by one and sets the bit at
// the new position to 0 or 1 to retrieve the two subnets.
func splitNetworkPrefix(prefix netip.Prefix) (left, right netip.Prefix) {
	// We neer to check that the host bits are zero.
	runtime.Must(checkHostBitsZero(prefix))

	// We need to convert the prefix to a byte slice to manipulate it.
	bin, err := prefix.MarshalBinary()
	runtime.Must(err)

	// We need to get the mask length to know where to split the prefix.
	maskLen := bin[len(bin)-1]

	// Since the prefix host bits are zero, we just need to shift
	// the mask length by one to get the first splitted prefix.
	left = netip.MustParsePrefix(
		fmt.Sprintf("%s/%d", convertByteSliceToString(bin[:4]), maskLen+1),
	)

	// We need to set the bit at the mask length position to 1 to get the second splitted prefix.
	// Since the IP is expressed like a slice of bytes, we need to get the byte index and the bit index to set the bit.
	byteIndex := maskLen / 8
	bitIndex := maskLen % 8

	// We set the bit at the mask length position to 1.
	bin[byteIndex], err = setBit(bin[byteIndex], int(bitIndex))
	runtime.Must(err)

	// We forge and return the second splitted prefix.
	right = netip.MustParsePrefix(
		fmt.Sprintf("%s/%d", convertByteSliceToString(bin[:4]), maskLen+1),
	)

	return left, right
}

// isPrefixChildOf checks if the child prefix is a child of the parent prefix.
func isPrefixChildOf(parent, child netip.Prefix) bool {
	if parent.Bits() <= child.Bits() && parent.Overlaps(child) {
		return true
	}
	return false
}
