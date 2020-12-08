# Tugger

### What does Tugger do?
Tugger is Kubernetes Admission webhook to enforce pulling of docker images from private registry.

### Note:
Tugger has graduated. Tugger's new home is [JFrog/Tugger](https://github.com/jfrog/tugger). 
JFrog is actively maintaining tugger.

### Prerequisites

Kubernetes 1.9.0 or above with the `admissionregistration.k8s.io/v1` API enabled. Verify that by the following command:
```
kubectl api-versions | grep admissionregistration.k8s.io/v1beta1
```
The result should be:
```
admissionregistration.k8s.io/v1beta1
```

In addition, the `MutatingAdmissionWebhook` and `ValidatingAdmissionWebhook` admission controllers should be added and listed in the correct order in the admission-control flag of kube-apiserver.

### Build and Push Tugger Docker Image

```bash
# Build docker image
docker build -t jainishshah17/tugger:0.0.7 .

# Push it to Docker Registry
docker push jainishshah17/tugger:0.0.7
```

### Create [Kubernetes Docker registry secret](https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/)

```bash
# Create a Docker registry secret called 'regsecret'
kubectl create secret docker-registry regsecret --docker-server=${DOCKER_REGISTRY} --docker-username=${DOCKER_USER} --docker-password=${DOCKER_PASS} --docker-email=${DOCKER_EMAIL}
```

**Note**: Create Docker registry secret in each non-whitelisted namespaces.

### Generate TLS Certs for Tugger

```bash
./tls/gen-cert.sh
```

### Get CA Bundle

```bash
./webhook/webhook-patch-ca-bundle.sh
```

### Deploy Tugger to Kubernetes

#### Deploy using Helm Chart

The helm chart can generate certificates and configure webhooks in a single step.  See the notes on webhooks below for more information.

```bash
helm install --name tugger \
  --set docker.registrySecret=regsecret, \
  --set docker.registryUrl=jainishshah17, \
  --set whitelistNamespaces={kube-system,default}, \
  --set whitelistRegistries={jainishshah17} \
  --set createValidatingWebhook=true \
  --set createMutatingWebhook=true \
  chart/tugger
```

#### Deploy using kubectl

1. Create deployment and service

	```bash
	# Run deployment
	kubectl create -f deployment/tugger-deployment.yaml
	
	# Create service
	kubectl create -f  deployment/tugger-svc.yaml
	```

2. Configure `MutatingAdmissionWebhook` and `ValidatingAdmissionWebhook`

	**Note**: Replace `${CA_BUNDLE}` with value generated by running `./webhook/webhook-patch-ca-bundle.sh`

	```bash
	# re MutatingAdmissionWebhook
	kubectl create -f webhook/tugger-mutating-webhook ration.yaml 
	```

	Note: Use MutatingAdmissionWebhook only if you want to enforce pulling of docker image from Private Docker Registry e.g [JFrog Artifactory](https://jfrog.com/artifactory/).
	If your container image is `nginx` then Tugger will append `REGISTRY_URL` to it. e.g `nginx` will become `jainishshah17/nginx`

	```bash
	# Configure ValidatingWebhookConfiguration
	kubectl create -f webhook/tugger-validating-webhook ration.yaml 
	```

	Note: Use ValidatingWebhookConfiguration only if you want to check pulling of docker image from Private Docker Registry e.g [JFrog Artifactory](https://jfrog.com/artifactory/).
	If your container image does not contain `REGISTRY_URL` then Tugger will deny request to run that pod.

### Test Tugger

```bash
# Deploy nginx 
kubectl apply -f test/nginx.yaml 
```



