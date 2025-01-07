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

package remotemetrics

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type trueMatcher struct{}

func (m *trueMatcher) Match(line string) bool {
	return true
}

type falseMatcher struct{}

func (m *falseMatcher) Match(line string) bool {
	return false
}

var _ = Context("MatcherCollection", func() {

	var matcher MatcherCollection
	var matchers []Matcher
	var line string
	var res bool

	JustBeforeEach(func() {
		for _, m := range matchers {
			matcher.Add(m)
		}

		res = matcher.Match(line)
	})

	Context("All True", func() {

		BeforeEach(func() {
			matchers = []Matcher{
				&trueMatcher{},
				&trueMatcher{},
			}
		})

		Context("matchAll", func() {

			BeforeEach(func() { matcher = MatchAll() })

			It("should match", func() { Expect(res).To(BeTrue()) })

		})

		Context("matchAny", func() {

			BeforeEach(func() { matcher = MatchAny() })

			It("should match", func() { Expect(res).To(BeTrue()) })

		})

	})

	Context("All False", func() {

		BeforeEach(func() {
			matchers = []Matcher{
				&falseMatcher{},
				&falseMatcher{},
			}
		})

		Context("matchAll", func() {

			BeforeEach(func() { matcher = MatchAll() })

			It("should match", func() { Expect(res).To(BeFalse()) })

		})

		Context("matchAny", func() {

			BeforeEach(func() { matcher = MatchAny() })

			It("should match", func() { Expect(res).To(BeFalse()) })

		})

	})

	Context("First False, Second True", func() {

		BeforeEach(func() {
			matchers = []Matcher{
				&falseMatcher{},
				&trueMatcher{},
			}
		})

		Context("matchAll", func() {

			BeforeEach(func() { matcher = MatchAll() })

			It("should not match", func() { Expect(res).To(BeFalse()) })

		})

		Context("matchAny", func() {

			BeforeEach(func() { matcher = MatchAny() })

			It("should match", func() { Expect(res).To(BeTrue()) })

		})

	})

	Context("First True, Second False", func() {

		BeforeEach(func() {
			matchers = []Matcher{
				&trueMatcher{},
				&falseMatcher{},
			}
		})

		Context("matchAll", func() {

			BeforeEach(func() { matcher = MatchAll() })

			It("should not match", func() { Expect(res).To(BeFalse()) })

		})

		Context("matchAny", func() {

			BeforeEach(func() { matcher = MatchAny() })

			It("should match", func() { Expect(res).To(BeTrue()) })

		})

	})

})
