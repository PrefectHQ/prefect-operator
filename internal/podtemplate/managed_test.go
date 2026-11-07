package podtemplate

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("MergeManaged", func() {
	It("keeps annotations it didn't set", func() {
		merged, managed := MergeManaged(
			map[string]string{"reloader.stakater.com/last-reloaded-from": "x"},
			map[string]string{"a": "1"},
			"",
		)

		Expect(merged).To(Equal(map[string]string{
			"reloader.stakater.com/last-reloaded-from": "x",
			"a": "1",
		}))
		Expect(managed).To(Equal("a"))
	})

	It("removes annotations it set that are no longer desired", func() {
		merged, managed := MergeManaged(
			map[string]string{"a": "1", "b": "2", "other": "x"},
			map[string]string{"b": "2"},
			"a,b",
		)

		Expect(merged).To(Equal(map[string]string{"b": "2", "other": "x"}))
		Expect(managed).To(Equal("b"))
	})

	It("lets the desired value win over the live one", func() {
		merged, _ := MergeManaged(
			map[string]string{"a": "live"},
			map[string]string{"a": "desired"},
			"",
		)

		Expect(merged).To(Equal(map[string]string{"a": "desired"}))
	})

	It("lists the managed keys sorted", func() {
		_, managed := MergeManaged(nil, map[string]string{"c": "", "a": "", "b": ""}, "")

		Expect(managed).To(Equal("a,b,c"))
	})

	It("returns nil and no managed keys when there's nothing to set", func() {
		merged, managed := MergeManaged(nil, nil, "a")

		Expect(merged).To(BeNil())
		Expect(managed).To(BeEmpty())
	})
})
