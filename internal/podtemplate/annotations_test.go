package podtemplate

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("MergeAnnotations", func() {
	It("keeps annotations it didn't set", func() {
		annotations, managed := MergeAnnotations(
			map[string]string{"reloader.stakater.com/last-reloaded-from": "x"},
			map[string]string{"a": "1"},
			"",
		)

		Expect(annotations).To(Equal(map[string]string{
			"reloader.stakater.com/last-reloaded-from": "x",
			"a": "1",
		}))
		Expect(managed).To(Equal("a"))
	})

	It("removes annotations it set that are no longer desired", func() {
		annotations, managed := MergeAnnotations(
			map[string]string{"a": "1", "b": "2", "other": "x"},
			map[string]string{"b": "2"},
			"a,b",
		)

		Expect(annotations).To(Equal(map[string]string{"b": "2", "other": "x"}))
		Expect(managed).To(Equal("b"))
	})

	It("lets the desired value win over the live one", func() {
		annotations, _ := MergeAnnotations(
			map[string]string{"a": "live"},
			map[string]string{"a": "desired"},
			"",
		)

		Expect(annotations).To(Equal(map[string]string{"a": "desired"}))
	})

	It("lists the managed keys sorted", func() {
		_, managed := MergeAnnotations(nil, map[string]string{"c": "", "a": "", "b": ""}, "")

		Expect(managed).To(Equal("a,b,c"))
	})

	It("returns nil and no managed keys when there's nothing to set", func() {
		annotations, managed := MergeAnnotations(nil, nil, "a")

		Expect(annotations).To(BeNil())
		Expect(managed).To(BeEmpty())
	})
})
