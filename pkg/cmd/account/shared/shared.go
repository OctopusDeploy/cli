package shared

var AzureEnvMap = map[string]string{
	"Global Cloud (Default)": "AzureCloud",
	"China Cloud":            "AzureChinaCloud",
	"German Cloud":           "AzureGermanCloud",
	"US Government":          "AzureUSGovernment",
}
var AzureADEndpointBaseUri = map[string]string{
	"AzureCloud":        "https://login.microsoftonline.com/",
	"AzureChinaCloud":   "https://login.chinacloudapi.cn/",
	"AzureGermanCloud":  "https://login.microsoftonline.de/",
	"AzureUSGovernment": "https://login.microsoftonline.us/",
}
var AzureResourceManagementBaseUri = map[string]string{
	"AzureCloud":        "https://management.azure.com/",
	"AzureChinaCloud":   "https://management.chinacloudapi.cn/",
	"AzureGermanCloud":  "https://management.microsoftazure.de/",
	"AzureUSGovernment": "https://management.usgovcloudapi.net/",
}
