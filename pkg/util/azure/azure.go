package azure

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/Azure/go-autorest/autorest"
	azureenv "github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/azure/auth"

	"yunion.io/x/jsonutils"
	"yunion.io/x/log"
	"yunion.io/x/onecloud/pkg/cloudprovider"
	"yunion.io/x/onecloud/pkg/compute/models"
	"yunion.io/x/onecloud/pkg/httperrors"
)

const (
	CLOUD_PROVIDER_AZURE    = models.CLOUD_PROVIDER_AZURE
	CLOUD_PROVIDER_AZURE_CN = "微软"

	AZURE_API_VERSION = "2016-02-01"
)

type SAzureClient struct {
	client              autorest.Client
	providerId          string
	providerName        string
	subscriptionId      string
	tenantId            string
	clientId            string
	clientScret         string
	domain              string
	baseUrl             string
	secret              string
	envName             string
	ressourceGroups     []SResourceGroup
	fetchResourceGroups bool
	subscriptionName    string
	env                 azureenv.Environment
	authorizer          autorest.Authorizer
	iregions            []cloudprovider.ICloudRegion
}

var DEFAULT_API_VERSION = map[string]string{
	"vmSizes": "2018-06-01", //2015-05-01-preview,2015-06-15,2016-03-30,2016-04-30-preview,2016-08-30,2017-03-30,2017-12-01,2018-04-01,2018-06-01,2018-10-01
	"Microsoft.Compute/virtualMachineScaleSets":      "2017-12-01",
	"Microsoft.Compute/virtualMachines":              "2018-04-01",
	"Microsoft.ClassicCompute/virtualMachines":       "2017-04-01",
	"Microsoft.Compute/operations":                   "2018-10-01",
	"Microsoft.ClassicCompute/operations":            "2017-04-01",
	"Microsoft.Network/virtualNetworks":              "2018-08-01",
	"Microsoft.ClassicNetwork/virtualNetworks":       "2017-11-15", //avaliable 2014-01-01,2014-06-01,2015-06-01,2015-12-01,2016-04-01,2016-11-01,2017-11-15
	"Microsoft.Compute/disks":                        "2018-06-01", //avaliable 2016-04-30-preview,2017-03-30,2018-04-01,2018-06-01
	"Microsoft.Storage/storageAccounts":              "2016-12-01", //2018-03-01-preview,2018-02-01,2017-10-01,2017-06-01,2016-12-01,2016-05-01,2016-01-01,2015-06-15,2015-05-01-preview
	"Microsoft.ClassicStorage/storageAccounts":       "2016-04-01", //2014-01-01,2014-04-01,2014-04-01-beta,2014-06-01,2015-06-01,2015-12-01,2016-04-01,2016-11-01
	"Microsoft.Compute/snapshots":                    "2018-06-01", //2016-04-30-preview,2017-03-30,2018-04-01,2018-06-01
	"Microsoft.Compute/images":                       "2018-10-01", //2016-04-30-preview,2016-08-30,2017-03-30,2017-12-01,2018-04-01,2018-06-01,2018-10-01
	"Microsoft.Storage":                              "2016-12-01", //2018-03-01-preview,2018-02-01,2017-10-01,2017-06-01,2016-12-01,2016-05-01,2016-01-01,2015-06-15,2015-05-01-preview
	"Microsoft.Network/publicIPAddresses":            "2018-06-01", //2014-12-01-preview, 2015-05-01-preview, 2015-06-15, 2016-03-30, 2016-06-01, 2016-07-01, 2016-08-01, 2016-09-01, 2016-10-01, 2016-11-01, 2016-12-01, 2017-03-01, 2017-04-01, 2017-06-01, 2017-08-01, 2017-09-01, 2017-10-01, 2017-11-01, 2018-01-01, 2018-02-01, 2018-03-01, 2018-04-01, 2018-05-01, 2018-06-01, 2018-07-01, 2018-08-01
	"Microsoft.Network/networkSecurityGroups":        "2018-06-01",
	"Microsoft.Network/networkInterfaces":            "2018-06-01", //2014-12-01-preview, 2015-05-01-preview, 2015-06-15, 2016-03-30, 2016-06-01, 2016-07-01, 2016-08-01, 2016-09-01, 2016-10-01, 2016-11-01, 2016-12-01, 2017-03-01, 2017-04-01, 2017-06-01, 2017-08-01, 2017-09-01, 2017-10-01, 2017-11-01, 2018-01-01, 2018-02-01, 2018-03-01, 2018-04-01, 2018-05-01, 2018-06-01, 2018-07-01, 2018-08-01
	"Microsoft.Network":                              "2018-06-01",
	"Microsoft.ClassicNetwork/reservedIps":           "2016-04-01", //2014-01-01,2014-06-01,2015-06-01,2015-12-01,2016-04-01,2016-11-01
	"Microsoft.ClassicNetwork/networkSecurityGroups": "2016-11-01", //2015-06-01,2015-12-01,2016-04-01,2016-11-01
}

func NewAzureClient(providerId string, providerName string, accessKey string, secret string, envName string) (*SAzureClient, error) {
	if clientInfo, accountInfo := strings.Split(secret, "/"), strings.Split(accessKey, "/"); len(clientInfo) >= 2 && len(accountInfo) >= 1 {
		client := SAzureClient{providerId: providerId, providerName: providerName, secret: secret, envName: envName}
		client.clientId, client.clientScret = clientInfo[0], strings.Join(clientInfo[1:], "/")
		client.tenantId = accountInfo[0]
		if len(accountInfo) == 2 {
			client.subscriptionId = accountInfo[1]
		}
		err := client.fetchRegions()
		if err != nil {
			return nil, err
		}
		return &client, nil
	}
	return nil, httperrors.NewUnauthorizedError("clientId、clientScret or subscriptId input error")
}

func (self *SAzureClient) getDefaultClient() (*autorest.Client, error) {
	client := autorest.NewClientWithUserAgent("Yunion API")
	conf := auth.NewClientCredentialsConfig(self.clientId, self.clientScret, self.tenantId)
	env, err := azureenv.EnvironmentFromName(self.envName)
	if err != nil {
		return nil, err
	}
	self.env = env
	self.domain = env.ResourceManagerEndpoint
	conf.Resource = env.ResourceManagerEndpoint
	conf.AADEndpoint = env.ActiveDirectoryEndpoint
	authorizer, err := conf.Authorizer()
	if err != nil {
		return nil, err
	}
	client.Authorizer = authorizer
	// client.RequestInspector = LogRequest()
	// client.ResponseInspector = LogResponse()
	return &client, nil
}

func (self *SAzureClient) jsonRequest(method, url string, body string) (jsonutils.JSONObject, error) {
	cli, err := self.getDefaultClient()
	if err != nil {
		return nil, err
	}
	return jsonRequest(cli, method, self.domain, url, body)
}

func (self *SAzureClient) Get(resourceId string, params []string, retVal interface{}) error {
	if len(resourceId) == 0 {
		return cloudprovider.ErrNotFound
	}
	path := resourceId
	if len(params) > 0 {
		path += fmt.Sprintf("?%s", strings.Join(params, "&"))
	}
	cli, err := self.getDefaultClient()
	if err != nil {
		return err
	}
	body, err := jsonRequest(cli, "GET", self.domain, path, "")
	if err != nil {
		return err
	}
	err = body.Unmarshal(retVal)
	if err != nil {
		return err
	}
	return nil
}

func (self *SAzureClient) ListVmSizes(location string) (jsonutils.JSONObject, error) {
	cli, err := self.getDefaultClient()
	if err != nil {
		return nil, err
	}
	if len(self.subscriptionId) == 0 {
		return nil, fmt.Errorf("need subscription id")
	}
	url := fmt.Sprintf("/subscriptions/%s/providers/Microsoft.Compute/locations/%s/vmSizes", self.subscriptionId, location)
	return jsonRequest(cli, "GET", self.domain, url, "")
}

func (self *SAzureClient) ListClassicDisks() (jsonutils.JSONObject, error) {
	cli, err := self.getDefaultClient()
	if err != nil {
		return nil, err
	}
	if len(self.subscriptionId) == 0 {
		return nil, fmt.Errorf("need subscription id")
	}
	url := fmt.Sprintf("/subscriptions/%s/services/disks", self.subscriptionId)
	return jsonRequest(cli, "GET", self.domain, url, "")
}

func (self *SAzureClient) ListAll(resourceType string, retVal interface{}) error {
	cli, err := self.getDefaultClient()
	if err != nil {
		return err
	}
	url := "/subscriptions"
	if len(self.subscriptionId) > 0 {
		url += fmt.Sprintf("/%s", self.subscriptionId)
	}
	if len(resourceType) > 0 {
		url += fmt.Sprintf("/providers/%s", resourceType)
	}
	body, err := jsonRequest(cli, "GET", self.domain, url, "")
	if err != nil {
		return err
	}
	if retVal != nil {
		body.Unmarshal(retVal, "value")
	}
	return nil
}

func (self *SAzureClient) ListSubscriptions() (jsonutils.JSONObject, error) {
	cli, err := self.getDefaultClient()
	if err != nil {
		return nil, err
	}
	return jsonRequest(cli, "GET", self.domain, "/subscriptions", "")
}

func (self *SAzureClient) List(golbalResource string, retVal interface{}) error {
	cli, err := self.getDefaultClient()
	if err != nil {
		return err
	}
	url := "/subscriptions"
	if len(self.subscriptionId) > 0 {
		url += fmt.Sprintf("/%s", self.subscriptionId)
	}
	if len(self.subscriptionId) > 0 && len(golbalResource) > 0 {
		url += fmt.Sprintf("/%s", golbalResource)
	}
	body, err := jsonRequest(cli, "GET", self.domain, url, "")
	if err != nil {
		return err
	}
	return body.Unmarshal(retVal, "value")
}

func (self *SAzureClient) ListByTypeWithResourceGroup(resourceGroupName string, Type string, retVal interface{}) error {
	cli, err := self.getDefaultClient()
	if err != nil {
		return err
	}
	if len(self.subscriptionId) == 0 {
		return fmt.Errorf("Missing subscription Info")
	}
	url := fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/%s", self.subscriptionId, resourceGroupName, Type)
	body, err := jsonRequest(cli, "GET", self.domain, url, "")
	if err != nil {
		return err
	}
	return body.Unmarshal(retVal, "value")
}

func (self *SAzureClient) Delete(resourceId string) error {
	cli, err := self.getDefaultClient()
	if err != nil {
		return err
	}
	_, err = jsonRequest(cli, "DELETE", self.domain, resourceId, "")
	return err
}

func (self *SAzureClient) PerformAction(resourceId string, action string, body string) (jsonutils.JSONObject, error) {
	cli, err := self.getDefaultClient()
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf("%s/%s", resourceId, action)
	return jsonRequest(cli, "POST", self.domain, url, body)
}

func (self *SAzureClient) fetchResourceGroup(cli *autorest.Client, location string) error {
	if !self.fetchResourceGroups {
		err := self.List("resourcegroups", &self.ressourceGroups)
		if err != nil {
			log.Errorf("failed to list resourceGroups: %v", err)
			return err
		}
		self.fetchResourceGroups = true
	}
	if len(self.ressourceGroups) == 0 {
		//Create Default resourceGroup
		_url := fmt.Sprintf("/subscriptions/%s/resourcegroups/Default", self.subscriptionId)
		body, err := jsonRequest(cli, "PUT", self.domain, _url, fmt.Sprintf(`{"name": "Default", "location": "%s"}`, location))
		if err != nil {
			return err
		}
		resourceGroup := SResourceGroup{}
		err = body.Unmarshal(&resourceGroup)
		if err != nil {
			return err
		}
		self.ressourceGroups = []SResourceGroup{resourceGroup}
	}
	return nil
}

func (self *SAzureClient) checkParams(body jsonutils.JSONObject, params []string) (map[string]string, error) {
	result := map[string]string{}
	for i := 0; i < len(params); i++ {
		data, err := body.GetString(params[i])
		if err != nil {
			return nil, fmt.Errorf("Missing %s params")
		}
		result[params[i]] = data
	}
	return result, nil
}

func (self *SAzureClient) Create(body jsonutils.JSONObject, retVal interface{}) error {
	cli, err := self.getDefaultClient()
	if err != nil {
		return err
	}
	if len(self.subscriptionId) == 0 {
		return fmt.Errorf("Missing subscription info")
	}
	params, err := self.checkParams(body, []string{"type", "name", "location"})
	if err != nil {
		return fmt.Errorf("Azure create resource failed: %s", err.Error())
	}
	err = self.fetchResourceGroup(cli, params["location"])
	if err != nil {
		return err
	}
	if len(self.ressourceGroups) == 0 {
		return fmt.Errorf("Create Default resourceGroup error?")
	}
	url := fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/%s/%s", self.subscriptionId, self.ressourceGroups[0].Name, params["type"], params["name"])
	result, err := jsonRequest(cli, "PUT", self.domain, url, body.String())
	if err != nil {
		return err
	}
	return result.Unmarshal(retVal)
}

func (self *SAzureClient) CheckNameAvailability(Type string, body string) (jsonutils.JSONObject, error) {
	cli, err := self.getDefaultClient()
	if err != nil {
		return nil, err
	}
	if len(self.subscriptionId) == 0 {
		return nil, fmt.Errorf("Missing subscription ID")
	}
	url := fmt.Sprintf("/subscriptions/%s/providers/%s/checkNameAvailability", self.subscriptionId, Type)
	return jsonRequest(cli, "POST", self.domain, url, body)
}

func (self *SAzureClient) Update(body jsonutils.JSONObject, retVal interface{}) error {
	cli, err := self.getDefaultClient()
	if err != nil {
		return err
	}
	url, err := body.GetString("id")
	result, err := jsonRequest(cli, "PUT", self.domain, url, body.String())
	if err != nil {
		return err
	}
	if retVal != nil {
		return result.Unmarshal(retVal)
	}
	return nil
}

func jsonRequest(client *autorest.Client, method, domain, baseUrl string, body string) (jsonutils.JSONObject, error) {
	return _jsonRequest(client, method, domain, baseUrl, body)
}

func waitForComplatetion(client *autorest.Client, req *http.Request, resp *http.Response, timeout time.Duration) (jsonutils.JSONObject, error) {
	location := resp.Header.Get("Location")
	asyncoperation := resp.Header.Get("Azure-Asyncoperation")
	startTime := time.Now()
	if len(location) > 0 || (len(asyncoperation) > 0 && resp.StatusCode != 200 || strings.Index(req.URL.String(), "enablevmaccess") > 0) {
		if len(asyncoperation) > 0 {
			location = asyncoperation
		}
		for {
			asyncReq, err := http.NewRequest("GET", location, nil)
			if err != nil {
				return nil, err
			}
			asyncResp, err := client.Do(asyncReq)
			if err != nil {
				return nil, err
			}
			if asyncResp.StatusCode == 202 {
				if time.Now().Sub(startTime) > timeout {
					return nil, fmt.Errorf("Process request %s %s timeout", req.Method, req.URL.String())
				}
				time.Sleep(time.Second * 5)
				continue
			}
			if asyncResp.ContentLength == 0 {
				return nil, nil
			}
			data, err := ioutil.ReadAll(asyncResp.Body)
			if err != nil {
				return nil, err
			}
			asyncData, err := jsonutils.Parse(data)
			if err != nil {
				return nil, err
			}
			if len(asyncoperation) > 0 && asyncData.Contains("status") {
				status, _ := asyncData.GetString("status")
				switch status {
				case "InProgress":
					log.Debugf("process %s %s InProgress", req.Method, req.URL.String())
					time.Sleep(time.Second * 5)
					continue
				case "Succeeded":
					log.Debugf("process %s %s Succeeded", req.Method, req.URL.String())
					output, err := asyncData.Get("properties", "output")
					if err == nil {
						return output, nil
					}
					return nil, nil
				case "Failed":
					if asyncData.Contains("error") {
						msg, _ := asyncData.Get("error")
						log.Errorf("process %s %s error: %s", req.Method, req.URL.String(), msg.String())
						return nil, fmt.Errorf(msg.String())
					}
				default:
					log.Errorf("Unknow status %s when process %s %s", status, req.Method, req.URL.String())
					return nil, fmt.Errorf("Unknow status %s", status)
				}
				return nil, fmt.Errorf("Create failed: %s", data)
			}
			log.Debugf("process %s %s return: %s", req.Method, req.URL.String(), data)
			return asyncData, nil
		}
	}
	return nil, nil
}

func _jsonRequest(client *autorest.Client, method, domain, baseURL, body string) (result jsonutils.JSONObject, err error) {
	version := AZURE_API_VERSION
	for resourceType, _version := range DEFAULT_API_VERSION {
		if strings.Index(strings.ToLower(baseURL), strings.ToLower(resourceType)) > 0 {
			version = _version
		}
	}
	url := fmt.Sprintf("%s%s?api-version=%s", domain, baseURL, version)
	if strings.Index(baseURL, "?") > 0 {
		url = fmt.Sprintf("%s%s&api-version=%s", domain, baseURL, version)
	}
	req := &http.Request{}
	if len(body) != 0 {
		req, err = http.NewRequest(method, url, strings.NewReader(body))
		if err != nil {
			log.Errorf("Azure %s new request: %s body: %s error: %v", method, url, body, err)
			return nil, err
		}
	} else {
		req, err = http.NewRequest(method, url, nil)
		if err != nil {
			log.Errorf("Azure %s new request: %s error: %v", method, url, err)
			return nil, err
		}
	}
	req.Header.Add("Content-Type", "application/json; charset=utf-8")
	resp, err := client.Do(req)
	if err != nil {
		log.Errorf("Azure %s request: %s \nbody: %s error: %v", req.Method, req.URL.String(), body, err)
		return nil, err
	}

	if resp.StatusCode == 404 {
		data := []byte{}
		if resp.ContentLength != 0 {
			data, _ = ioutil.ReadAll(resp.Body)
		}
		log.Errorf("failed find %s error: %s", url, string(data))
		return nil, cloudprovider.ErrNotFound
	}
	// 异步任务最多耗时半小时，否则以失败处理
	asyncData, err := waitForComplatetion(client, req, resp, time.Minute*30)
	if err != nil {
		return nil, err
	}
	if asyncData != nil {
		return asyncData, nil
	}
	if resp.ContentLength == 0 {
		return jsonutils.NewDict(), nil
	}
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	_data := strings.Replace(string(data), "\r", "", -1)
	result, err = jsonutils.Parse([]byte(_data))
	if err != nil {
		return nil, err
	}
	if result.Contains("error") {
		log.Errorf("Azure %s request: %s \nbody: %s error: %v", req.Method, req.URL.String(), body, err)
		return nil, fmt.Errorf(result.String())
	}
	return result, nil
}

func (self *SAzureClient) UpdateAccount(tenantId, secret, envName string) error {
	if self.tenantId != tenantId || self.secret != secret || self.envName != envName {
		if clientInfo, accountInfo := strings.Split(secret, "/"), strings.Split(tenantId, "/"); len(clientInfo) >= 2 && len(accountInfo) >= 1 {
			self.clientId, self.clientScret = clientInfo[0], strings.Join(clientInfo[1:], "/")
			self.tenantId = accountInfo[0]
			if len(accountInfo) == 2 {
				self.subscriptionId = accountInfo[1]
			}
			err := self.fetchRegions()
			if err != nil {
				return err
			}
			return nil
		} else {
			return httperrors.NewUnauthorizedError("clientId、clientScret or subscriptId input error")
		}
	}
	return nil
}

func (self *SAzureClient) fetchRegions() error {
	if len(self.subscriptionId) > 0 {
		regions := []SRegion{}
		err := self.List("locations", &regions)
		if err != nil {
			return err
		}
		self.iregions = make([]cloudprovider.ICloudRegion, len(regions))
		for i := 0; i < len(regions); i++ {
			regions[i].client = self
			regions[i].SubscriptionID = self.subscriptionId
			self.iregions[i] = &regions[i]
		}
	}
	body, err := self.ListSubscriptions()
	if err != nil {
		return err
	}
	subscriptions, err := body.GetArray("value")
	if err != nil {
		return err
	}
	for _, subscription := range subscriptions {
		subscriptionId, _ := subscription.GetString("subscriptionId")
		if subscriptionId == self.subscriptionId {
			self.subscriptionName, _ = subscription.GetString("displayName")
			break
		}
	}
	return nil
}

func (self *SAzureClient) GetRegions() []SRegion {
	regions := make([]SRegion, len(self.iregions))
	for i := 0; i < len(regions); i += 1 {
		region := self.iregions[i].(*SRegion)
		regions[i] = *region
	}
	return regions
}

func (self *SAzureClient) GetSubAccounts() (jsonutils.JSONObject, error) {
	body, err := self.ListSubscriptions()
	if err != nil {
		return nil, err
	}
	value, err := body.GetArray("value")
	if err != nil {
		return nil, err
	}
	result := jsonutils.NewDict()
	result.Add(jsonutils.NewInt(int64(len(value))), "total")
	result.Add(jsonutils.NewArray(value...), "data")
	return result, nil
}

func (self *SAzureClient) GetIRegions() []cloudprovider.ICloudRegion {
	return self.iregions
}

func (self *SAzureClient) getDefaultRegion() (cloudprovider.ICloudRegion, error) {
	if len(self.iregions) > 0 {
		return self.iregions[0], nil
	}
	return nil, cloudprovider.ErrNotFound
}

func (self *SAzureClient) GetIRegionById(id string) (cloudprovider.ICloudRegion, error) {
	for i := 0; i < len(self.iregions); i += 1 {
		if self.iregions[i].GetGlobalId() == id {
			return self.iregions[i], nil
		}
	}
	return nil, cloudprovider.ErrNotFound
}

func (self *SAzureClient) GetRegion(regionId string) *SRegion {
	for i := 0; i < len(self.iregions); i += 1 {
		if self.iregions[i].GetId() == regionId {
			return self.iregions[i].(*SRegion)
		}
	}
	return nil
}

func (self *SAzureClient) GetIHostById(id string) (cloudprovider.ICloudHost, error) {
	for i := 0; i < len(self.iregions); i += 1 {
		ihost, err := self.iregions[i].GetIHostById(id)
		if err == nil {
			return ihost, nil
		} else if err != cloudprovider.ErrNotFound {
			return nil, err
		}
	}
	return nil, cloudprovider.ErrNotFound
}

func (self *SAzureClient) GetIVpcById(id string) (cloudprovider.ICloudVpc, error) {
	for i := 0; i < len(self.iregions); i += 1 {
		ihost, err := self.iregions[i].GetIVpcById(id)
		if err == nil {
			return ihost, nil
		} else if err != cloudprovider.ErrNotFound {
			return nil, err
		}
	}
	return nil, cloudprovider.ErrNotFound
}

func (self *SAzureClient) GetIStorageById(id string) (cloudprovider.ICloudStorage, error) {
	for i := 0; i < len(self.iregions); i += 1 {
		ihost, err := self.iregions[i].GetIStorageById(id)
		if err == nil {
			return ihost, nil
		} else if err != cloudprovider.ErrNotFound {
			return nil, err
		}
	}
	return nil, cloudprovider.ErrNotFound
}

func (self *SAzureClient) GetIStoragecacheById(id string) (cloudprovider.ICloudStoragecache, error) {
	for i := 0; i < len(self.iregions); i += 1 {
		ihost, err := self.iregions[i].GetIStoragecacheById(id)
		if err == nil {
			return ihost, nil
		} else if err != cloudprovider.ErrNotFound {
			return nil, err
		}
	}
	return nil, cloudprovider.ErrNotFound
}

type SAccountBalance struct {
	AvailableAmount     float64
	AvailableCashAmount float64
	CreditAmount        float64
	MybankCreditAmount  float64
	Currency            string
}

func (self *SAzureClient) QueryAccountBalance() (*SAccountBalance, error) {
	return nil, cloudprovider.ErrNotImplemented
}
