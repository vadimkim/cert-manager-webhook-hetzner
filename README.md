# ACME webhook for Hetzner DNS API

This solver can be used when you want to use cert-manager with Hetzner DNS API. API documentation
is [here](https://dns.hetzner.com/api-docs)

## Requirements

- [go](https://golang.org/) >= 1.13.0
- [helm](https://helm.sh/) >= v3.0.0
- [kubernetes](https://kubernetes.io/) >= v1.14.0
- [cert-manager](https://cert-manager.io/) >= 0.12.0

## Installation

### cert-manager

Follow the [instructions](https://cert-manager.io/docs/installation/) using the cert-manager documentation to install it
within your cluster.

### Webhook

#### Using public helm chart

```bash
helm repo add cert-manager-webhook-hetzner https://vadimkim.github.io/cert-manager-webhook-hetzner
helm install --namespace cert-manager cert-manager-webhook-hetzner cert-manager-webhook-hetzner/cert-manager-webhook-hetzner
```

#### From local checkout

```bash
helm install --namespace cert-manager cert-manager-webhook-hetzner deploy/cert-manager-webhook-hetzner
```

**Note**: The kubernetes resources used to install the Webhook should be deployed within the same namespace as the
cert-manager.

To uninstall the webhook run

```bash
helm uninstall --namespace cert-manager cert-manager-webhook-hetzner
```

## Issuer

Create a `ClusterIssuer` or `Issuer` resource as following:
(Keep in Mind that the Example uses the Staging URL from Let's Encrypt. Look
at [Getting Start](https://letsencrypt.org/getting-started/) for using the normal Let's Encrypt URL.)

```yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-staging
spec:
  acme:
    # The ACME server URL
    server: https://acme-staging-v02.api.letsencrypt.org/directory

    # Email address used for ACME registration
    email: mail@example.com # REPLACE THIS WITH YOUR EMAIL!!!

    # Name of a secret used to store the ACME account private key
    privateKeySecretRef:
      name: letsencrypt-staging

    solvers:
      - dns01:
          webhook: 
            groupName: hetzner.cert-mananger-webhook.noshoes.xyz
            solverName: hetzner
            config:
              secretName: hetzner-secret
              zoneName: example.com # (Optional): When not provided the Zone will searched in Hetzner API by recursion on full domain name
              apiUrl: https://dns.hetzner.com/api/v1
```

### Credentials

In order to access the Hetzner API, the webhook needs an API token.

If you choose another name for the secret than `hetzner-secret`, you must install the chart with a modified `secretName`
value. Policies ensure that no other secrets can be read by the webhook. Also modify the value of `secretName` in the
`[Cluster]Issuer`.

The secret for the example above will look like this:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: hetzner-secret
  namespace: cert-manager
type: Opaque
data:
  api-key: your-key-base64-encoded
```

### Create a certificate

Finally you can create certificates, for example:

```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: example-cert
  namespace: cert-manager
spec:
  commonName: example.com
  dnsNames:
    - example.com
  issuerRef:
    name: letsencrypt-staging
    kind: ClusterIssuer
  secretName: example-cert
```

## Development

### Running the test suite

All DNS providers **must** run the DNS01 provider conformance testing suite,
else they will have undetermined behaviour when used with cert-manager.

**It is essential that you configure and run the test suite when creating a
DNS01 webhook.**

First, you need to have Hetzner account with access to DNS control panel. You need to create API token and have a
registered and verified DNS zone there.
Then you need to replace `zoneName` parameter at `testdata/hetzner/config.json` file with actual one.
You also must encode your api token into base64 and put the hash into `testdata/hetzner/hetzner-secret.yml` file.

You can then run the test suite with:

```bash
# first install necessary binaries (only required once)
./scripts/fetch-test-binaries.sh
# then run the tests
TEST_ZONE_NAME=example.com. make verify
```

## Creating new package

To build new Docker image for multiple architectures and push it to hub:

```shell
docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 -t zmejg/cert-manager-webhook-hetzner:1.2.0 . --push
```

To compile and publish new Helm chart version:

```shell
helm package deploy/cert-manager-webhook-hetzner
git checkout gh-pages
helm repo index . --url https://vadimkim.github.io/cert-manager-webhook-hetzner/
```
