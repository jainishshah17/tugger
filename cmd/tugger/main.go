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
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"k8s.io/api/admission/v1beta1"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	tlsCertFile string
	tlsKeyFile  string
)

var (
	dockerRegistryUrl   = os.Getenv("DOCKER_REGISTRY_URL")
	registrySecretName  = os.Getenv("REGISTRY_SECRET_NAME")
	whitelistNamespaces = os.Getenv("WHITELIST_NAMESPACES")
	whitelist           = strings.Split(whitelistNamespaces, ",")
)

type patch struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

func main() {
	flag.StringVar(&tlsCertFile, "tls-cert", "/etc/admission-controller/tls/tls.crt", "TLS certificate file.")
	flag.StringVar(&tlsKeyFile, "tls-key", "/etc/admission-controller/tls/tls.key", "TLS key file.")
	flag.Parse()

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
		log.Println(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	log.Println(string(data))

	ar := v1beta1.AdmissionReview{}
	if err := json.Unmarshal(data, &ar); err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	namespace := ar.Request.Namespace
	log.Printf("AdmissionReview Namespace is: %s", namespace)

	admissionResponse := v1beta1.AdmissionResponse{Allowed: false}
	patches := []patch{}
	if !contains(whitelist, namespace) {
		pod := v1.Pod{}
		if err := json.Unmarshal(ar.Request.Object.Raw, &pod); err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Handle Containers
		for _, container := range pod.Spec.Containers {
			createPatch := handleContainer(&container, dockerRegistryUrl)
			if createPatch {
				patches = append(patches, patch{
					Op:    "add",
					Path:  "/spec/containers",
					Value: []v1.Container{container},
				})
			}
		}

		// Handle init containers
		for _, container := range pod.Spec.InitContainers {
			createPatch := handleContainer(&container, dockerRegistryUrl)
			if createPatch {
				patches = append(patches, patch{
					Op:    "add",
					Path:  "/spec/initContainers",
					Value: []v1.Container{container},
				})
			}
		}
	} else {
		log.Printf("Namespace is %s Whitelisted", namespace)
	}

	admissionResponse.Allowed = true
	if len(patches) > 0 {

		// Add image pull secret patche
		patches = append(patches, patch{
			Op:   "add",
			Path: "/spec/imagePullSecrets",
			Value: []v1.LocalObjectReference{
				v1.LocalObjectReference{
					Name: registrySecretName,
				},
			},
		})

		patchContent, err := json.Marshal(patches)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusBadRequest)
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
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func handleContainer(container *v1.Container, dockerRegistryUrl string) bool {
	log.Println("Container Image is", container.Image)

	if !strings.Contains(container.Image, dockerRegistryUrl) {
		message := fmt.Sprintf("Image is not being pulled from Private Registry: %s", container.Image)
		log.Printf(message)

		newImage := dockerRegistryUrl + "/" + container.Image
		log.Printf("Changing image registry to: %s", newImage)

		container.Image = newImage
		return true
	} else {
		log.Printf("Image is being pulled from Private Registry: %s", container.Image)
	}
	return false
}

func validateAdmissionReviewHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Serving request: %s", r.URL.Path)
	//set header
	w.Header().Set("Content-Type", "application/json")

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	log.Println(string(data))

	ar := v1beta1.AdmissionReview{}
	if err := json.Unmarshal(data, &ar); err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	namespace := ar.Request.Namespace
	log.Printf("AdmissionReview Namespace is: %s", namespace)

	admissionResponse := v1beta1.AdmissionResponse{Allowed: false}
	if contains(whitelist, namespace) {
		pod := v1.Pod{}
		if err := json.Unmarshal(ar.Request.Object.Raw, &pod); err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Handle containers
		for _, container := range pod.Spec.Containers {
			log.Println("Container Image is", container.Image)

			if !strings.Contains(container.Image, dockerRegistryUrl) {
				message := fmt.Sprintf("Image is not being pulled from Private Registry: %s", container.Image)
				log.Printf(message)
				admissionResponse.Result = getInvalidContainerResponse(message)
				goto done
			} else {
				log.Printf("Image is being pulled from Private Registry: %s", container.Image)
				admissionResponse.Allowed = true
			}
		}

		// Handle init containers
		for _, container := range pod.Spec.InitContainers {
			log.Println("Init Container Image is", container.Image)

			if !strings.Contains(container.Image, dockerRegistryUrl) {
				message := fmt.Sprintf("Image is not being pulled from Private Registry: %s", container.Image)
				log.Printf(message)
				admissionResponse.Result = getInvalidContainerResponse(message)
				goto done
			} else {
				log.Printf("Image is being pulled from Private Registry: %s", container.Image)
				admissionResponse.Allowed = true
			}
		}
	} else {
		log.Printf("Namespace is %s Whitelisted", namespace)
		admissionResponse.Allowed = true
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

// ping responds to the request with a plain-text "Ok" message.
func healthCheck(w http.ResponseWriter, r *http.Request) {
	log.Printf("Serving request: %s", r.URL.Path)
	fmt.Fprintf(w, "Ok")
}
