package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jetstack/cert-manager-webhook-hetzner/internal"
	"github.com/jetstack/cert-manager/pkg/acme/webhook/apis/acme/v1alpha1"
	"github.com/jetstack/cert-manager/pkg/acme/webhook/cmd"
	"io"
	"io/ioutil"
	extapi "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
	"net/http"
	"os"
	"regexp"
)

var GroupName = os.Getenv("GROUP_NAME")

func main() {
	if GroupName == "" {
		panic("GROUP_NAME must be specified")
	}

	cmd.RunWebhookServer(GroupName,
		&hetznerDNSProviderSolver{},
	)
}

type hetznerDNSProviderSolver struct {
	client *kubernetes.Clientset
}

type hetznerDNSProviderConfig struct {
	SecretRef string `json:"secretName"`
}

func (c *hetznerDNSProviderSolver) Name() string {
	return "hetzner"
}

func (c *hetznerDNSProviderSolver) Present(ch *v1alpha1.ChallengeRequest) error {
	klog.V(6).Infof("call function Present: namespace=%s, zone=%s, fqdn=%s", ch.ResourceNamespace, ch.ResolvedZone, ch.ResolvedFQDN)

	secret, err := hetznerSecret(c, ch)

	if err != nil {
		return fmt.Errorf("unable to get secret `%s`; %v", ch.ResourceNamespace, err)
	}

	addTxtRecord(secret, ch)

	klog.Infof("Presented txt record %v", ch.ResolvedFQDN)

	return nil
}

// CleanUp should delete the relevant TXT record from the DNS provider console.
// If multiple TXT records exist with the same record name (e.g.
// _acme-challenge.example.com) then **only** the record with the same `key`
// value provided on the ChallengeRequest should be cleaned up.
// This is in order to facilitate multiple DNS validations for the same domain
// concurrently.
func (c *hetznerDNSProviderSolver) CleanUp(ch *v1alpha1.ChallengeRequest) error {
	secret, err := hetznerSecret(c, ch)

	if err != nil {
		return fmt.Errorf("unable to get secret `%s`; %v", ch.ResourceNamespace, err)
	}
	var url = "https://dns.hetzner.com/api/v1/records?zone_id=" + secret.ZoneId

	// Get all DNS records
	dnsRecords, err := callDnsApi(url, "GET", nil, secret)

	if err != nil {
		panic(err)
	}

	// Unmarshall response
	records := internal.RecordResponse{}
	readErr := json.Unmarshal(dnsRecords, &records)

	if readErr != nil {
		panic(readErr)
	}

	var recordId string
	name := recordName(ch.ResolvedFQDN)
	for i := len(records.Records) - 1; i >= 0; i-- {
		if records.Records[i].Name == name {
			recordId = records.Records[i].Id
			break
		}
	}

	// Delete TXT record
	url = "https://dns.hetzner.com/api/v1/records/" + recordId
	del, err := callDnsApi(url, "DELETE", nil, secret)

	if err != nil {
		klog.Error(err)
	}
	klog.Infof("Delete TXT record result: %s", string(del))
	return nil
}

func (c *hetznerDNSProviderSolver) Initialize(kubeClientConfig *rest.Config, stopCh <-chan struct{}) error {
	k8sClient, err := kubernetes.NewForConfig(kubeClientConfig)
	klog.V(6).Infof("Input variable stopCh is %d length", len(stopCh))
	if err != nil {
		return err
	}

	c.client = k8sClient

	return nil
}

func loadConfig(cfgJSON *extapi.JSON) (hetznerDNSProviderConfig, error) {
	cfg := hetznerDNSProviderConfig{}
	// handle the 'base case' where no configuration has been provided
	if cfgJSON == nil {
		return cfg, nil
	}
	if err := json.Unmarshal(cfgJSON.Raw, &cfg); err != nil {
		return cfg, fmt.Errorf("error decoding solver config: %v", err)
	}

	return cfg, nil
}

func stringFromSecretData(secretData *map[string][]byte, key string) (string, error) {
	data, ok := (*secretData)[key]
	if !ok {
		return "", fmt.Errorf("key %q not found in secret data", key)
	}
	return string(data), nil
}

func addTxtRecord(secret internal.Secret, ch *v1alpha1.ChallengeRequest) {
	url := "https://dns.hetzner.com/api/v1/records"

	name := recordName(ch.ResolvedFQDN)

	var jsonStr = fmt.Sprintf(`{"value":"%s", "ttl":120, "type":"TXT", "name":"%s", "zone_id":"%s"}`, ch.Key, name, secret.ZoneId)

	add, err := callDnsApi(url, "POST", bytes.NewBuffer([]byte(jsonStr)), secret)

	if err != nil {
		klog.Error(err)
	}
	klog.Infof("Added TXT record result: %s", string (add))
}

func hetznerSecret(c *hetznerDNSProviderSolver, ch *v1alpha1.ChallengeRequest) (internal.Secret, error) {
	var secret internal.Secret

	cfg, err := loadConfig(ch.Config)
	if err != nil {
		return secret, err
	}

	secretName := cfg.SecretRef
	sec, err := c.client.CoreV1().Secrets(ch.ResourceNamespace).Get(secretName, metav1.GetOptions{})

	if err != nil {
		return secret, fmt.Errorf("unable to get secret `%s/%s`; %v", secretName, ch.ResourceNamespace, err)
	}

	apiKey, err := stringFromSecretData(&sec.Data, "api-key")
	secret.ApiKey = apiKey
	if err != nil {
		return secret, fmt.Errorf("unable to get api-key from secret `%s/%s`; %v", secretName, ch.ResourceNamespace, err)
	}

	zoneId, err := stringFromSecretData(&sec.Data, "zone-id")
	secret.ZoneId = zoneId
	if err != nil {
		return secret, fmt.Errorf("unable to get zone-id from secret `%s/%s`; %v", secretName, ch.ResourceNamespace, err)
	}
	return secret, nil
}

func recordName (fqdn string) string {
	r := regexp.MustCompile("(.+)\\.(.+)\\.(.+)\\.")
	name := r.FindStringSubmatch(fqdn)
	if len(name) != 4 {
		panic("Splitting domain name failed! " + fqdn)
	}
	return name[1]
}

func callDnsApi (url string, method string, body io.Reader, secret internal.Secret) ([]byte, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Auth-API-Token", secret.ApiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer func() {
		err := resp.Body.Close()
		if err != nil {
			klog.Fatal(err)
		}
	}()

	respBody, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusOK {
		return respBody, nil
	}

	text := "Error calling API status:" + resp.Status + " reason: " +  string(respBody)
	klog.Error(text)
	return nil, errors.New(text)
}