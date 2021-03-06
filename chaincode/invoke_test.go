package chaincode

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/hyperledger/fabric-protos-go/peer"
)

func set(t *testing.T, a, b string) {
	resp, err := Invoke2(
		ChainOpt{Path: "sacc", Name: "sacc", IsInit: true, Version: "1.0.4"},

		MSPOpt{
			Path: "/mnt/shareSSD/code/YunPhant/wasabi_3/src/wasabi/backEnd/conf/nfs_data/baas98/msp/baas98/peers/peer-0-baas98/msp",
			ID:   "baas98",
		},
		[][]byte{[]byte("set"), []byte(a), []byte(b)},
		map[string][]byte{},
		"channel1",
		[]EndpointWithPath{
			{
				Address: "192.168.9.196:30060",
				GrpcTLSOptWithPath: GrpcTLSOptWithPath{
					CaPath:             "/mnt/shareSSD/code/YunPhant/wasabi_3/src/wasabi/backEnd/conf/nfs_data/baas98/msp/baas98/users/Admin@baas98/tls/ca.crt",
					ServerNameOverride: "peer-0-baas98",
					Timeout:            6 * time.Second,
				},
			},
		},
		[]EndpointWithPath{
			{
				Address: "192.168.9.196:30062",
				GrpcTLSOptWithPath: GrpcTLSOptWithPath{
					CaPath:             "/mnt/shareSSD/code/YunPhant/wasabi_3/src/wasabi/backEnd/conf/nfs_data/baas98/msp/baas98/users/Admin@baas98/tls/ca.crt",
					ServerNameOverride: "orderer-0-baas98",
					Timeout:            6 * time.Second,
				},
			},
		},
	)

	if err != nil {
		t.Error(err)
	}

	t.Log(resp)
}

func TestInvoke(t *testing.T) {
	set(t, "a", "xiaoxiao")
	set(t, "b", "xiaoxiao2")
}

func set2(t *testing.T, b [][]byte) {
	resp, err := Invoke2(
		ChainOpt{Path: "assetTransfer", Name: "basic", IsInit: false, Version: "1.0", Type: peer.ChaincodeSpec_GOLANG},

		MSPOpt{
			Path: "/mnt/shareSSD/code/Fabric/fabric-samples/test-network/organizations/peerOrganizations/org1.example.com/users/Admin@org1.example.com/msp",
			ID:   "Org1MSP",
		},
		b,
		map[string][]byte{},
		"mychannel",
		[]EndpointWithPath{
			{
				Address: "127.0.0.1:7051",
				GrpcTLSOptWithPath: GrpcTLSOptWithPath{
					CaPath:             "/mnt/shareSSD/code/Fabric/fabric-samples/test-network/organizations/peerOrganizations/org1.example.com/users/Admin@org1.example.com/tls/ca.crt",
					ServerNameOverride: "peer0.org1.example.com",
					Timeout:            6 * time.Second,
				},
			},
			{
				Address: "127.0.0.1:9051",
				GrpcTLSOptWithPath: GrpcTLSOptWithPath{
					CaPath:             "/mnt/shareSSD/code/Fabric/fabric-samples/test-network/organizations/peerOrganizations/org2.example.com/users/Admin@org2.example.com/tls/ca.crt",
					ServerNameOverride: "peer0.org2.example.com",
					Timeout:            6 * time.Second,
				},
			},
		},
		[]EndpointWithPath{
			{
				Address: "127.0.0.1:7050",
				GrpcTLSOptWithPath: GrpcTLSOptWithPath{
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

	t.Log("state code:", resp.Response.Status, "payload: -> ", resp.Response.Payload)
}

func TestSet2(t *testing.T) {
	rand.Seed(time.Now().Unix())
	set2(t, [][]byte{[]byte("set"), []byte("time"), []byte(fmt.Sprintf("%f", rand.ExpFloat64()) + " - " + time.Now().String())})
	set2(t, [][]byte{[]byte("set2"), []byte("namae"), []byte("ざこ - " + time.Now().String())})
}

func TestTSS(t *testing.T) {
	a := time.Now().Round(time.Minute).Add(-5 * time.Minute)
	t.Log(a.UTC())
	t.Log(a.Add(5 * 365 * 24 * time.Hour).UTC())
}
