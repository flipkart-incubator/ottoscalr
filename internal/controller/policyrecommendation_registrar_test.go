package controller

import (
	"context"
	rolloutv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	ottoscaleriov1alpha1 "github.com/flipkart-incubator/ottoscalr/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestPolicyRecommendationRegistrationController(t *testing.T) {

	// Create a new scheme and register the types with it
	scheme := runtime.NewScheme()
	err := clientgoscheme.AddToScheme(scheme)
	assert.NoError(t, err)
	err = rolloutv1alpha1.AddToScheme(scheme)
	assert.NoError(t, err)
	err = ottoscaleriov1alpha1.AddToScheme(scheme)
	assert.NoError(t, err)

	rollout := &rolloutv1alpha1.Rollout{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Rollout",
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-rollout",
			Namespace: "my-namespace",
		},
	}

	// Create a fake controller runtime client with the instance and policy
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRuntimeObjects(rollout).
		Build()

	// Create the controller and inject the fake client
	controller := &PolicyRecommendationRegistrar{
		Client: fakeClient,
	}

	// Create the reconcile request
	request := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "my-rollout",
			Namespace: "my-namespace",
		},
	}

	// Call the Reconcile method and verify that a new PolicyRecommendation was created
	result, err := controller.Reconcile(context.Background(), request)
	assert.NoError(t, err)
	assert.True(t, result.Requeue == false)

	foundPolicy := &ottoscaleriov1alpha1.PolicyRecommendation{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{Name: "my-rollout", Namespace: "my-namespace"}, foundPolicy)
	assert.NoError(t, err)
	assert.Equal(t, rollout.GetName(), foundPolicy.OwnerReferences[0].Name)
	assert.Equal(t, rollout.Kind, foundPolicy.OwnerReferences[0].Kind)
	assert.Equal(t, rollout.APIVersion, foundPolicy.OwnerReferences[0].APIVersion)

	// Call the Reconcile method again and verify that a new PolicyRecommendation is not created
	result, err = controller.Reconcile(context.Background(), request)
	assert.NoError(t, err)
	assert.True(t, result.Requeue == false)

	policies := &ottoscaleriov1alpha1.PolicyRecommendationList{}
	err = fakeClient.List(context.Background(), policies)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(policies.Items))
}
