package argocd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The fixtures below are real `aws eks` responses, captured from an EKS cluster
// running the Argo CD capability. The field names are not obvious - the address
// lives at configuration.argoCd.serverUrl, and the listing key is
// capabilityName rather than name - so they are pinned here.

const listCapabilitiesResponse = `{
    "capabilities": [
        {
            "capabilityName": "travisl-argocd",
            "arn": "arn:aws:eks:ap-southeast-2:111122223333:capability/travisl/argocd/travisl-argocd/f4ce5b2d",
            "type": "ARGOCD",
            "status": "ACTIVE",
            "version": "3.3.10-eks-6",
            "createdAt": "2026-03-04T10:58:00.239000+10:00",
            "modifiedAt": "2026-07-03T15:56:41.867000+10:00"
        }
    ]
}`

const describeCapabilityResponse = `{
    "capability": {
        "capabilityName": "travisl-argocd",
        "clusterName": "travisl",
        "type": "ARGOCD",
        "roleArn": "arn:aws:iam::111122223333:role/ArgoCDCapabilityRole",
        "status": "ACTIVE",
        "version": "3.3.10-eks-6",
        "configuration": {
            "argoCd": {
                "namespace": "argocd",
                "networkAccess": {"vpceIds": []},
                "serverUrl": "https://1dddc569b37fd006.eks-capabilities.ap-southeast-2.amazonaws.com"
            }
        },
        "health": {"issues": []}
    }
}`

func TestParseCapabilityList(t *testing.T) {
	capabilities, err := parseCapabilityList([]byte(listCapabilitiesResponse))
	require.NoError(t, err)
	require.Len(t, capabilities, 1)

	assert.Equal(t, "travisl-argocd", capabilities[0].CapabilityName)
	assert.Equal(t, "ARGOCD", capabilities[0].Type)
	assert.Equal(t, "3.3.10-eks-6", capabilities[0].Version)
	assert.Equal(t, "ACTIVE", capabilities[0].Status)
}

func TestParseCapabilityList_IgnoresOtherCapabilityTypes(t *testing.T) {
	capabilities, err := parseCapabilityList([]byte(`{"capabilities":[
		{"capabilityName":"my-ack","type":"ACK","status":"ACTIVE"},
		{"capabilityName":"my-kro","type":"KRO","status":"ACTIVE"}
	]}`))
	require.NoError(t, err)
	assert.Empty(t, capabilities)
}

func TestParseCapabilityList_EmptyOnClusterWithNoCapabilities(t *testing.T) {
	capabilities, err := parseCapabilityList([]byte(`{"capabilities": []}`))
	require.NoError(t, err)
	assert.Empty(t, capabilities)
}

func TestParseCapabilityDescription(t *testing.T) {
	summary := capabilitySummary{CapabilityName: "travisl-argocd", Version: "3.3.10-eks-6", Status: "ACTIVE"}

	instance, found, err := parseCapabilityDescription([]byte(describeCapabilityResponse), summary)
	require.NoError(t, err)
	require.True(t, found)

	assert.Equal(t, KindEKSManaged, instance.Kind)
	assert.Equal(t, "travisl-argocd", instance.Name)
	assert.Equal(t, "argocd", instance.Namespace)
	assert.Equal(t, "3.3.10-eks-6", instance.Version)
	assert.Equal(t, "ACTIVE", instance.Status)
	assert.Equal(t, "grpc://1dddc569b37fd006.eks-capabilities.ap-southeast-2.amazonaws.com", instance.ServerGRPCURL)
	assert.Equal(t, "https://1dddc569b37fd006.eks-capabilities.ap-southeast-2.amazonaws.com", instance.WebUIURL)

	assert.True(t, instance.GRPCWeb)
	assert.False(t, instance.SelfSignedTLS)
	assert.False(t, instance.Plaintext)
}

// A capability that is still provisioning has no address yet, which is a
// "nothing found" rather than a failure.
func TestParseCapabilityDescription_NoAddressYet(t *testing.T) {
	_, found, err := parseCapabilityDescription([]byte(`{"capability":{
		"capabilityName":"new-argocd","type":"ARGOCD","status":"CREATING",
		"configuration":{"argoCd":{"namespace":"argocd"}}
	}}`), capabilitySummary{CapabilityName: "new-argocd"})

	require.NoError(t, err)
	assert.False(t, found)
}
