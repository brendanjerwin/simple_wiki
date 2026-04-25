//revive:disable:dot-imports
package server

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("jsonBytesToMap", func() {
	When("the JSON is malformed", func() {
		var result map[string]any
		var err error

		BeforeEach(func() {
			result, err = jsonBytesToMap([]byte("{not valid json"))
		})

		It("should return an error mentioning decode json", func() {
			Expect(err).To(MatchError(ContainSubstring("decode json")))
		})

		It("should return a nil map", func() {
			Expect(result).To(BeNil())
		})
	})

	When("the JSON contains nested objects, arrays, integer numbers, and float numbers", func() {
		var result map[string]any
		var err error

		BeforeEach(func() {
			input := []byte(`{
				"name": "outer",
				"count": 42,
				"ratio": 1.5,
				"nested": {
					"inner_count": 7,
					"items": [1, 2.25, 3]
				},
				"arr": [{"k": 11}, {"k": 22.5}]
			}`)
			result, err = jsonBytesToMap(input)
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should preserve string scalars", func() {
			Expect(result["name"]).To(Equal("outer"))
		})

		It("should normalize integer json.Numbers to float64 (Int64 path)", func() {
			Expect(result["count"]).To(BeNumerically("==", 42))
			_, ok := result["count"].(float64)
			Expect(ok).To(BeTrue(), "expected count to be float64 after normalization")
		})

		It("should normalize floating-point json.Numbers to float64 (Float64 fallback path)", func() {
			Expect(result["ratio"]).To(BeNumerically("==", 1.5))
			_, ok := result["ratio"].(float64)
			Expect(ok).To(BeTrue(), "expected ratio to be float64 after normalization")
		})

		It("should recursively normalize numbers in nested maps", func() {
			nested, ok := result["nested"].(map[string]any)
			Expect(ok).To(BeTrue())
			Expect(nested["inner_count"]).To(BeNumerically("==", 7))
			_, isFloat := nested["inner_count"].(float64)
			Expect(isFloat).To(BeTrue())
		})

		It("should recursively normalize numbers inside nested arrays", func() {
			nested, ok := result["nested"].(map[string]any)
			Expect(ok).To(BeTrue())
			items, ok := nested["items"].([]any)
			Expect(ok).To(BeTrue())
			Expect(items).To(HaveLen(3))
			for _, item := range items {
				_, isFloat := item.(float64)
				Expect(isFloat).To(BeTrue(), "expected each array element to be float64")
			}
		})

		It("should recursively normalize numbers inside arrays of objects", func() {
			arr, ok := result["arr"].([]any)
			Expect(ok).To(BeTrue())
			Expect(arr).To(HaveLen(2))
			first, ok := arr[0].(map[string]any)
			Expect(ok).To(BeTrue())
			_, isFloat := first["k"].(float64)
			Expect(isFloat).To(BeTrue())
		})
	})
})

var _ = Describe("mapToJSONBytes", func() {
	When("encoding a nested map containing strings, floats, and arrays", func() {
		var (
			input     map[string]any
			encoded   []byte
			encodeErr error
		)

		BeforeEach(func() {
			input = map[string]any{
				"title":  "hi",
				"weight": 2.5,
				"tags":   []any{"a", "b"},
				"nested": map[string]any{
					"k": "v",
					"n": 3.0,
				},
			}
			encoded, encodeErr = mapToJSONBytes(input)
		})

		It("should not return an error", func() {
			Expect(encodeErr).NotTo(HaveOccurred())
		})

		It("should produce non-empty bytes", func() {
			Expect(encoded).NotTo(BeEmpty())
		})

		Describe("when the encoded bytes are decoded again via jsonBytesToMap", func() {
			var (
				roundTrip   map[string]any
				roundTripErr error
			)

			BeforeEach(func() {
				roundTrip, roundTripErr = jsonBytesToMap(encoded)
			})

			It("should not return an error on the round trip", func() {
				Expect(roundTripErr).NotTo(HaveOccurred())
			})

			It("should preserve top-level string scalars", func() {
				Expect(roundTrip["title"]).To(Equal("hi"))
			})

			It("should preserve top-level numeric scalars as float64", func() {
				Expect(roundTrip["weight"]).To(BeNumerically("==", 2.5))
			})

			It("should preserve nested string values", func() {
				nested, ok := roundTrip["nested"].(map[string]any)
				Expect(ok).To(BeTrue())
				Expect(nested["k"]).To(Equal("v"))
			})

			It("should preserve nested numeric values as float64", func() {
				nested, ok := roundTrip["nested"].(map[string]any)
				Expect(ok).To(BeTrue())
				Expect(nested["n"]).To(BeNumerically("==", 3.0))
			})

			It("should preserve array elements", func() {
				tags, ok := roundTrip["tags"].([]any)
				Expect(ok).To(BeTrue())
				Expect(tags).To(Equal([]any{"a", "b"}))
			})
		})
	})
})
