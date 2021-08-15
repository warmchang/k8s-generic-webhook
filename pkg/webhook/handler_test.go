package webhook

import (
	"context"
	"encoding/json"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var _ = Describe("Handler", func() {
	Context("Handle", func() {
		var (
			decoder *admission.Decoder
		)
		BeforeEach(func() {
			scheme := runtime.NewScheme()
			err := corev1.AddToScheme(scheme)
			Ω(err).ShouldNot(HaveOccurred())
			decoder, err = admission.NewDecoder(scheme)
			Ω(err).ShouldNot(HaveOccurred())

		})
		It("should deny by default", func() {
			result := (&handler{}).Handle(context.TODO(), admission.Request{})
			Ω(result.Allowed).Should(BeFalse())
		})
		It("should validate", func() {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
			}
			raw, err := json.Marshal(pod)
			Ω(err).ShouldNot(HaveOccurred())

			h := handler{
				Handler: &ValidateFuncs{
					CreateFunc: func(ctx context.Context, request admission.Request) admission.Response {
						return admission.Allowed("")
					},
					UpdateFunc: func(ctx context.Context, request admission.Request) admission.Response {
						return admission.Denied("")
					},
					DeleteFunc: func(ctx context.Context, request admission.Request) admission.Response {
						return admission.Denied("")
					},
				},
				Object: &corev1.Pod{},
			}
			err = h.InjectDecoder(decoder)
			Ω(err).ShouldNot(HaveOccurred())

			result := h.Handle(context.TODO(), admission.Request{
				AdmissionRequest: admissionv1.AdmissionRequest{
					Object: runtime.RawExtension{
						Raw: raw,
					},
					Operation: admissionv1.Create,
				},
			})
			Ω(result.Allowed).Should(BeTrue())
			result = h.Handle(context.TODO(), admission.Request{
				AdmissionRequest: admissionv1.AdmissionRequest{
					Object: runtime.RawExtension{
						Raw: raw,
					},
					Operation: admissionv1.Update,
				},
			})
			Ω(result.Allowed).Should(BeFalse())
			result = h.Handle(context.TODO(), admission.Request{
				AdmissionRequest: admissionv1.AdmissionRequest{
					Object: runtime.RawExtension{
						Raw: raw,
					},
					Operation: admissionv1.Delete,
				},
			})
			Ω(result.Allowed).Should(BeFalse())

		})
		It("should decode object", func() {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
				Spec: corev1.PodSpec{
					NodeName: "jin",
				},
			}
			raw, err := json.Marshal(pod)
			Ω(err).ShouldNot(HaveOccurred())

			h := handler{
				Handler: &MutateFunc{
					Func: func(ctx context.Context, request admission.Request) admission.Response {
						if len(request.Object.Raw) > 0 {
							Ω(request.Object.Object).Should(Equal(pod))
						}
						if len(request.OldObject.Raw) > 0 {
							Ω(request.OldObject.Object).Should(Equal(pod))
						}
						return admission.Allowed("")
					},
				},
				Object: &corev1.Pod{},
			}
			err = h.InjectDecoder(decoder)
			Ω(err).ShouldNot(HaveOccurred())

			result := h.Handle(context.TODO(), admission.Request{
				AdmissionRequest: admissionv1.AdmissionRequest{
					Object: runtime.RawExtension{
						Raw: raw,
					},
				},
			})
			Ω(result.Allowed).Should(BeTrue())

			result = h.Handle(context.TODO(), admission.Request{
				AdmissionRequest: admissionv1.AdmissionRequest{
					OldObject: runtime.RawExtension{
						Raw: raw,
					},
				},
			})
			Ω(result.Allowed).Should(BeTrue())
		})
		It("should not decode invalid object", func() {
			h := handler{
				Handler: &MutateFunc{
					Func: func(ctx context.Context, request admission.Request) admission.Response {
						return admission.Allowed("")
					},
				},
				Object: &corev1.Pod{},
			}
			err := h.InjectDecoder(decoder)
			Ω(err).ShouldNot(HaveOccurred())

			result := h.Handle(context.TODO(), admission.Request{
				AdmissionRequest: admissionv1.AdmissionRequest{
					Object: runtime.RawExtension{
						Raw: []byte{1, 2, 3, 4, 5},
					},
				},
			})
			Ω(result.Allowed).Should(BeFalse())

			result = h.Handle(context.TODO(), admission.Request{
				AdmissionRequest: admissionv1.AdmissionRequest{
					OldObject: runtime.RawExtension{
						Raw: []byte{1, 2, 3, 4, 5},
					},
				},
			})
			Ω(result.Allowed).Should(BeFalse())
		})
	})
	Context("InjectDecoder", func() {
		var (
			decoder *admission.Decoder
		)
		BeforeEach(func() {
			decoder = &admission.Decoder{}
		})
		It("should pass decoder to validating webhook", func() {
			webhook := ValidatingWebhook{}
			Ω((&handler{Handler: &webhook}).InjectDecoder(decoder)).ShouldNot(HaveOccurred())
			Ω(webhook.Decoder).Should(Equal(decoder))
		})
		It("should pass decoder to mutating webhook", func() {
			webhook := MutatingWebhook{}
			Ω((&handler{Handler: &webhook}).InjectDecoder(decoder)).ShouldNot(HaveOccurred())
			Ω(webhook.Decoder).Should(Equal(decoder))
		})
		It("should never fail if handler not set", func() {
			Ω((&handler{}).InjectDecoder(decoder)).ShouldNot(HaveOccurred())
		})
	})
	Context("InjectClient", func() {
		var (
			client client.Client
		)
		BeforeEach(func() {
			client = fake.NewClientBuilder().Build()
		})
		It("should pass client to validating webhook", func() {
			webhook := ValidatingWebhook{}
			Ω((&handler{Handler: &webhook}).InjectClient(client)).ShouldNot(HaveOccurred())
			Ω(webhook.Client).Should(Equal(client))
		})
		It("should pass client to mutating webhook", func() {
			webhook := MutatingWebhook{}
			Ω((&handler{Handler: &webhook}).InjectClient(client)).ShouldNot(HaveOccurred())
			Ω(webhook.Client).Should(Equal(client))
		})
		It("should never fail if handler not set", func() {
			Ω((&handler{}).InjectClient(client)).ShouldNot(HaveOccurred())
		})
	})
})
