package lifecycle

import (
	"testing"
	"time"

	"github.com/Asutorufa/fabricsdk/chaincode"
	"github.com/gogo/protobuf/proto"
	"github.com/hyperledger/fabric-protos-go/peer/lifecycle"
)

func TestApproveForMyOrg(t *testing.T) {
	resp, err := ApproveForMyOrg2(
		chaincode.ChainOpt{
			Name:     "basic",
			Version:  "1.0",
			Sequence: 1,
		},
		chaincode.MSPOpt{
			Path: "/mnt/shareSSD/code/Fabric/fabric-samples/test-network/organizations/peerOrganizations/org1.example.com/users/Admin@org1.example.com/msp",
			Id:   "Org1MSP",
		},
		"mychannel",
		[]chaincode.Endpoint2{
			{
				Address: "127.0.0.1:7051",
				GrpcTLSOpt2: chaincode.GrpcTLSOpt2{
					ClientCrtPath:      "/mnt/shareSSD/code/Fabric/fabric-samples/test-network/organizations/peerOrganizations/org1.example.com/users/Admin@org1.example.com/tls/client.crt",
					ClientKeyPath:      "/mnt/shareSSD/code/Fabric/fabric-samples/test-network/organizations/peerOrganizations/org1.example.com/users/Admin@org1.example.com/tls/client.key",
					CaPath:             "/mnt/shareSSD/code/Fabric/fabric-samples/test-network/organizations/peerOrganizations/org1.example.com/users/Admin@org1.example.com/tls/ca.crt",
					ServerNameOverride: "peer0.org1.example.com",
					Timeout:            6 * time.Second,
				},
			},
		},
		[]chaincode.Endpoint2{
			{
				Address: "127.0.0.1:7050",
				GrpcTLSOpt2: chaincode.GrpcTLSOpt2{
					ClientCrtPath:      "/mnt/shareSSD/code/Fabric/fabric-samples/test-network/organizations/ordererOrganizations/example.com/users/Admin@example.com/tls/client.crt",
					ClientKeyPath:      "/mnt/shareSSD/code/Fabric/fabric-samples/test-network/organizations/ordererOrganizations/example.com/users/Admin@example.com/tls/client.key",
					CaPath:             "/mnt/shareSSD/code/Fabric/fabric-samples/test-network/organizations/ordererOrganizations/example.com/users/Admin@example.com/tls/ca.crt",
					ServerNameOverride: "orderer.example.com",
					Timeout:            6 * time.Second,
				},
			},
		},
	)

	if err != nil {
		t.Error(err)
	}

	t.Log(string(resp.Payload))

	x := &lifecycle.ApproveChaincodeDefinitionForMyOrgResult{} // nothing
	err = proto.Unmarshal(resp.Payload, x)
	if err != nil {
		t.Error(err)
	}
}