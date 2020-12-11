package chaincode

import (
	"context"
	"crypto/tls"
	"errors"
	"fabricSDK/chaincode/client/clientcommon"
	"fabricSDK/chaincode/client/orderclient"
	"fabricSDK/chaincode/client/peerclient"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/hyperledger/fabric-chaincode-go/shim"
	"github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric-protos-go/orderer"
	"github.com/hyperledger/fabric-protos-go/peer"
	"github.com/hyperledger/fabric/common/util"
	"github.com/hyperledger/fabric/msp"
	"github.com/hyperledger/fabric/protoutil"
)

// Invoke2 arguments detail function Invoke
func Invoke2(
	chaincode ChainOpt,
	mspOpt MSPOpt,
	args [][]byte, // [][]byte{[]byte("function"),[]byte("a"),[]byte("b")}, first array is function name
	privateData map[string][]byte,
	channelID string,
	peers []Endpoint2,
	orderer Endpoint2,
) (*peer.ProposalResponse, error) {
	var eps []Endpoint
	for index := range peers {
		ep, err := Endpoint2ToEndpoint(peers[index])
		if err != nil {
			return nil, err
		}
		eps = append(eps, ep)
	}
	ordereR, err := Endpoint2ToEndpoint(orderer)
	if err != nil {
		return nil, err
	}
	return Invoke(
		chaincode,
		mspOpt,
		args,
		privateData,
		channelID,
		//"",
		eps,
		ordereR,
	)
}

// Invoke
// chaincode just need Path,Name,IsInit, Version, Type
// peerGrpcOpt Timeout is necessary
// ordererGrpcOpt Timeout is necessary
// mspOpt necessary
// args [][]byte{[]byte("function"),[]byte("a"),[]byte("b")}, first array is function name
// privateData not necessary, like: map[string][]byte{"cert":[]byte("transient")}, more: https://hyperledger-fabric.readthedocs.io/zh_CN/latest/private_data_tutorial.html
// channelID necessary channel name
// peerAddress necessary peer address array
// ordererAddress necessary orderer address
func Invoke(
	chaincode ChainOpt,
	mspOpt MSPOpt,
	args [][]byte,
	privateData map[string][]byte,
	channelID string,
	//txID string,
	peers []Endpoint,
	orderer Endpoint,
) (*peer.ProposalResponse, error) {

	invocation := getChaincodeInvocationSpec(
		chaincode.Path,
		chaincode.Name,
		chaincode.IsInit,
		chaincode.Version,
		peer.ChaincodeSpec_GOLANG,
		args,
	)
	signer, err := GetSigner(mspOpt.Path, mspOpt.Id)
	if err != nil {
		return nil, err
	}
	creator, err := signer.Serialize()
	if err != nil {
		return nil, err
	}

	//tMap := map[string][]byte{
	//	"cert": []byte("transient"),
	//}

	prop, txid, err := protoutil.CreateChaincodeProposalWithTxIDAndTransient(
		common.HeaderType_ENDORSER_TRANSACTION,
		channelID,
		invocation,
		creator,
		"",
		privateData, // transientMap <- 因为链码提案被存储在区块链上，
		// 不要把私有数据包含在链码提案中也是非常重要的。
		//在链码提案中有一个特殊的字段 transient，
		//可以用它把私有数据来从客户端（或者链码将用来生成私有数据的数据）传递给节点上的链码调用。
		//链码可以通过调用 GetTransient() API 来获取 transient 字段。
		//这个 transient 字段会从通道交易中被排除
	)
	if err != nil {
		return nil, err
	}

	signedProp, err := protoutil.GetSignedProposal(prop, signer)
	if err != nil {
		return nil, err
	}

	var peerClients []*peerclient.PeerClient
	var endorserClients []peer.EndorserClient
	var deliverClients []peer.DeliverClient
	var certificate tls.Certificate
	for index := range peers {
		peerClient, err := peerclient.NewPeerClient(
			peers[index].Address,
			peers[index].GrpcTLSOpt.ServerNameOverride,
			clientcommon.WithClientCert2(peers[index].GrpcTLSOpt.ClientKey, peers[index].GrpcTLSOpt.ClientCrt),
			clientcommon.WithTLS2(peers[index].GrpcTLSOpt.Ca),
			clientcommon.WithTimeout(peers[index].GrpcTLSOpt.Timeout),
		)
		if err != nil {
			return nil, err
		}
		peerClients = append(peerClients, peerClient)
		certificate = peerClient.Certificate()
		endorserClient, err := peerClient.Endorser()
		if err != nil {
			return nil, err
		}
		endorserClients = append(endorserClients, endorserClient)

		deliverClient, err := peerClient.PeerDeliver()
		if err != nil {
			return nil, err
		}
		deliverClients = append(deliverClients, deliverClient)
	}
	defer func() {
		for index := range peerClients {
			_ = peerClients[index].Close()
		}
	}()

	responses, err := processProposals(endorserClients, signedProp)
	if err != nil {
		return nil, err
	}
	fmt.Printf("txid: %s\n", txid)
	resp := responses[0]
	if resp == nil {
		return resp, nil
	}

	if resp.Response.Status >= shim.ERRORTHRESHOLD {
		return resp, nil
	}

	env, err := protoutil.CreateSignedTx(prop, signer, responses...)
	if err != nil {
		return resp, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
	defer cancel()

	var eps []string
	for index := range peers {
		eps = append(eps, peers[index].Address)
	}
	dg := NewDeliverGroup(
		deliverClients,
		eps,
		signer,
		certificate,
		channelID,
		txid,
	)

	err = dg.Connect(ctx)
	if err != nil {
		return nil, err
	}

	order, err := orderclient.NewOrdererClient(
		orderer.Address,
		orderer.GrpcTLSOpt.ServerNameOverride,
		clientcommon.WithClientCert2(orderer.GrpcTLSOpt.ClientKey, orderer.GrpcTLSOpt.ClientCrt),
		clientcommon.WithTLS2(orderer.GrpcTLSOpt.Ca),
		clientcommon.WithTimeout(orderer.GrpcTLSOpt.Timeout),
	)
	if err != nil {
		return nil, err
	}
	defer order.Close()
	ordererClient, err := order.Broadcast()
	if err != nil {
		return nil, err
	}
	err = ordererClient.Send(env)
	if err != nil {
		return resp, err
	}

	if dg != nil && ctx != nil {
		err = dg.Wait(ctx)
		if err != nil {
			return nil, fmt.Errorf("dg.Wait() -> %v", err)
		}
	}
	return resp, nil
}

// DeliverGroup holds all of the information needed to connect
// to a set of peers to wait for the interested txid to be
// committed to the ledgers of all peers. This functionality
// is currently implemented via the peer's DeliverFiltered service.
// An error from any of the peers/deliver clients will result in
// the invoke command returning an error. Only the first error that
// occurs will be set
type DeliverGroup struct {
	Clients     []*DeliverClient
	Certificate tls.Certificate
	ChannelID   string
	TxID        string
	Signer      msp.SigningIdentity
	mutex       sync.Mutex
	Error       error
	wg          sync.WaitGroup
}

// DeliverClient holds the client/connection related to a specific
// peer. The address is included for logging purposes
type DeliverClient struct {
	Client     peer.DeliverClient
	Connection peer.Deliver_DeliverClient
	Address    string
}

func NewDeliverGroup(
	deliverClients []peer.DeliverClient,
	peerAddresses []string,
	signer msp.SigningIdentity,
	certificate tls.Certificate,
	channelID string,
	txid string,
) *DeliverGroup {
	clients := make([]*DeliverClient, len(deliverClients))
	for i, client := range deliverClients {
		address := peerAddresses[i]
		//if address == "" {
		//	address = viper.GetString("peer.address")
		//}
		dc := &DeliverClient{
			Client:  client,
			Address: address,
		}
		clients[i] = dc
	}

	dg := &DeliverGroup{
		Clients:     clients,
		Certificate: certificate,
		ChannelID:   channelID,
		TxID:        txid,
		Signer:      signer,
	}

	return dg
}

// Connect waits for all deliver clients in the group to connect to
// the peer's deliver service, receive an error, or for the context
// to timeout. An error will be returned whenever even a single
// deliver client fails to connect to its peer
func (dg *DeliverGroup) Connect(ctx context.Context) error {
	dg.wg.Add(len(dg.Clients))
	for _, client := range dg.Clients {
		go dg.ClientConnect(ctx, client)
	}
	readyCh := make(chan struct{})
	go dg.WaitForWG(readyCh)

	select {
	case <-readyCh:
		if dg.Error != nil {
			err := fmt.Errorf("%v failed to connect to deliver on all peers", dg.Error)
			return err
		}
	case <-ctx.Done():
		err := errors.New("timed out waiting for connection to deliver on all peers")
		return err
	}

	return nil
}

// ClientConnect sends a deliver seek info envelope using the
// provided deliver client, setting the deliverGroup's Error
// field upon any error
func (dg *DeliverGroup) ClientConnect(ctx context.Context, dc *DeliverClient) {
	defer dg.wg.Done()
	df, err := dc.Client.DeliverFiltered(ctx)
	if err != nil {
		//err = errors.WithMessagef(err, "error connecting to deliver filtered at %s", dc.Address)
		dg.setError(err)
		return
	}
	defer df.CloseSend()
	dc.Connection = df

	envelope := createDeliverEnvelope(dg.ChannelID, dg.Certificate, dg.Signer)
	err = df.Send(envelope)
	if err != nil {
		//err = errors.WithMessagef(err, "error sending deliver seek info envelope to %s", dc.Address)
		dg.setError(err)
		return
	}
}

// Wait waits for all deliver client connections in the group to
// either receive a block with the txid, an error, or for the
// context to timeout
func (dg *DeliverGroup) Wait(ctx context.Context) error {
	if len(dg.Clients) == 0 {
		return nil
	}

	dg.wg.Add(len(dg.Clients))
	for _, client := range dg.Clients {
		go dg.ClientWait(client)
	}
	readyCh := make(chan struct{})
	go dg.WaitForWG(readyCh)

	select {
	case <-readyCh:
		if dg.Error != nil {
			return dg.Error
		}
	case <-ctx.Done():
		err := errors.New("timed out waiting for txid on all peers")
		return err
	}

	return nil
}

// ClientWait waits for the specified deliver client to receive
// a block event with the requested txid
func (dg *DeliverGroup) ClientWait(dc *DeliverClient) {
	defer dg.wg.Done()
	for {
		resp, err := dc.Connection.Recv()
		if err != nil {
			//err = errors.WithMessagef(err, "error receiving from deliver filtered at %s", dc.Address)
			dg.setError(err)
			return
		}
		switch r := resp.Type.(type) {
		case *peer.DeliverResponse_FilteredBlock:
			filteredTransactions := r.FilteredBlock.FilteredTransactions
			for _, tx := range filteredTransactions {
				if tx.Txid == dg.TxID {
					//logger.Infof("txid [%s] committed with status (%s) at %s", dg.TxID, tx.TxValidationCode, dc.Address)
					if tx.TxValidationCode != peer.TxValidationCode_VALID {
						//err = errors.Errorf("transaction invalidated with status (%s)", tx.TxValidationCode)
						dg.setError(err)
					}
					return
				}
			}
		case *peer.DeliverResponse_Status:
			//err = errors.Errorf("deliver completed with status (%s) before txid received", r.Status)
			dg.setError(err)
			return
		default:
			//err = errors.Errorf("received unexpected response type (%T) from %s", r, dc.Address)
			dg.setError(err)
			return
		}
	}
}

// WaitForWG waits for the deliverGroup's wait group and closes
// the channel when ready
func (dg *DeliverGroup) WaitForWG(readyCh chan struct{}) {
	dg.wg.Wait()
	close(readyCh)
}

// setError serializes an error for the deliverGroup
func (dg *DeliverGroup) setError(err error) {
	dg.mutex.Lock()
	dg.Error = err
	dg.mutex.Unlock()
}

func createDeliverEnvelope(
	channelID string,
	certificate tls.Certificate,
	signer msp.SigningIdentity,
) *common.Envelope {
	var tlsCertHash []byte
	// check for client certificate and create hash if present
	if len(certificate.Certificate) > 0 {
		tlsCertHash = util.ComputeSHA256(certificate.Certificate[0])
	}

	start := &orderer.SeekPosition{
		Type: &orderer.SeekPosition_Newest{
			Newest: &orderer.SeekNewest{},
		},
	}

	stop := &orderer.SeekPosition{
		Type: &orderer.SeekPosition_Specified{
			Specified: &orderer.SeekSpecified{
				Number: math.MaxUint64,
			},
		},
	}

	seekInfo := &orderer.SeekInfo{
		Start:    start,
		Stop:     stop,
		Behavior: orderer.SeekInfo_BLOCK_UNTIL_READY,
	}

	env, err := protoutil.CreateSignedEnvelopeWithTLSBinding(
		common.HeaderType_DELIVER_SEEK_INFO,
		channelID,
		signer,
		seekInfo,
		int32(0),
		uint64(0),
		tlsCertHash,
	)
	if err != nil {
		//logger.Errorf("Error signing envelope: %s", err)
		return nil
	}

	return env
}
