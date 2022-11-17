package machinescommon

var CommunicationStyleToDescriptionMap = map[string]string{
	"TentaclePassive":           "Listening Tentacle",
	"TentacleActive":            "Polling Tentacle",
	"Ssh":                       "SSH Connection",
	"OfflineDrop":               "Offline Package Drop",
	"AzureWebApp":               "Azure Web App",
	"AzureCloudService":         "Azure Cloud Service",
	"AzureServiceFabricCluster": "Service Fabric Cluster",
	"Kubernetes":                "Kubernetes Cluster",
	"None":                      "Cloud Region",
	"StepPackage":               "Step Package",
}

var CommunicationStyleToDeploymentTargetTypeMap = map[string]string{
	"TentaclePassive":           "TentaclePassive",
	"TentacleActive":            "TentacleActive",
	"Ssh":                       "Ssh",
	"OfflineDrop":               "OfflineDrop",
	"AzureWebApp":               "AzureWebApp",
	"AzureCloudService":         "AzureCloudService",
	"AzureServiceFabricCluster": "AzureServiceFabricCluster",
	"Kubernetes":                "Kubernetes",
	"None":                      "CloudRegion",
}
