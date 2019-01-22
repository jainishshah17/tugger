# Tugger

### What does Tugger do?
Tugger is Kubernetes Admission webhook to enforce pulling of docker images from private registry.

### Prerequisites

Kubernetes 1.9.0 or above with the `admissionregistration.k8s.io/v1beta1` API enabled. Verify that by the following command:
```
kubectl api-versions | grep admissionregistration.k8s.io/v1beta1
```
The result should be:
```
admissionregistration.k8s.io/v1beta1
```

In addition, the `MutatingAdmissionWebhook` and `ValidatingAdmissionWebhook` admission controllers should be added and listed in the correct order in the admission-control flag of kube-apiserver.

### Build Tugger

```bash
# Build docker image
docker build -t jainishshah17/tugger:0.0.1 .

# Push it to Docker Registry
docker push jainishshah17/tugger:0.0.1
```

### Deploy Tugger to Kubernetes

```bash
# Run deployment
kubectl create -f deployment/tugger-deployment.yaml

# Create service
kubectl create -f  deployment/tugger-svc.yaml
```

### Configure `MutatingAdmissionWebhook` and `ValidatingAdmissionWebhook`

```bash
# Configure MutatingAdmissionWebhook
kubectl create -f webhook/tugger-mutating-webhook-configuration.yaml 

# Configure ValidatingAdmissionWebhook
kubectl create -f webhook/tugger-validating-webhook-configuration.yaml 
```
