package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"k8s.io/api/admission/v1beta1"
	"k8s.io/api/core/v1"
	"log"
	"net/http"
	"os"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	notesPath       = "/v1alpha1/projects/image-signing/notes"
	occurrencesPath = "/v1alpha1/projects/image-signing/occurrences"
)

func main() {

	dockerRegistryUrl := os.Getenv("DOCKER_REGISTRY_URL")

	// use PORT environment variable, or default to 8080
	port := "8080"
	if fromEnv := os.Getenv("PORT"); fromEnv != "" {
		port = fromEnv
	}

	// register function to handle all requests
	server := http.NewServeMux()
	server.HandleFunc("/validate", func(w http.ResponseWriter, r *http.Request) {
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

		pod := v1.Pod{}
		if err := json.Unmarshal(ar.Request.Object.Raw, &pod); err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		admissionResponse := v1beta1.AdmissionResponse{Allowed: false}
		for _, container := range pod.Spec.Containers {
			log.Println("Container Image is", container.Image)

			if !strings.Contains(container.Image, dockerRegistryUrl) {
				message := fmt.Sprintf("Image is not being pulled from Private Registry: %s", container.Image)
				log.Printf(message)
				admissionResponse.Result = &metav1.Status{
					Reason: metav1.StatusReasonInvalid,
					Details: &metav1.StatusDetails{
						Causes: []metav1.StatusCause{
							{Message: message},
						},
					},
				}
				goto done
			} else {
				log.Printf("Image is being pulled from Private Registry: %s", container.Image)
				admissionResponse.Allowed = true
			}
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
	})

	server.HandleFunc("/ping", healthCheck)

	server.HandleFunc("/mutate", func(w http.ResponseWriter, r *http.Request) {
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

		pod := v1.Pod{}
		if err := json.Unmarshal(ar.Request.Object.Raw, &pod); err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		admissionResponse := v1beta1.AdmissionResponse{Allowed: false}
		for _, container := range pod.Spec.Containers {
			log.Println("Container Image is", container.Image)

			if !strings.Contains(container.Image, dockerRegistryUrl) {
				message := fmt.Sprintf("Image is not being pulled from Private Registry: %s", container.Image)
				log.Printf(message)

				newImage := dockerRegistryUrl + "/" + container.Image
				log.Printf("Changing image registry to:%s", newImage)
				addContainerPatch := `[
		 			{"op":"add","path":"/spec/containers","value":[{"image":"`+newImage+`","name":"`+container.Name+`","resources":{}}]}
				]`

				admissionResponse.Allowed = true
				admissionResponse.Patch = []byte(string(addContainerPatch))
				pt := v1beta1.PatchTypeJSONPatch
				admissionResponse.PatchType = &pt

				goto done
			} else {
				log.Printf("Image is being pulled from Private Registry: %s", container.Image)
				admissionResponse.Allowed = true
			}
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
	})

	// start the web server on port and accept requests
	log.Printf("Server listening on port %s", port)
	err := http.ListenAndServe(":"+port, server)
	log.Fatal(err)
}

// ping responds to the request with a plain-text "Ok" message.
func healthCheck(w http.ResponseWriter, r *http.Request) {
	log.Printf("Serving request: %s", r.URL.Path)
	fmt.Fprintf(w, "Ok")
}