// Copyright 2017 Google Inc. All Rights Reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/infobloxopen/atlas-app-toolkit/logging"
	"github.com/sirupsen/logrus"
	"k8s.io/api/admission/v1beta1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	ifExists    bool
	log         *logrus.Logger
	policy      *Policy
	tlsCertFile string
	tlsKeyFile  string
)

var (
	env                   = os.Getenv("ENV")
	dockerRegistryUrl     = os.Getenv("DOCKER_REGISTRY_URL")
	registrySecretName    = os.Getenv("REGISTRY_SECRET_NAME")
	whitelistRegistries   = os.Getenv("WHITELIST_REGISTRIES")
	whitelistNamespaces   = os.Getenv("WHITELIST_NAMESPACES")
	webhookUrl            = os.Getenv("WEBHOOK_URL")
	whitelistedNamespaces = strings.Split(whitelistNamespaces, ",")
	whitelistedRegistries = strings.Split(whitelistRegistries, ",")
)

type patch struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

type SlackRequestBody struct {
	Text string `json:"text"`
}

func main() {
	flag.BoolVar(&ifExists, "if-exists", false, "makes the mutation conditional on whether the mutated image name exists in the registry")
	logLevel := flag.String("log-level", "info", "log verbosity")
	policyFile := flag.String("policy-file", "", "YAML file defining allowed image name patterns (see readme)")
	flag.StringVar(&tlsCertFile, "tls-cert", "/etc/admission-controller/tls/tls.crt", "TLS certificate file.")
	flag.StringVar(&tlsKeyFile, "tls-key", "/etc/admission-controller/tls/tls.key", "TLS key file.")
	flag.Parse()

	log = logging.New(*logLevel)

	if *policyFile != "" {
		var err error
		if policy, err = NewPolicy(WithConfigFile(*policyFile)); err != nil {
			log.WithError(err).WithField("policy-file", *policyFile).Fatal("failed to load policy file")
		}
	}

	http.HandleFunc("/ping", healthCheck)
	http.HandleFunc("/mutate", mutateAdmissionReviewHandler)
	http.HandleFunc("/validate", validateAdmissionReviewHandler)
	s := http.Server{
		Addr: ":443",
		TLSConfig: &tls.Config{
			ClientAuth: tls.NoClientCert,
		},
	}
	log.Fatal(s.ListenAndServeTLS(tlsCertFile, tlsKeyFile))
}

func mutateAdmissionReviewHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Serving request: %s", r.URL.Path)
	//set header
	w.Header().Set("Content-Type", "application/json")

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.WithError(err).WithField("body", string(data)).Error("could not parse request")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	log.Println(string(data))

	ar := v1beta1.AdmissionReview{}
	if err := json.Unmarshal(data, &ar); err != nil || ar.Request == nil {
		log.WithError(err).WithField("body", string(data)).Error("could not parse request")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	namespace := ar.Request.Namespace
	log.Debugf("AdmissionReview Namespace is: %s", namespace)

	admissionResponse := v1beta1.AdmissionResponse{Allowed: false}
	patches := []patch{}

	pod := v1.Pod{}
	if !contains(whitelistedNamespaces, namespace) {
		if err := json.Unmarshal(ar.Request.Object.Raw, &pod); err != nil {
			log.WithError(err).WithField("object", ar.Request.Object.Raw).Error("could unmarshal pod spec")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Handle Containers
		for i, container := range pod.Spec.Containers {
			originalImage := container.Image
			if handleContainer(&container, dockerRegistryUrl) {
				patches = append(
					patches, patch{
						Op:    "replace",
						Path:  fmt.Sprintf("/spec/containers/%d/image", i),
						Value: container.Image,
					},
					patch{
						Op:    "add",
						Path:  fmt.Sprintf("/metadata/annotations/tugger-original-image-%d", i),
						Value: originalImage,
					},
				)
			}
		}

		// Handle init containers
		for i, container := range pod.Spec.InitContainers {
			originalImage := container.Image
			if handleContainer(&container, dockerRegistryUrl) {
				patches = append(patches,
					patch{
						Op:    "replace",
						Path:  fmt.Sprintf("/spec/initContainers/%d/image", i),
						Value: container.Image,
					},
					patch{
						Op:    "add",
						Path:  fmt.Sprintf("/metadata/annotations/tugger-original-init-image-%d", i),
						Value: originalImage,
					},
				)
			}
		}
	} else {
		log.Printf("Namespace is %s Whitelisted", namespace)
	}

	admissionResponse.Allowed = true
	if len(patches) > 0 {

		// If the pod doesnt have annotations prepend a patch
		// so the annotations map exists before the patches above
		if pod.ObjectMeta.Annotations == nil {
			patches = append([]patch{patch{
				Op:    "add",
				Path:  "/metadata/annotations",
				Value: map[string]string{},
			}}, patches...)
		}

		// If the pod doesn't have labels append a patch
		if pod.ObjectMeta.Labels == nil {
			patches = append(patches, patch{
				Op:    "add",
				Path:  "/metadata/labels",
				Value: map[string]string{},
			})
		}

		// Add image pull secret and label
		patches = append(patches, patch{
			Op:   "add",
			Path: "/spec/imagePullSecrets",
			Value: []v1.LocalObjectReference{
				v1.LocalObjectReference{
					Name: registrySecretName,
				},
			}},
			patch{
				Op:    "add",
				Path:  "/metadata/labels/tugger-modified",
				Value: "true",
			})

		patchContent, err := json.Marshal(patches)
		if err != nil {
			log.WithError(err).WithField("patches", patches).Error("could not marshal patches")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		admissionResponse.Patch = patchContent
		pt := v1beta1.PatchTypeJSONPatch
		admissionResponse.PatchType = &pt
	}

	ar = v1beta1.AdmissionReview{
		Response: &admissionResponse,
	}

	data, err = json.Marshal(ar)
	if err != nil {
		log.WithError(err).WithField("resp", ar).Error("could not marshal response")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

// imageExists verifies an image exists in the remote registry
func imageExists(image string) bool {
	ref, err := name.ParseReference(image)
	if err != nil {
		log.WithError(err).WithField("image", image).Error("could not parse image")
		return false
	}

	if _, err := remote.Image(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain)); err != nil {
		log.WithError(err).WithField("image", image).Error("could not fetch image")
		return false
	}

	return true
}

func handleContainer(container *v1.Container, dockerRegistryUrl string) bool {
	log.Println("Container Image is", container.Image)

	if policy != nil {
		originalImage := container.Image
		container.Image, _ = policy.MutateImage(container.Image)
		if originalImage != container.Image {
			log.Println("Changing image from", originalImage, "to", container.Image)
			return true
		}
		return false
	}

	// backwards compatibility when policy is undefined
	if containsRegisty(whitelistedRegistries, container.Image) {
		log.Printf("Image is being pulled from Private Registry: %s", container.Image)
		return false
	}
	message := fmt.Sprintf("Image is not being pulled from Private Registry: %s", container.Image)
	log.Printf(message)

	newImage := dockerRegistryUrl + "/" + container.Image
	if ifExists && !imageExists(newImage) {
		message := fmt.Sprintf("%s does not exist in private registry, skipping patching of %s", newImage, container.Name)
		log.Print(message)
		SendSlackNotification(message)
		return false
	}

	log.Println("Changing image from", container.Image, "to", newImage)

	container.Image = newImage
	return true
}

func validateAdmissionReviewHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Serving request: %s", r.URL.Path)
	//set header
	w.Header().Set("Content-Type", "application/json")

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.WithError(err).WithField("body", string(data)).Error("could not read request")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	log.Println(string(data))

	ar := v1beta1.AdmissionReview{}
	if err := json.Unmarshal(data, &ar); err != nil {
		log.WithError(err).WithField("body", string(data)).Error("could not parse request")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	namespace := ar.Request.Namespace
	log.Debugf("AdmissionReview Namespace is: %s", namespace)

	admissionResponse := v1beta1.AdmissionResponse{Allowed: true}
	if !contains(whitelistedNamespaces, namespace) {
		pod := v1.Pod{}
		if err := json.Unmarshal(ar.Request.Object.Raw, &pod); err != nil {
			log.WithError(err).WithField("object", ar.Request.Object.Raw).Error("could unmarshal pod spec")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		var validateImage func(string) bool
		if policy != nil {
			validateImage = policy.ValidateImage
		} else {
			// backwards compatibility when policy is undefined
			validateImage = func(image string) bool {
				return containsRegisty(whitelistedRegistries, image)
			}
		}

		// Handle containers
		containers := []v1.Container{}
		containers = append(containers, pod.Spec.Containers...)
		containers = append(containers, pod.Spec.InitContainers...)
		for _, container := range containers {
			log.Println("Container Image is", container.Image)
			if !validateImage(container.Image) {
				message := fmt.Sprintf("Image is not being pulled from Private Registry: %s", container.Image)
				log.Printf(message)
				SendSlackNotification(message)
				admissionResponse.Allowed = false
				admissionResponse.Result = getInvalidContainerResponse(message)
				goto done
			} else {
				log.Printf("Image is being pulled from Private Registry: %s", container.Image)
				admissionResponse.Allowed = true && admissionResponse.Allowed
			}
		}
	} else {
		log.Printf("Namespace is %s Whitelisted", namespace)
		admissionResponse.Allowed = true && admissionResponse.Allowed
	}

done:
	ar = v1beta1.AdmissionReview{
		Response: &admissionResponse,
	}

	data, err = json.Marshal(ar)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func getInvalidContainerResponse(message string) *metav1.Status {
	return &metav1.Status{
		Reason: metav1.StatusReasonInvalid,
		Details: &metav1.StatusDetails{
			Causes: []metav1.StatusCause{
				{Message: message},
			},
		},
	}
}

// if current namespace is part of whitelisted namespaces
func contains(arr []string, str string) bool {
	for _, a := range arr {
		if a == str || strings.Contains(a, str) {
			return true
		}
	}
	return false
}

// if current registry is part of whitelisted registries
func containsRegisty(arr []string, str string) bool {
	for _, a := range arr {
		if a == str || strings.Contains(str, a) {
			return true
		}
	}
	return false
}

// ping responds to the request with a plain-text "Ok" message.
func healthCheck(w http.ResponseWriter, r *http.Request) {
	log.Debugf("Serving request: %s", r.URL.Path)
	fmt.Fprintf(w, "Ok")
}

// SendSlackNotification will post to an 'Incoming Webook' url setup in Slack Apps. It accepts
// some text and the slack channel is saved within Slack.
func SendSlackNotification(msg string) {
	if webhookUrl == "" {
		log.Debugln("Slack Webhook URL is not provided")
		return
	}

	if env != "" {
		msg = fmt.Sprintf("[%s] %s", env, msg)
	}
	slackBody, _ := json.Marshal(SlackRequestBody{Text: msg})
	req, err := http.NewRequest(http.MethodPost, webhookUrl, bytes.NewBuffer(slackBody))
	if err != nil {
		log.WithError(err).Error("unable to build slack request")
		return
	}

	req.Header.Add("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.WithError(err).Error("got error from Slack")
		return
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	if buf.String() != "ok" {
		log.WithField("resp", buf.String()).Errorln("Non-ok response returned from Slack")
	}
	defer resp.Body.Close()
}
