package registry

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("DeploymentClientRegistry", func() {

	Describe("GetObjectClient", func() {
		Context("when the object kind is found in the client registry", func() {
			It("returns the corresponding ObjectClient", func() {
				// Create a mock ObjectClient for testing

				// Add the mock ObjectClient to the DeploymentClientRegistry
				deploymentClientRegistry.Clients = []ObjectClient{deploymentClient, rolloutClient}

				// Call GetObjectClient with the object kind "Deployment"
				client, err := deploymentClientRegistry.GetObjectClient("Deployment")
				Expect(err).NotTo(HaveOccurred())
				Expect(client).To(Equal(deploymentClient))
			})
		})

		Context("when the object kind is not found in the client registry", func() {
			It("returns an error", func() {
				// Call GetObjectClient with an unknown object kind
				client, err := deploymentClientRegistry.GetObjectClient("Unknown")
				Expect(err).To(HaveOccurred())
				Expect(client).To(BeNil())
				Expect(err.Error()).To(Equal("object kind not found in client registry"))
			})
		})
	})

	Describe("WithCustomDeploymentClient", func() {
		It("adds the ObjectClient to the client registry", func() {
			// Create a mock ObjectClient for testing

			// Add the mock ObjectClient to the DeploymentClientRegistry using WithCustomDeploymentClient
			registryBuilder := NewDeploymentClientRegistryBuilder().
				WithCustomDeploymentClient(deploymentClient).WithCustomDeploymentClient(rolloutClient)
			Expect(registryBuilder.Clients).To(Equal([]ObjectClient{deploymentClient, rolloutClient}))
		})
	})
})
