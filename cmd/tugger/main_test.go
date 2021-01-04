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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/infobloxopen/atlas-app-toolkit/logging"
	"github.com/jarcoal/httpmock"
	"github.com/patrickmn/go-cache"
)

const (
	mockSlackURL            = "https://slack/"
	trustedRegistry         = "private-registry.cluster.local"
	trustedAdmissionRequest = `
	{
		"kind": "AdmissionReview",
		"request": {
		  "kind": {
			"kind": "Pod",
			"version": "v1"
		  },
		  "name": "myapp",
		  "namespace": "foobar",
			"object": {
			"metadata": {
			  "name": "myapp",
			  "namespace": "foobar"
			},
			"spec": {
			  "containers": [
				{
				  "image": "` + trustedRegistry + `/nginx",
				  "name": "nginx-frontend"
				},
				{
				  "image": "` + trustedRegistry + `/mysql",
				  "name": "mysql-backend"
				}
			  ],
			  "initContainers": [
				{
				  "image": "` + trustedRegistry + `/nginx",
				  "name": "nginx-frontend"
				},
				{
				  "image": "` + trustedRegistry + `/mysql",
				  "name": "mysql-backend"
				}
			  ]
			}
		  }
		}
	  }`
	untrustedAdmissionRequest = `
	{
		"kind": "AdmissionReview",
		"request": {
		  "kind": {
			"kind": "Pod",
			"version": "v1"
		  },
		  "name": "myapp",
		  "namespace": "foobar",
	  	  "object": {
			"metadata": {
			  "name": "myapp",
			  "namespace": "foobar"
			},
			"spec": {
			  "containers": [
				{
				  "image": "nginx",
				  "name": "nginx-frontend"
				},
				{
				  "image": "mysql",
				  "name": "mysql-backend"
				}
			  ],
			  "initContainers": [
				{
				  "image": "nginx",
				  "name": "nginx-frontend"
				},
				{
				  "image": "mysql",
				  "name": "mysql-backend"
				}
			  ]
			}
		  }
		}
	  }`
	mixedTrustAdmissionRequest = `
	  {
		  "kind": "AdmissionReview",
		  "request": {
			"kind": {
			  "kind": "Pod",
			  "version": "v1"
			},
			"name": "myapp",
			"namespace": "foobar",
			  "object": {
			  "metadata": {
				"name": "myapp",
				"namespace": "foobar"
			  },
			  "spec": {
				"containers": [
					{
					"image": "` + trustedRegistry + `/nginx",
					"name": "nginx-frontend"
					},
					{
					"image": "` + trustedRegistry + `/mysql",
					"name": "mysql-backend"
					}
				],
				"initContainers": [
				  {
					"image": "` + trustedRegistry + `/nginx",
					"name": "nginx-frontend"
				  },
				  {
					"image": "mysql",
					"name": "mysql-backend"
				  }
				]
			  }
			}
		  }
		}`
	whitelistedAdmissionRequest = `
	  {
		  "kind": "AdmissionReview",
		  "request": {
			"kind": {
			  "kind": "Pod",
			  "version": "v1"
			},
			"name": "kube-apiserver",
			"namespace": "kube-system",
			  "object": {
			  "metadata": {
				"name": "kube-apiserver",
				"namespace": "kube-system"
			  },
			  "spec": {
				"containers": [
				  {
					"image": "apiserver",
					"name": "apiserver"
				  }
				]
			  }
			}
		  }
		}`
)

var (
	testCases = []struct {
		name         string
		handler      func(http.ResponseWriter, *http.Request)
		reqMethod    string
		reqPath      string
		reqBody      string
		expectStatus int
		expectBody   string
	}{
		{
			name:         "ping",
			handler:      healthCheck,
			expectStatus: http.StatusOK,
			expectBody:   `Ok`,
		},
		{
			name:         "mutate/empty",
			handler:      mutateAdmissionReviewHandler,
			reqMethod:    "POST",
			reqPath:      "/mutate",
			reqBody:      `{}`,
			expectStatus: http.StatusBadRequest,
		},
		{
			name:         "mutate/happy",
			handler:      mutateAdmissionReviewHandler,
			reqMethod:    "POST",
			reqPath:      "/mutate",
			reqBody:      string(untrustedAdmissionRequest),
			expectStatus: http.StatusOK,
			expectBody:   `{"response":{"uid":"","allowed":true,"patch":"W3sib3AiOiJhZGQiLCJwYXRoIjoiL21ldGFkYXRhL2Fubm90YXRpb25zIiwidmFsdWUiOnt9fSx7Im9wIjoicmVwbGFjZSIsInBhdGgiOiIvc3BlYy9jb250YWluZXJzLzAvaW1hZ2UiLCJ2YWx1ZSI6InByaXZhdGUtcmVnaXN0cnkuY2x1c3Rlci5sb2NhbC9uZ2lueCJ9LHsib3AiOiJhZGQiLCJwYXRoIjoiL21ldGFkYXRhL2Fubm90YXRpb25zL3R1Z2dlci1vcmlnaW5hbC1pbWFnZS0wIiwidmFsdWUiOiJuZ2lueCJ9LHsib3AiOiJyZXBsYWNlIiwicGF0aCI6Ii9zcGVjL2NvbnRhaW5lcnMvMS9pbWFnZSIsInZhbHVlIjoicHJpdmF0ZS1yZWdpc3RyeS5jbHVzdGVyLmxvY2FsL215c3FsIn0seyJvcCI6ImFkZCIsInBhdGgiOiIvbWV0YWRhdGEvYW5ub3RhdGlvbnMvdHVnZ2VyLW9yaWdpbmFsLWltYWdlLTEiLCJ2YWx1ZSI6Im15c3FsIn0seyJvcCI6InJlcGxhY2UiLCJwYXRoIjoiL3NwZWMvaW5pdENvbnRhaW5lcnMvMC9pbWFnZSIsInZhbHVlIjoicHJpdmF0ZS1yZWdpc3RyeS5jbHVzdGVyLmxvY2FsL25naW54In0seyJvcCI6ImFkZCIsInBhdGgiOiIvbWV0YWRhdGEvYW5ub3RhdGlvbnMvdHVnZ2VyLW9yaWdpbmFsLWluaXQtaW1hZ2UtMCIsInZhbHVlIjoibmdpbngifSx7Im9wIjoicmVwbGFjZSIsInBhdGgiOiIvc3BlYy9pbml0Q29udGFpbmVycy8xL2ltYWdlIiwidmFsdWUiOiJwcml2YXRlLXJlZ2lzdHJ5LmNsdXN0ZXIubG9jYWwvbXlzcWwifSx7Im9wIjoiYWRkIiwicGF0aCI6Ii9tZXRhZGF0YS9hbm5vdGF0aW9ucy90dWdnZXItb3JpZ2luYWwtaW5pdC1pbWFnZS0xIiwidmFsdWUiOiJteXNxbCJ9LHsib3AiOiJhZGQiLCJwYXRoIjoiL21ldGFkYXRhL2xhYmVscyIsInZhbHVlIjp7fX0seyJvcCI6ImFkZCIsInBhdGgiOiIvc3BlYy9pbWFnZVB1bGxTZWNyZXRzIiwidmFsdWUiOlt7fV19LHsib3AiOiJhZGQiLCJwYXRoIjoiL21ldGFkYXRhL2xhYmVscy90dWdnZXItbW9kaWZpZWQiLCJ2YWx1ZSI6InRydWUifV0=","patchType":"JSONPatch"}}`,
		},
		{
			name:         "mutate/trusted",
			handler:      mutateAdmissionReviewHandler,
			reqMethod:    "POST",
			reqPath:      "/mutate",
			reqBody:      string(trustedAdmissionRequest),
			expectStatus: http.StatusOK,
			expectBody:   `{"response":{"uid":"","allowed":true}}`,
		},
		{
			name:         "mutate/whitelisted",
			handler:      mutateAdmissionReviewHandler,
			reqMethod:    "POST",
			reqPath:      "/mutate",
			reqBody:      string(whitelistedAdmissionRequest),
			expectStatus: http.StatusOK,
			expectBody:   `{"response":{"uid":"","allowed":true}}`,
		},
		{
			name:         "validate/untrusted",
			handler:      validateAdmissionReviewHandler,
			reqMethod:    "POST",
			reqPath:      "/validate",
			reqBody:      string(untrustedAdmissionRequest),
			expectStatus: http.StatusOK,
			expectBody:   `{"response":{"uid":"","allowed":false,"status":{"metadata":{},"reason":"Invalid","details":{"causes":[{"message":"Image is not being pulled from Private Registry: nginx"}]}}}}`,
		},
		{
			name:         "validate/mixedtrust",
			handler:      validateAdmissionReviewHandler,
			reqMethod:    "POST",
			reqPath:      "/validate",
			reqBody:      string(mixedTrustAdmissionRequest),
			expectStatus: http.StatusOK,
			expectBody:   `{"response":{"uid":"","allowed":false,"status":{"metadata":{},"reason":"Invalid","details":{"causes":[{"message":"Image is not being pulled from Private Registry: mysql"}]}}}}`,
		},
		{
			name:         "validate/trusted",
			handler:      validateAdmissionReviewHandler,
			reqMethod:    "POST",
			reqPath:      "/validate",
			reqBody:      string(trustedAdmissionRequest),
			expectStatus: http.StatusOK,
			expectBody:   `{"response":{"uid":"","allowed":true}}`,
		},
		{
			name:         "validate/whitelisted",
			handler:      validateAdmissionReviewHandler,
			reqMethod:    "POST",
			reqPath:      "/validate",
			reqBody:      string(whitelistedAdmissionRequest),
			expectStatus: http.StatusOK,
			expectBody:   `{"response":{"uid":"","allowed":true}}`,
		},
	}
)

func TestHandlerLegacy(t *testing.T) {
	dockerRegistryUrl = trustedRegistry
	whitelistRegistries = dockerRegistryUrl
	whitelistedRegistries = strings.Split(whitelistRegistries, ",")
	testHandler(t)
}

func TestHandlerPolicy(t *testing.T) {
	policy, _ = NewPolicy()
	policy.Load([]byte(`
rules:
- pattern: ^` + trustedRegistry + `/.*
- pattern: (.*)
  replacement: ` + trustedRegistry + `/$1
`))
	defer func() {
		policy = nil
	}()
	testHandler(t)
}

func testHandler(t *testing.T) {
	whitelistNamespaces = "kube-system"
	whitelistedNamespaces = strings.Split(whitelistNamespaces, ",")
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(tt.reqMethod, tt.reqPath, strings.NewReader(tt.reqBody))
			if err != nil {
				t.Fatal(err)
			}

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(tt.handler)

			handler.ServeHTTP(rr, req)

			// Check the status code is what we expect.
			if status := rr.Code; status != tt.expectStatus {
				t.Errorf("handler returned wrong status code: got %v want %v",
					status, tt.expectStatus)
			}

			// Check the response body is what we expect.
			if rr.Body.String() != tt.expectBody {
				t.Errorf("handler returned unexpected body: got %v want %v",
					rr.Body.String(), tt.expectBody)
			}
		})
	}
}

func runMockRegistry() func() {
	httpmock.Activate()
	httpmock.RegisterResponder("GET", "https://index.docker.io/v2/",
		httpmock.NewStringResponder(http.StatusOK, `{}`))
	httpmock.RegisterResponder("GET", "https://index.docker.io/v2/library/nginx/manifests/latest",
		httpmock.NewStringResponder(http.StatusOK, `{}`))
	httpmock.RegisterResponder("GET", "https://index.docker.io/v2/jainishshah17/nginx/manifests/latest",
		httpmock.NewStringResponder(http.StatusOK, `{}`))
	httpmock.RegisterResponder("GET", "https://index.docker.io/v2/jainishshah17/nginx/manifests/notexist",
		httpmock.NewStringResponder(http.StatusNotFound, `{"errors":[{"code":"MANIFEST_UNKNOWN","message":"manifest unknown","detail":{"Tag":"notexist"}}]}`))
	return httpmock.DeactivateAndReset
}

func Test_imageExists(t *testing.T) {
	tests := []struct {
		name  string
		image string
		want  bool
	}{
		{
			name:  "happy",
			image: "nginx",
			want:  true,
		},
		{
			name:  "doesn't exist",
			image: "jainishshah17/nginx:notexist",
			want:  false,
		},
		{
			name:  "doesn't parse",
			image: "doesn't parse",
			want:  false,
		},
	}
	defer runMockRegistry()()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := imageExists(tt.image); got != tt.want {
				t.Errorf("imageExists() = %v, want %v", got, tt.want)
			}
		})
	}
}

func runMockSlack() func() {
	httpmock.Activate()
	httpmock.RegisterResponder("POST", mockSlackURL,
		httpmock.NewStringResponder(http.StatusOK, `ok`))
	httpmock.RegisterResponder("POST", mockSlackURL+"error",
		httpmock.NewStringResponder(http.StatusBadRequest, `invalid arguments`))
	return httpmock.DeactivateAndReset
}

func TestSendSlackNotification(t *testing.T) {
	defaultEnv := env
	defaultSlackDupeCache := slackDupeCache
	defaultWebhookURL := webhookUrl
	sharedDupeCache := cache.New(time.Minute, time.Minute)
	tests := []struct {
		name           string
		msg            string
		env            string
		slackDupeCache *cache.Cache
		webhookURL     string
	}{
		{
			name:       "disabled",
			webhookURL: "",
		},
		{
			name:       "happy",
			msg:        "foo does not exist in private registry",
			webhookURL: mockSlackURL,
		},
		{
			name:       "with env",
			msg:        "foo does not exist in private registry",
			env:        "dev-1",
			webhookURL: mockSlackURL,
		},
		{
			name:           "with dupe cache miss",
			msg:            "foo does not exist in private registry",
			slackDupeCache: sharedDupeCache,
			webhookURL:     mockSlackURL,
		},
		{
			name:           "with dupe cache hit",
			msg:            "foo does not exist in private registry",
			slackDupeCache: sharedDupeCache,
			webhookURL:     mockSlackURL,
		},
		{
			name:       "slack connection error",
			webhookURL: "example.com",
		},
		{
			name:       "slack response error",
			webhookURL: mockSlackURL + "error",
		},
		{
			name:       "build request error",
			webhookURL: "://",
		},
	}
	defer runMockSlack()()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env = tt.env
			slackDupeCache = tt.slackDupeCache
			webhookUrl = tt.webhookURL
			defer func() {
				env = defaultEnv
				slackDupeCache = defaultSlackDupeCache
				webhookUrl = defaultWebhookURL
			}()
			SendSlackNotification(tt.msg)
		})
	}
}

func init() {
	log = logging.New("debug")
}
