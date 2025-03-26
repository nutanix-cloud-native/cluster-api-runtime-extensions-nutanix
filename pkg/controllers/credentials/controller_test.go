package credentials

// import (
// 	"context"

// 	. "github.com/onsi/ginkgo/v2"
// 	. "github.com/onsi/gomega"
// 	"k8s.io/apimachinery/pkg/api/errors"
// 	"k8s.io/apimachinery/pkg/types"
// 	"sigs.k8s.io/controller-runtime/pkg/reconcile"

// 	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// 	carennutanixcomv1alpha1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
// )

// var _ = Describe("CredentialsRequest Controller", func() {
// 	Context("When reconciling a resource", func() {
// 		const resourceName = "test-resource"

// 		ctx := context.Background()

// 		typeNamespacedName := types.NamespacedName{
// 			Name:      resourceName,
// 			Namespace: "default", // TODO(user):Modify as needed
// 		}
// 		credentialsrequest := &carennutanixcomv1alpha1.CredentialsRequest{}

// 		BeforeEach(func() {
// 			By("creating the custom resource for the Kind CredentialsRequest")
// 			err := k8sClient.Get(ctx, typeNamespacedName, credentialsrequest)
// 			if err != nil && errors.IsNotFound(err) {
// 				resource := &carennutanixcomv1alpha1.CredentialsRequest{
// 					ObjectMeta: metav1.ObjectMeta{
// 						Name:      resourceName,
// 						Namespace: "default",
// 					},
// 					// TODO(user): Specify other spec details if needed.
// 				}
// 				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
// 			}
// 		})

// 		AfterEach(func() {
// 			// TODO(user): Cleanup logic after each test, like removing the resource instance.
// 			resource := &carennutanixcomv1alpha1.CredentialsRequest{}
// 			err := k8sClient.Get(ctx, typeNamespacedName, resource)
// 			Expect(err).NotTo(HaveOccurred())

// 			By("Cleanup the specific resource instance CredentialsRequest")
// 			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
// 		})
// 		It("should successfully reconcile the resource", func() {
// 			By("Reconciling the created resource")
// 			controllerReconciler := &CredentialsRequestReconciler{
// 				Client: k8sClient,
// 				Scheme: k8sClient.Scheme(),
// 			}

// 			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
// 				NamespacedName: typeNamespacedName,
// 			})
// 			Expect(err).NotTo(HaveOccurred())
// 			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
// 			// Example: If you expect a certain status condition after reconciliation, verify it here.
// 		})
// 	})
// })
