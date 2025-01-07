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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Ipam low level utilities ", func() {
	Context("bit operations", func() {
		When("setting bit in byte", func() {
			It("should return 0", func() {
				bytes := []byte{
					0b10000000,
					0b01000000,
					0b00100000,
					0b00010000,
					0b00001000,
					0b00000100,
					0b00000010,
					0b00000001,
				}

				var b byte = 0b00000000
				for i := range bytes {
					r, err := setBit(b, i)
					Expect(err).ToNot(HaveOccurred())
					Expect(r).To(Equal(bytes[i]))
				}
			})

			It("should return 255", func() {
				bytes := []byte{
					0b01111111,
					0b10111111,
					0b11011111,
					0b11101111,
					0b11110111,
					0b11111011,
					0b11111101,
					0b11111110,
				}

				for i := range bytes {
					r, err := setBit(bytes[i], i)
					Expect(err).ToNot(HaveOccurred())
					Expect(r).To(Equal(byte(0b11111111)))
				}
			})

			It("should keep the byte unmodified", func() {
				b := byte(0b00000000)

				r, err := setBit(b, 8)
				Expect(r).To(Equal(b))
				Expect(err).To(HaveOccurred())

				r, err = setBit(b, 9)
				Expect(r).To(Equal(b))
				Expect(err).To(HaveOccurred())

				r, err = setBit(b, -1)
				Expect(r).To(Equal(b))
				Expect(err).To(HaveOccurred())

				r, err = setBit(b, -2)
				Expect(r).To(Equal(b))
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
