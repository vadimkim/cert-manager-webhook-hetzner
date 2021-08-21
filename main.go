package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/jetstack/cert-manager/pkg/acme/webhook/apis/acme/v1alpha1"
	"github.com/jetstack/cert-manager/pkg/acme/webhook/cmd"
	"github.com/vadimkim/cert-manager-webhook-hetzner/internal"
	extapi "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
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
	ZoneName string  `json:"zoneName"`
	ApiUrl string	 `json:"apiUrl"`
}

func (c *hetznerDNSProviderSolver) Name() string {
	return "hetzner"
}

func (c *hetznerDNSProviderSolver) Present(ch *v1alpha1.ChallengeRequest) error {
	klog.V(6).Infof("call function Present: namespace=%s, zone=%s, fqdn=%s", ch.ResourceNamespace, ch.ResolvedZone, ch.ResolvedFQDN)

	config, err := clientConfig(c, ch)

	if err != nil {
		return fmt.Errorf("unable to get secret `%s`; %v", ch.ResourceNamespace, err)
	}

	addTxtRecord(config, ch)

	klog.Infof("Presented txt record %v", ch.ResolvedFQDN)

	return nil
}


func (c *hetznerDNSProviderSolver) CleanUp(ch *v1alpha1.ChallengeRequest) error {
	config, err := clientConfig(c, ch)

	if err != nil {
		return fmt.Errorf("unable to get secret `%s`; %v", ch.ResourceNamespace, err)
	}

	zoneId, err := searchZoneId(config)

	if err != nil {
		return fmt.Errorf("unable to find id for zone name `%s`; %v", config.ZoneName, err)
	}

	var url = config.ApiUrl + "/records?zone_id=" + zoneId

	// Get all DNS records
	dnsRecords, err := callDnsApi(url, "GET", nil, config)

	if err != nil {
		return fmt.Errorf("unable to get DNS records %v", err)
	}

	// Unmarshall response
	records := internal.RecordResponse{}
	readErr := json.Unmarshal(dnsRecords, &records)

	if readErr != nil {
		return fmt.Errorf("unable to unmarshal response %v", readErr)
	}

	var recordId string
	name := recordName(ch.ResolvedFQDN, config.ZoneName)
	for i := len(records.Records) - 1; i >= 0; i-- {
		if records.Records[i].Name == name {
			recordId = records.Records[i].Id
			break
		}
	}

	// Delete TXT record
	url = config.ApiUrl + "/records/" + recordId
	del, err := callDnsApi(url, "DELETE", nil, config)

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

func addTxtRecord(config internal.Config, ch *v1alpha1.ChallengeRequest) {
	url := config.ApiUrl + "/records"

	name := recordName(ch.ResolvedFQDN, config.ZoneName)
	zoneId, err := searchZoneId(config)

	if err != nil {
		klog.Errorf("unable to find id for zone name `%s`; %v", config.ZoneName, err)
	}

	var jsonStr = fmt.Sprintf(`{"value":"%s", "ttl":120, "type":"TXT", "name":"%s", "zone_id":"%s"}`, ch.Key, name, zoneId)

	add, err := callDnsApi(url, "POST", bytes.NewBuffer([]byte(jsonStr)), config)

	if err != nil {
		klog.Error(err)
	}
	klog.Infof("Added TXT record result: %s", string (add))
}

func clientConfig(c *hetznerDNSProviderSolver, ch *v1alpha1.ChallengeRequest) (internal.Config, error) {
	var config internal.Config

	cfg, err := loadConfig(ch.Config)
	if err != nil {
		return config, err
	}
	config.ZoneName = cfg.ZoneName
	config.ApiUrl = cfg.ApiUrl

	secretName := cfg.SecretRef
	sec, err := c.client.CoreV1().Secrets(ch.ResourceNamespace).Get(secretName, metav1.GetOptions{})

	if err != nil {
		return config, fmt.Errorf("unable to get secret `%s/%s`; %v", secretName, ch.ResourceNamespace, err)
	}

	apiKey, err := stringFromSecretData(&sec.Data, "api-key")
	config.ApiKey = apiKey

	if err != nil {
		return config, fmt.Errorf("unable to get api-key from secret `%s/%s`; %v", secretName, ch.ResourceNamespace, err)
	}

	// Get ZoneName by api search if not provided by config
	if config.ZoneName == "" {
		foundZone, err := searchZoneName(config, ch.ResolvedZone)
		if err!= nil {
			return config, err
		}
		config.ZoneName = foundZone
	}

	return config, nil
}

/*
Domain name in Hetzner is divided in 2 parts: record + zone name. API works
with record name that is FQDN without zone name. Sub-domains is a part of
record name and is separated by "."
 */
func recordName (fqdn string, domain string) string {
	r := regexp.MustCompile("(.+)\\." + domain + "\\.")
	name := r.FindStringSubmatch(fqdn)
	if len(name) != 2 {
		klog.Errorf("splitting domain name %s failed!", fqdn)
		return ""
	}
	return name[1]
}

func callDnsApi (url string, method string, body io.Reader, config internal.Config) ([]byte, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return []byte{}, fmt.Errorf("unable to execute request %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Auth-API-Token", config.ApiKey)

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

	text := "Error calling API status:" + resp.Status + " url: " +  url + " method: " + method
	klog.Error(text)
	return nil, errors.New(text)
}

func searchZoneId(config internal.Config) (string, error) {
	url := config.ApiUrl + "/zones?name=" + config.ZoneName

	// Get Zone configuration
	zoneRecords, err := callDnsApi(url, "GET", nil, config)

	if err != nil {
		return "", fmt.Errorf("unable to get zone info %v", err)
	}

	// Unmarshall response
	zones := internal.ZoneResponse{}
	readErr := json.Unmarshal(zoneRecords, &zones)

	if readErr != nil {
		return "", fmt.Errorf("unable to unmarshal response %v", readErr)
	}

	if zones.Meta.Pagination.TotalEntries != 1 {
		return "", fmt.Errorf("wrong number of zones in response %d must be exactly = 1", zones.Meta.Pagination.TotalEntries)
	}
	return zones.Zones[0].Id, nil
}

func searchZoneName(config internal.Config, searchZone string) (string, error) {
	parts := strings.Split(searchZone, ".")
	parts = parts[:len(parts)-1]
	for i := 0; i <= len(parts) - 2; i++ {
		config.ZoneName = strings.Join(parts[i:], ".")
		zoneId, _ := searchZoneId(config)
		if zoneId != "" {
			klog.Infof("Found ID with ZoneName: %s", config.ZoneName)
			return config.ZoneName, nil
		}
	}
	return "", fmt.Errorf("unable to find hetzner dns zone with: %s", searchZone)
}