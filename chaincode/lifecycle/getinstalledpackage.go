package lifecycle

import (
	"github.com/Asutorufa/fabricsdk/chaincode"
	"github.com/hyperledger/fabric-protos-go/peer"
	"github.com/hyperledger/fabric-protos-go/peer/lifecycle"
)

// GetInstalledPackage chainOpt just need PackageID
func GetInstalledPackage(
	chainOpt chaincode.ChainOpt,
	mspOpt chaincode.MSPOpt,
	peer chaincode.Endpoint,
) (*peer.ProposalResponse, error) {
	signer, err := chaincode.GetSigner(mspOpt.Path, mspOpt.Id)
	args := &lifecycle.GetInstalledChaincodePackageArgs{
		PackageId: chainOpt.PackageID,
	}

	function := "GetInstalledChaincodePackage"

	proposal, err := createProposal(args, signer, function, "")
	if err != nil {

	}

	return query(signer, proposal, peer)
}

// GetInstalledPackage2 get installed package
func GetInstalledPackage2(
	chainOpt chaincode.ChainOpt,
	mspOpt chaincode.MSPOpt,
	peer chaincode.Endpoint2,
) (*peer.ProposalResponse, error) {
	ep, err := chaincode.Endpoint2ToEndpoint(peer)
	if err != nil {
		return nil, err
	}
	return GetInstalledPackage(chainOpt, mspOpt, ep)
}