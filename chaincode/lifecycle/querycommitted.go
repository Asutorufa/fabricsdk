package lifecycle

import (
	"github.com/Asutorufa/fabricsdk/chaincode"
	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric-protos-go/peer"
	"github.com/hyperledger/fabric-protos-go/peer/lifecycle"
)

// QueryCommitted query committed
// chainOpt optional: all
func QueryCommitted(
	chainOpt chaincode.ChainOpt,
	mspOpt chaincode.MSPOpt,
	channelID string,
	peer []chaincode.Endpoint,
) (*peer.ProposalResponse, error) {
	var function string
	var args proto.Message
	if chainOpt.Name != "" {
		function = "QueryChaincodeDefinition"
		args = &lifecycle.QueryChaincodeDefinitionArgs{
			Name: chainOpt.Name,
		}
	} else {
		function = "QueryChaincodeDefinitions"
		args = &lifecycle.QueryChaincodeDefinitionsArgs{}
	}

	signer, err := chaincode.GetSigner(mspOpt.Path, mspOpt.ID)
	if err != nil {
		return nil, err
	}

	proposal, _, err := createProposal(args, signer, function, channelID)
	if err != nil {
		return nil, err
	}

	return query(signer, proposal, peer)
}

func QueryCommitted2(
	chainOpt chaincode.ChainOpt,
	mspOpt chaincode.MSPOpt,
	channelID string,
	peer []chaincode.EndpointWithPath,
) (*peer.ProposalResponse, error) {
	ep, err := chaincode.ParseEndpointsWithPath(peer)
	if err != nil {
		return nil, err
	}
	return QueryCommitted(chainOpt, mspOpt, channelID, ep)
}
