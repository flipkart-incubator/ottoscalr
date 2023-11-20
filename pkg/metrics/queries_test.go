package metrics

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Queries", func() {
	var (
		qc *QueryComponent
	)

	BeforeEach(func() {
		qc = &QueryComponent{
			metric: "test_metric",
			labels: map[string]string{
				"label1": "value1",
				"label2": "value2",
			},
		}
	})

	Describe("AddLabel", func() {
		Context("when adding a new label", func() {
			It("should add the label to the labels map", func() {
				qc.AddLabel("label3", "value3")
				Expect(qc.labels).To(HaveKeyWithValue("label3", "value3"))
			})
		})
	})

	Describe("Render", func() {
		Context("when labels are present", func() {
			It("should render the metric with labels", func() {
				Expect(qc.Render()).To(Equal("test_metric{label1=\"value1\",label2=\"value2\"}"))
			})
		})

		Context("when labels are not present", func() {
			It("should render the metric without labels", func() {
				qc.labels = nil
				Expect(qc.Render()).To(Equal("test_metric"))
			})
		})
	})

	Describe("ValidateQuery", func() {
		Context("when the query is valid", func() {
			It("should return true", func() {
				Expect(ValidateQuery("(test_metric{label1=\"value1\",label2=\"value2\"})")).To(BeTrue())
			})
		})

		Context("when the query is not valid", func() {
			It("should return false", func() {
				Expect(ValidateQuery("(test_metric{label1=\"value1\",label2=\"value2\"}")).To(BeFalse())
			})
		})
	})
})
