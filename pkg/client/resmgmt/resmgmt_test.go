/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package resmgmt

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	txnmocks "github.com/hyperledger/fabric-sdk-go/pkg/client/common/mocks"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/context"
	contextImpl "github.com/hyperledger/fabric-sdk-go/pkg/context"
	"github.com/hyperledger/fabric-sdk-go/pkg/context/api/core"
	"github.com/hyperledger/fabric-sdk-go/pkg/context/api/fab"
	"github.com/hyperledger/fabric-sdk-go/pkg/core/config"
	fcmocks "github.com/hyperledger/fabric-sdk-go/pkg/fab/mocks"
	"github.com/hyperledger/fabric-sdk-go/pkg/fab/peer"
	"github.com/hyperledger/fabric-sdk-go/pkg/fab/resource/api"
	"github.com/hyperledger/fabric-sdk-go/pkg/fabsdk/provider/fabpvdr"
	"github.com/hyperledger/fabric-sdk-go/third_party/github.com/hyperledger/fabric/common/cauthdsl"
	"github.com/hyperledger/fabric-sdk-go/third_party/github.com/hyperledger/fabric/protos/common"
	pb "github.com/hyperledger/fabric-sdk-go/third_party/github.com/hyperledger/fabric/protos/peer"
)

const channelConfig = "../../../test/fixtures/fabric/v1.0/channel/mychannel.tx"
const networkCfg = "../../../test/fixtures/config/config_test.yaml"

func TestJoinChannelFail(t *testing.T) {

	grpcServer := grpc.NewServer()
	defer grpcServer.Stop()

	endorserServer, addr := startEndorserServer(t, grpcServer)
	ctx := setupTestContext("test", "Org1MSP")

	// Create mock orderer with simple mock block
	orderer := fcmocks.NewMockOrderer("", nil)
	defer orderer.Close()
	orderer.EnqueueForSendDeliver(fcmocks.NewSimpleMockBlock())
	orderer.EnqueueForSendDeliver(common.Status_SUCCESS)

	setupCustomOrderer(ctx, orderer)

	rc := setupResMgmtClient(ctx, nil, t)

	// Setup target peers
	peer1, _ := peer.New(fcmocks.NewMockConfig(), peer.WithURL("grpc://"+addr))

	// Test fail with send proposal error
	endorserServer.ProposalError = errors.New("Test Error")
	err := rc.JoinChannel("mychannel", WithTargets(peer1))

	if err == nil {
		t.Fatal("Should have failed to get genesis block")
	}

}

func TestJoinChannelSuccess(t *testing.T) {
	grpcServer := grpc.NewServer()
	defer grpcServer.Stop()

	_, addr := startEndorserServer(t, grpcServer)
	ctx := setupTestContext("test", "Org1MSP")

	// Create mock orderer with simple mock block
	orderer := fcmocks.NewMockOrderer("", nil)
	defer orderer.Close()
	orderer.EnqueueForSendDeliver(fcmocks.NewSimpleMockBlock())
	orderer.EnqueueForSendDeliver(common.Status_SUCCESS)

	setupCustomOrderer(ctx, orderer)

	rc := setupResMgmtClient(ctx, nil, t)

	// Setup target peers
	var peers []fab.Peer
	peer1, _ := peer.New(fcmocks.NewMockConfig(), peer.WithURL("grpc://"+addr))
	peers = append(peers, peer1)

	// Test valid join channel request (success)
	err := rc.JoinChannel("mychannel", WithTargets(peer1))
	if err != nil {
		t.Fatal(err)
	}

}

func TestWithFilterOption(t *testing.T) {
	ctx := setupTestContext("test", "Org1MSP")
	rc := setupResMgmtClient(ctx, nil, t, getDefaultTargetFilterOption())
	if rc == nil {
		t.Fatal("Expected Resource Management Client to be set")
	}
}

func TestJoinChannelWithFilter(t *testing.T) {
	grpcServer := grpc.NewServer()
	defer grpcServer.Stop()

	_, addr := startEndorserServer(t, grpcServer)
	ctx := setupTestContext("test", "Org1MSP")

	// Create mock orderer with simple mock block
	orderer := fcmocks.NewMockOrderer("", nil)
	defer orderer.Close()
	orderer.EnqueueForSendDeliver(fcmocks.NewSimpleMockBlock())
	orderer.EnqueueForSendDeliver(common.Status_SUCCESS)
	setupCustomOrderer(ctx, orderer)

	//the target filter ( client option) will be set
	rc := setupResMgmtClient(ctx, nil, t)

	// Setup target peers
	var peers []fab.Peer
	peer1, _ := peer.New(fcmocks.NewMockConfig(), peer.WithURL("grpc://"+addr))
	peers = append(peers, peer1)

	// Test valid join channel request (success)
	err := rc.JoinChannel("mychannel", WithTargets(peer1))
	if err != nil {
		t.Fatal(err)
	}
}

func TestNoSigningUserFailure(t *testing.T) {
	user := fcmocks.NewMockUserWithMSPID("test", "")

	// Setup client without user context
	fabCtx := fcmocks.NewMockContext(user)
	config := getNetworkConfig(t)
	fabCtx.SetConfig(config)

	clientCtx := createClientContext(contextImpl.Client{
		Providers: fabCtx,
		Identity:  fabCtx,
	})

	_, err := New(clientCtx)
	if err == nil {
		t.Fatal("Should have failed due to missing msp")
	}

}

func TestJoinChannelRequiredParameters(t *testing.T) {

	rc := setupDefaultResMgmtClient(t)

	// Test empty channel name
	err := rc.JoinChannel("")
	if err == nil {
		t.Fatalf("Should have failed for empty channel name")
	}

	// Setup test client with different msp (default targets cannot be calculated)
	ctx := setupTestContext("test", "otherMSP")
	config := getNetworkConfig(t)
	ctx.SetConfig(config)

	// Create new resource management client ("otherMSP")
	rc = setupResMgmtClient(ctx, nil, t)

	// Test missing default targets
	err = rc.JoinChannel("mychannel")
	if err == nil || !strings.Contains(err.Error(), "No targets available") {
		t.Fatalf("InstallCC should have failed with no default targets error")
	}

}

func TestJoinChannelWithOptsRequiredParameters(t *testing.T) {

	grpcServer := grpc.NewServer()
	defer grpcServer.Stop()

	_, addr := startEndorserServer(t, grpcServer)

	ctx := setupTestContextWithDiscoveryError("test", "Org1MSP", nil)
	network := getNetworkConfig(t)
	ctx.SetConfig(network)

	// Create mock orderer with simple mock block
	orderer := fcmocks.NewMockOrderer("", nil)
	defer orderer.Close()
	orderer.EnqueueForSendDeliver(fcmocks.NewSimpleMockBlock())
	orderer.EnqueueForSendDeliver(common.Status_SUCCESS)
	setupCustomOrderer(ctx, orderer)

	rc := setupResMgmtClient(ctx, nil, t, getDefaultTargetFilterOption())

	// Test empty channel name for request with no opts
	err := rc.JoinChannel("")
	if err == nil {
		t.Fatalf("Should have failed for empty channel name")
	}

	var peers []fab.Peer
	peer1, _ := peer.New(fcmocks.NewMockConfig(), peer.WithURL("grpc://"+addr), peer.WithMSPID("Org1MSP"))
	peers = append(peers, peer1)

	// Test both targets and filter provided (error condition)
	err = rc.JoinChannel("mychannel", WithTargets(peers...), WithTargetFilter(&MSPFilter{mspID: "MspID"}))
	if err == nil || !strings.Contains(err.Error(), "If targets are provided, filter cannot be provided") {
		t.Fatalf("Should have failed if both target and filter provided")
	}

	// Test targets only
	err = rc.JoinChannel("mychannel", WithTargets(peers...))
	if err != nil {
		t.Fatalf(err.Error())
	}

	// Test filter only (filter has no match)
	err = rc.JoinChannel("mychannel", WithTargetFilter(&MSPFilter{mspID: "MspID"}))
	if err == nil || !strings.Contains(err.Error(), "No targets available") {
		t.Fatalf("InstallCC should have failed with no targets error")
	}

	//Some cleanup before further test
	orderer = fcmocks.NewMockOrderer("", nil)
	defer orderer.Close()
	orderer.EnqueueForSendDeliver(fcmocks.NewSimpleMockBlock())
	orderer.EnqueueForSendDeliver(common.Status_SUCCESS)
	setupCustomOrderer(ctx, orderer)

	rc = setupResMgmtClient(ctx, nil, t, getDefaultTargetFilterOption())
	disProvider, _ := fcmocks.NewMockDiscoveryProvider(nil, peers)
	rc.discovery, _ = disProvider.CreateDiscoveryService("mychannel")

	// Test filter only (filter has a match)
	err = rc.JoinChannel("mychannel", WithTargetFilter(&MSPFilter{mspID: "Org1MSP"}))
	if err != nil {
		t.Fatalf(err.Error())
	}

}

func TestJoinChannelDiscoveryError(t *testing.T) {

	// Setup test client and config
	ctx := setupTestContextWithDiscoveryError("test", "Org1MSP", errors.New("Test Error"))
	config := getNetworkConfig(t)
	ctx.SetConfig(config)

	// Create resource management client with discovery service that will generate an error
	rc := setupResMgmtClient(ctx, nil, t)

	err := rc.JoinChannel("mychannel")
	if err == nil {
		t.Fatalf("Should have failed to join channel with discovery error")
	}

	// If targets are not provided discovery service is used
	err = rc.JoinChannel("mychannel")
	if err == nil {
		t.Fatalf("Should have failed to join channel with discovery error")
	}

}

func TestOrdererConfigFail(t *testing.T) {

	ctx := setupTestContext("test", "Org1MSP")

	// No channel orderer, no global orderer
	noOrdererConfig, err := config.FromFile("./testdata/noorderer_test.yaml")()
	assert.Nil(t, err)

	ctx.SetConfig(noOrdererConfig)
	rc := setupResMgmtClient(ctx, nil, t)

	opts := Opts{}
	orderer, err := rc.ordererConfig(&opts, "mychannel")
	assert.Nil(t, orderer)
	assert.NotNil(t, err, "should fail since no orderer has been configured")
}

/*
func TestOrdererConfigFromOpts(t *testing.T) {
	ctx := setupTestContext("test", "Org1MSP")

	// No channel orderer, no global orderer
	noOrdererConfig, err := config.FromFile("./testdata/ccproposal_test.yaml")()
	assert.Nil(t, err)

	ctx.SetConfig(noOrdererConfig)
	rc := setupResMgmtClient(ctx, nil, t)

	opts := Opts{}
	orderer, err := rc.ordererConfig(&opts, "mychannel")
	assert.Nil(t, orderer)
	assert.NotNil(t, err, "should fail since no orderer has been configured")
}*/

func TestJoinChannelNoOrdererConfig(t *testing.T) {

	ctx := setupTestContext("test", "Org1MSP")

	// No channel orderer, no global orderer
	noOrdererConfig, err := config.FromFile("./testdata/noorderer_test.yaml")()
	if err != nil {
		t.Fatal(err)
	}
	ctx.SetConfig(noOrdererConfig)
	rc := setupResMgmtClient(ctx, nil, t)

	err = rc.JoinChannel("mychannel")
	assert.NotNil(t, err, "Should have failed to join channel since no orderer has been configured")

	// Misconfigured channel orderer
	invalidChOrdererConfig, err := config.FromFile("./testdata/invalidchorderer_test.yaml")()
	if err != nil {
		t.Fatal(err)
	}
	ctx.SetConfig(invalidChOrdererConfig)

	rc = setupResMgmtClient(ctx, nil, t)

	err = rc.JoinChannel("mychannel")
	if err == nil {
		t.Fatalf("Should have failed to join channel since channel orderer has been misconfigured")
	}

	// Misconfigured global orderer (cert cannot be loaded)
	invalidOrdererConfig, err := config.FromFile("./testdata/invalidorderer_test.yaml")()
	if err != nil {
		t.Fatal(err)
	}
	ctx.SetConfig(invalidOrdererConfig)
	customFabProvider := fabpvdr.New(ctx.Config())
	customFabProvider.Initialize(ctx)
	ctx.SetCustomInfraProvider(customFabProvider)

	rc = setupResMgmtClient(ctx, nil, t)

	err = rc.JoinChannel("mychannel")
	if err == nil {
		t.Fatalf("Should have failed to join channel since global orderer certs are not configured properly")
	}
}

func TestIsChaincodeInstalled(t *testing.T) {

	rc := setupDefaultResMgmtClient(t)

	//prepare sample response
	response := new(pb.ChaincodeQueryResponse)
	chaincodes := make([]*pb.ChaincodeInfo, 1)
	chaincodes[0] = &pb.ChaincodeInfo{Name: "test-name", Path: "test-path", Version: "test-version"}
	response.Chaincodes = chaincodes
	responseBytes, err := proto.Marshal(response)
	if err != nil {
		t.Fatal("failed to marshal sample response")
	}

	peer1 := &fcmocks.MockPeer{MockName: "Peer1", MockURL: "grpc://peer1.com", MockRoles: []string{}, MockCert: nil, MockMSP: "Org1MSP", Status: http.StatusOK, Payload: responseBytes}

	// Chaincode found request
	req := InstallCCRequest{Name: "test-name", Path: "test-path", Version: "test-version"}

	// Test chaincode installed (valid peer)
	installed, err := rc.isChaincodeInstalled(req, peer1)
	if err != nil {
		t.Fatal(err)
	}
	if !installed {
		t.Fatalf("CC should have been installed: %v", req)
	}

	// Chaincode not found request
	req = InstallCCRequest{Name: "ID", Version: "v0", Path: "path"}

	// Test chaincode installed
	installed, err = rc.isChaincodeInstalled(req, peer1)
	if err != nil {
		t.Fatal(err)
	}
	if installed {
		t.Fatalf("CC should NOT have been installed: %s", req)
	}

	// Test error retrieving installed cc info (peer is nil)
	_, err = rc.isChaincodeInstalled(req, nil)
	if err == nil {
		t.Fatalf("Should have failed with error in get installed chaincodes")
	}

}

func TestQueryInstalledChaincodes(t *testing.T) {

	rc := setupDefaultResMgmtClient(t)

	// Test error
	_, err := rc.QueryInstalledChaincodes(nil)
	if err == nil {
		t.Fatalf("QueryInstalledChaincodes: peer cannot be nil")
	}

	peer := &fcmocks.MockPeer{MockName: "Peer1", MockURL: "http://peer1.com", MockRoles: []string{}, MockCert: nil, MockMSP: "Org1MSP", Status: http.StatusOK}

	// Test success (valid peer)
	_, err = rc.QueryInstalledChaincodes(peer)
	if err != nil {
		t.Fatal(err)
	}

}

func TestQueryChannels(t *testing.T) {

	rc := setupDefaultResMgmtClient(t)

	//prepare sample response
	response := new(pb.ChannelQueryResponse)
	channels := make([]*pb.ChannelInfo, 1)
	channels[0] = &pb.ChannelInfo{ChannelId: "test"}
	response.Channels = channels

	responseBytes, err := proto.Marshal(response)
	if err != nil {
		t.Fatal("failed to marshal sample response")
	}

	// Test error
	_, err = rc.QueryChannels(nil)
	if err == nil {
		t.Fatalf("QueryChannels: peer cannot be nil")
	}

	peer := &fcmocks.MockPeer{MockName: "Peer1", MockURL: "http://peer1.com", MockRoles: []string{}, MockCert: nil, MockMSP: "Org1MSP", Status: http.StatusOK, Payload: responseBytes}

	// Test success (valid peer)
	found := false
	response, err = rc.QueryChannels(peer)
	if err != nil {
		t.Fatalf("failed to query channel for peer: %s", err)
	}
	for _, responseChannel := range response.Channels {
		if responseChannel.ChannelId == "test" {
			found = true
		}
	}

	if !found {
		t.Fatal("Peer has not joined 'test' channel")
	}

}

func TestInstallCCWithOpts(t *testing.T) {

	rc := setupDefaultResMgmtClient(t)

	//prepare sample response
	response := new(pb.ChaincodeQueryResponse)
	chaincodes := make([]*pb.ChaincodeInfo, 1)
	chaincodes[0] = &pb.ChaincodeInfo{Name: "name", Path: "path", Version: "version"}
	response.Chaincodes = chaincodes
	responseBytes, err := proto.Marshal(response)

	// Setup targets
	var peers []fab.Peer
	peer := fcmocks.MockPeer{MockName: "Peer1", MockURL: "http://peer1.com",
		Status: http.StatusOK, MockRoles: []string{}, MockCert: nil, MockMSP: "Org1MSP", Payload: responseBytes}
	peers = append(peers, &peer)

	// Already installed chaincode request
	req := InstallCCRequest{Name: "name", Version: "version", Path: "path", Package: &api.CCPackage{Type: 1, Code: []byte("code")}}
	responses, err := rc.InstallCC(req, WithTargets(peers...))
	if err != nil {
		t.Fatal(err)
	}

	if responses == nil || len(responses) != 1 {
		t.Fatal("Should have one 'already installed' response")
	}

	if !strings.Contains(responses[0].Info, "already installed") {
		t.Fatal("Should have 'already installed' info set")
	}

	if responses[0].Target != peer.MockURL {
		t.Fatalf("Expecting %s target URL, got %s", peer.MockURL, responses[0].Target)
	}

	// Chaincode not found request (it will be installed)
	req = InstallCCRequest{Name: "ID", Version: "v0", Path: "path", Package: &api.CCPackage{Type: 1, Code: []byte("code")}}
	responses, err = rc.InstallCC(req, WithTargets(peers...))
	if err != nil {
		t.Fatal(err)
	}

	if responses[0].Target != peer.MockURL {
		t.Fatal("Wrong target URL set")
	}

	if strings.Contains(responses[0].Info, "already installed") {
		t.Fatal("Should not have 'already installed' info set since it was not previously installed")
	}

	// Chaincode that causes generic (system) error in installed chaincodes

	//prepare sample response

	//prepare sample response
	response = new(pb.ChaincodeQueryResponse)
	chaincodes = make([]*pb.ChaincodeInfo, 1)
	chaincodes[0] = &pb.ChaincodeInfo{Name: "name1", Path: "path1", Version: "version1"}
	response.Chaincodes = chaincodes
	responseBytes, _ = proto.Marshal(response)

	peer = fcmocks.MockPeer{MockName: "Peer1", MockURL: "http://peer1.com",
		Status: http.StatusOK, MockRoles: []string{}, MockCert: nil, MockMSP: "Org1MSP", Payload: responseBytes}

	req = InstallCCRequest{Name: "error", Version: "v0", Path: "", Package: &api.CCPackage{Type: 1, Code: []byte("code")}}
	_, err = rc.InstallCC(req, WithTarget(&peer))
	if err == nil {
		t.Fatalf("Should have failed since install cc returns an error in the client")
	}
}

func TestInstallCC(t *testing.T) {

	rc := setupDefaultResMgmtClient(t)

	peer := fcmocks.MockPeer{MockName: "Peer1", MockURL: "http://peer1.com",
		Status: http.StatusOK, MockRoles: []string{}, MockCert: nil, MockMSP: "Org1MSP"}

	// Chaincode that is not installed already (it will be installed)
	req := InstallCCRequest{Name: "ID", Version: "v0", Path: "path", Package: &api.CCPackage{Type: 1, Code: []byte("code")}}
	responses, err := rc.InstallCC(req, WithTarget(&peer))
	if err != nil {
		t.Fatal(err)
	}
	if responses == nil || len(responses) != 1 {
		t.Fatal("Should have one successful response")
	}

	expected := "http://peer1.com"
	if responses[0].Target != expected {
		t.Fatalf("Expecting %s target URL, got %s", expected, responses[0].Target)
	}

	if responses[0].Status != http.StatusOK {
		t.Fatalf("Expecting %d status, got %d", 0, responses[0].Status)
	}

	// Chaincode that causes generic (system) error in installed chaincodes
	req = InstallCCRequest{Name: "error", Version: "v0", Path: "", Package: &api.CCPackage{Type: 1, Code: []byte("code")}}
	_, err = rc.InstallCC(req)
	if err == nil {
		t.Fatalf("Should have failed since install cc returns an error in the client")
	}
}

func TestInstallCCRequiredParameters(t *testing.T) {

	rc := setupDefaultResMgmtClient(t)

	// Test missing required parameters
	req := InstallCCRequest{}
	_, err := rc.InstallCC(req)
	if err == nil {
		t.Fatalf("Should have failed for empty install cc request")
	}

	// Test missing chaincode ID
	req = InstallCCRequest{Name: "", Version: "v0", Path: "path"}
	_, err = rc.InstallCC(req)
	if err == nil {
		t.Fatalf("Should have failed for empty cc ID")
	}

	// Test missing chaincode version
	req = InstallCCRequest{Name: "ID", Version: "", Path: "path"}
	_, err = rc.InstallCC(req)
	if err == nil {
		t.Fatalf("Should have failed for empty cc version")
	}

	// Test missing chaincode path
	req = InstallCCRequest{Name: "ID", Version: "v0", Path: ""}
	_, err = rc.InstallCC(req)
	if err == nil {
		t.Fatalf("InstallCC should have failed for empty cc path")
	}

	// Test missing chaincode package
	req = InstallCCRequest{Name: "ID", Version: "v0", Path: "path"}
	_, err = rc.InstallCC(req)
	if err == nil {
		t.Fatalf("InstallCC should have failed for nil chaincode package")
	}

	// Setup test client with different msp (default targets cannot be calculated)
	ctx := setupTestContext("test", "otherMSP")
	config := getNetworkConfig(t)
	ctx.SetConfig(config)

	// Create new resource management client ("otherMSP")
	rc = setupResMgmtClient(ctx, nil, t)
	req = InstallCCRequest{Name: "ID", Version: "v0", Path: "path", Package: &api.CCPackage{Type: 1, Code: []byte("code")}}

	// Test missing default targets
	_, err = rc.InstallCC(req)
	if err == nil {
		t.Fatalf("InstallCC should have failed with no default targets error")
	}

}

func TestInstallCCWithOptsRequiredParameters(t *testing.T) {

	rc := setupDefaultResMgmtClient(t)

	// Test missing required parameters
	req := InstallCCRequest{}
	_, err := rc.InstallCC(req)
	if err == nil {
		t.Fatalf("Should have failed for empty install cc request")
	}

	// Test missing chaincode ID
	req = InstallCCRequest{Name: "", Version: "v0", Path: "path"}
	_, err = rc.InstallCC(req)
	if err == nil {
		t.Fatalf("Should have failed for empty cc ID")
	}

	// Test missing chaincode version
	req = InstallCCRequest{Name: "ID", Version: "", Path: "path"}
	_, err = rc.InstallCC(req)
	if err == nil {
		t.Fatalf("Should have failed for empty cc version")
	}

	// Test missing chaincode path
	req = InstallCCRequest{Name: "ID", Version: "v0", Path: ""}
	_, err = rc.InstallCC(req)
	if err == nil {
		t.Fatalf("InstallCC should have failed for empty cc path")
	}

	// Test missing chaincode package
	req = InstallCCRequest{Name: "ID", Version: "v0", Path: "path"}
	_, err = rc.InstallCC(req)
	if err == nil {
		t.Fatalf("InstallCC should have failed for nil chaincode package")
	}

	// Valid request
	req = InstallCCRequest{Name: "name", Version: "version", Path: "path", Package: &api.CCPackage{Type: 1, Code: []byte("code")}}

	// Setup targets
	var peers []fab.Peer
	peer := fcmocks.MockPeer{MockName: "Peer1", MockURL: "http://peer1.com", MockRoles: []string{}, MockCert: nil, MockMSP: "Org1MSP"}
	peers = append(peers, &peer)

	// Test both targets and filter provided (error condition)
	_, err = rc.InstallCC(req, WithTargets(peers...), WithTargetFilter(&MSPFilter{mspID: "Org1MSP"}))
	if err == nil {
		t.Fatalf("Should have failed if both target and filter provided")
	}

	// Setup test client with different msp
	ctx := setupTestContext("test", "otherMSP")
	config := getNetworkConfig(t)
	ctx.SetConfig(config)

	// Create new resource management client ("otherMSP")
	rc = setupResMgmtClient(ctx, nil, t)

	// No targets and no filter -- default filter msp doesn't match discovery service peer msp
	_, err = rc.InstallCC(req)
	if err == nil {
		t.Fatalf("Should have failed with no targets error")
	}

	// Test filter only provided (filter rejects discovery service peer msp)
	_, err = rc.InstallCC(req, WithTargetFilter(&MSPFilter{mspID: "Org2MSP"}))
	if err == nil {
		t.Fatalf("Should have failed with no targets since filter rejected all discovery targets")
	}
}

func TestInstallCCDiscoveryError(t *testing.T) {

	// Setup test client and config
	ctx := setupTestContextWithDiscoveryError("test", "Org1MSP", errors.New("Test Error"))
	config := getNetworkConfig(t)
	ctx.SetConfig(config)

	// Create resource management client with discovery service that will generate an error
	rc := setupResMgmtClient(ctx, nil, t)

	// Test InstallCC discovery service error
	req := InstallCCRequest{Name: "ID", Version: "v0", Path: "path", Package: &api.CCPackage{Type: 1, Code: []byte("code")}}
	_, err := rc.InstallCC(req)
	if err == nil {
		t.Fatalf("Should have failed to install cc with discovery error")
	}

	// Test InstallCC discovery service error
	// if targets are not provided discovery service is used
	_, err = rc.InstallCC(req)
	if err == nil {
		t.Fatalf("Should have failed to install cc with opts with discovery error")
	}

}

func TestInstantiateCCRequiredParameters(t *testing.T) {

	rc := setupDefaultResMgmtClient(t)

	// Test missing required parameters
	req := InstantiateCCRequest{}

	// Test empty channel name
	err := rc.InstantiateCC("", req)
	if err == nil {
		t.Fatalf("Should have failed for empty request")
	}

	// Test empty request
	err = rc.InstantiateCC("mychannel", req)
	if err == nil {
		t.Fatalf("Should have failed for empty request")
	}

	// Test missing chaincode ID
	req = InstantiateCCRequest{Name: "", Version: "v0", Path: "path"}
	err = rc.InstantiateCC("mychannel", req)
	if err == nil {
		t.Fatalf("Should have failed for empty cc name")
	}

	// Test missing chaincode version
	req = InstantiateCCRequest{Name: "ID", Version: "", Path: "path"}
	err = rc.InstantiateCC("mychannel", req)
	if err == nil {
		t.Fatalf("Should have failed for empty cc version")
	}

	// Test missing chaincode path
	req = InstantiateCCRequest{Name: "ID", Version: "v0", Path: ""}
	err = rc.InstantiateCC("mychannel", req)
	if err == nil {
		t.Fatalf("Should have failed for empty cc path")
	}

	// Test missing chaincode policy
	req = InstantiateCCRequest{Name: "ID", Version: "v0", Path: "path"}
	err = rc.InstantiateCC("mychannel", req)
	if err == nil {
		t.Fatalf("Should have failed for nil chaincode policy")
	}

	// Setup test client with different msp (default targets cannot be calculated)
	ctx := setupTestContext("test", "otherMSP")
	config := getNetworkConfig(t)
	ctx.SetConfig(config)

	// Create new resource management client ("otherMSP")
	rc = setupResMgmtClient(ctx, nil, t)

	// Valid request
	ccPolicy := cauthdsl.SignedByMspMember("otherMSP")
	req = InstantiateCCRequest{Name: "name", Version: "version", Path: "path", Policy: ccPolicy}

	// Test missing default targets
	err = rc.InstantiateCC("mychannel", req)
	if err == nil {
		t.Fatalf("InstallCC should have failed with no default targets error")
	}

}

func TestInstantiateCCWithOptsRequiredParameters(t *testing.T) {

	rc := setupDefaultResMgmtClient(t)

	// Test missing required parameters
	req := InstantiateCCRequest{}

	// Test empty channel name
	err := rc.InstantiateCC("", req)
	if err == nil {
		t.Fatalf("Should have failed for empty channel name")
	}

	// Test empty request
	err = rc.InstantiateCC("mychannel", req)
	if err == nil {
		t.Fatalf("Should have failed for empty instantiate cc request")
	}

	// Test missing chaincode ID
	req = InstantiateCCRequest{Name: "", Version: "v0", Path: "path"}
	err = rc.InstantiateCC("mychannel", req)
	if err == nil {
		t.Fatalf("Should have failed for empty cc name")
	}

	// Test missing chaincode version
	req = InstantiateCCRequest{Name: "ID", Version: "", Path: "path"}
	err = rc.InstantiateCC("mychannel", req)
	if err == nil {
		t.Fatalf("Should have failed for empty cc version")
	}

	// Test missing chaincode path
	req = InstantiateCCRequest{Name: "ID", Version: "v0", Path: ""}
	err = rc.InstantiateCC("mychannel", req)
	if err == nil {
		t.Fatalf("Should have failed for empty cc path")
	}

	// Test missing chaincode policy
	req = InstantiateCCRequest{Name: "ID", Version: "v0", Path: "path"}
	err = rc.InstantiateCC("mychannel", req)
	if err == nil {
		t.Fatalf("Should have failed for missing chaincode policy")
	}

	// Valid request
	ccPolicy := cauthdsl.SignedByMspMember("Org1MSP")
	req = InstantiateCCRequest{Name: "name", Version: "version", Path: "path", Policy: ccPolicy}

	// Setup targets
	var peers []fab.Peer
	peer := fcmocks.MockPeer{MockName: "Peer1", MockURL: "http://peer1.com", MockRoles: []string{}, MockCert: nil, MockMSP: "Org1MSP"}
	peers = append(peers, &peer)

	// Test both targets and filter provided (error condition)
	err = rc.InstantiateCC("mychannel", req, WithTargets(peers...), WithTargetFilter(&MSPFilter{mspID: "Org1MSP"}))
	if err == nil {
		t.Fatalf("Should have failed if both target and filter provided")
	}

	// Setup test client with different msp
	ctx := setupTestContext("test", "otherMSP")
	config := getNetworkConfig(t)
	ctx.SetConfig(config)

	// Create new resource management client ("otherMSP")
	rc = setupResMgmtClient(ctx, nil, t)

	// No targets and no filter -- default filter msp doesn't match discovery service peer msp
	err = rc.InstantiateCC("mychannel", req)
	if err == nil {
		t.Fatalf("Should have failed with no targets error")
	}

	// Test filter only provided (filter rejects discovery service peer msp)
	err = rc.InstantiateCC("mychannel", req, WithTargetFilter(&MSPFilter{mspID: "Org2MSP"}))
	if err == nil {
		t.Fatalf("Should have failed with no targets since filter rejected all discovery targets")
	}
}

func TestInstantiateCCDiscoveryError(t *testing.T) {

	// Setup test client and config
	ctx := setupTestContext("test", "Org1MSP")
	config := getNetworkConfig(t)
	ctx.SetConfig(config)

	// Create resource management client with discovery service that will generate an error
	rc := setupResMgmtClient(ctx, errors.New("Test Error"), t)

	orderer := fcmocks.NewMockOrderer("", nil)

	transactor := txnmocks.MockTransactor{
		Ctx:       ctx,
		ChannelID: "mychannel",
		Orderers:  []fab.Orderer{orderer},
	}
	rc.ctx.ChannelProvider().(*fcmocks.MockChannelProvider).SetTransactor(&transactor)

	// Start mock event hub
	eventServer, err := fcmocks.StartMockEventServer(fmt.Sprintf("%s:%d", "127.0.0.1", 7053))
	if err != nil {
		t.Fatalf("Failed to start mock event hub: %v", err)
	}
	defer eventServer.Stop()

	ccPolicy := cauthdsl.SignedByMspMember("Org1MSP")
	req := InstantiateCCRequest{Name: "name", Version: "version", Path: "path", Policy: ccPolicy}

	// Test InstantiateCC create new discovery service per channel error
	err = rc.InstantiateCC("error", req)
	if err == nil {
		t.Fatalf("Should have failed to instantiate cc with create discovery service error")
	}

	// Test InstantiateCC discovery service get peers error
	err = rc.InstantiateCC("mychannel", req)
	if err == nil {
		t.Fatalf("Should have failed to instantiate cc with get peers discovery error")
	}

	// Test InstantiateCCWithOpts create new discovery service per channel error
	err = rc.InstantiateCC("error", req)
	if err == nil {
		t.Fatalf("Should have failed to instantiate cc with opts with create discovery service error")
	}

	// Test InstantiateCCWithOpts discovery service get peers error
	// if targets are not provided discovery service is used
	err = rc.InstantiateCC("mychannel", req)
	if err == nil {
		t.Fatalf("Should have failed to instantiate cc with opts with get peers discovery error")
	}

}

func TestUpgradeCCRequiredParameters(t *testing.T) {

	rc := setupDefaultResMgmtClient(t)

	// Test missing required parameters
	req := UpgradeCCRequest{}

	// Test empty channel name
	err := rc.UpgradeCC("", req)
	if err == nil {
		t.Fatalf("Should have failed for empty channel name")
	}

	// Test empty request
	err = rc.UpgradeCC("mychannel", req)
	if err == nil {
		t.Fatalf("Should have failed for empty upgrade cc request")
	}

	// Test missing chaincode ID
	req = UpgradeCCRequest{Name: "", Version: "v0", Path: "path"}
	err = rc.UpgradeCC("mychannel", req)
	if err == nil {
		t.Fatalf("Should have failed for empty cc name")
	}

	// Test missing chaincode version
	req = UpgradeCCRequest{Name: "ID", Version: "", Path: "path"}
	err = rc.UpgradeCC("mychannel", req)
	if err == nil {
		t.Fatalf("Should have failed for empty cc version")
	}

	// Test missing chaincode path
	req = UpgradeCCRequest{Name: "ID", Version: "v0", Path: ""}
	err = rc.UpgradeCC("mychannel", req)
	if err == nil {
		t.Fatalf("Should have failed for empty cc path")
	}

	// Test missing chaincode policy
	req = UpgradeCCRequest{Name: "ID", Version: "v0", Path: "path"}
	err = rc.UpgradeCC("mychannel", req)
	if err == nil {
		t.Fatalf("Should have failed for nil chaincode policy")
	}

	// Setup test client with different msp (default targets cannot be calculated)
	ctx := setupTestContext("test", "otherMSP")
	config := getNetworkConfig(t)
	ctx.SetConfig(config)

	// Create new resource management client ("otherMSP")
	rc = setupResMgmtClient(ctx, nil, t)

	// Valid request
	ccPolicy := cauthdsl.SignedByMspMember("otherMSP")
	req = UpgradeCCRequest{Name: "name", Version: "version", Path: "path", Policy: ccPolicy}

	// Test missing default targets
	err = rc.UpgradeCC("mychannel", req)
	if err == nil {
		t.Fatalf("Should have failed with no default targets error")
	}

}

func TestUpgradeCCWithOptsRequiredParameters(t *testing.T) {

	rc := setupDefaultResMgmtClient(t)

	// Test missing required parameters
	req := UpgradeCCRequest{}

	// Test empty channel name
	err := rc.UpgradeCC("", req)
	if err == nil {
		t.Fatalf("Should have failed for empty channel name")
	}

	// Test empty request
	err = rc.UpgradeCC("mychannel", req)
	if err == nil {
		t.Fatalf("Should have failed for empty upgrade cc request")
	}

	// Test missing chaincode ID
	req = UpgradeCCRequest{Name: "", Version: "v0", Path: "path"}
	err = rc.UpgradeCC("mychannel", req)
	if err == nil {
		t.Fatalf("Should have failed for empty cc name")
	}

	// Test missing chaincode version
	req = UpgradeCCRequest{Name: "ID", Version: "", Path: "path"}
	err = rc.UpgradeCC("mychannel", req)
	if err == nil {
		t.Fatalf("Should have failed for empty cc version")
	}

	// Test missing chaincode path
	req = UpgradeCCRequest{Name: "ID", Version: "v0", Path: ""}
	err = rc.UpgradeCC("mychannel", req)
	if err == nil {
		t.Fatalf("UpgradeCC should have failed for empty cc path")
	}

	// Test missing chaincode policy
	req = UpgradeCCRequest{Name: "ID", Version: "v0", Path: "path"}
	err = rc.UpgradeCC("mychannel", req)
	if err == nil {
		t.Fatalf("UpgradeCC should have failed for missing chaincode policy")
	}

	// Valid request
	ccPolicy := cauthdsl.SignedByMspMember("Org1MSP")
	req = UpgradeCCRequest{Name: "name", Version: "version", Path: "path", Policy: ccPolicy}

	// Setup targets
	var peers []fab.Peer
	peer := fcmocks.MockPeer{MockName: "Peer1", MockURL: "http://peer1.com", MockRoles: []string{}, MockCert: nil, MockMSP: "Org1MSP"}
	peers = append(peers, &peer)

	// Test both targets and filter provided (error condition)
	err = rc.UpgradeCC("mychannel", req, WithTargets(peers...), WithTargetFilter(&MSPFilter{mspID: "Org1MSP"}))
	if err == nil {
		t.Fatalf("Should have failed if both target and filter provided")
	}

	// Setup test client with different msp
	ctx := setupTestContext("test", "otherMSP")
	config := getNetworkConfig(t)
	ctx.SetConfig(config)

	// Create new resource management client ("otherMSP")
	rc = setupResMgmtClient(ctx, nil, t)

	// No targets and no filter -- default filter msp doesn't match discovery service peer msp
	err = rc.UpgradeCC("mychannel", req)
	if err == nil {
		t.Fatalf("Should have failed with no targets error")
	}

	// Test filter only provided (filter rejects discovery service peer msp)
	err = rc.UpgradeCC("mychannel", req, WithTargetFilter(&MSPFilter{mspID: "Org2MSP"}))
	if err == nil {
		t.Fatalf("Should have failed with no targets since filter rejected all discovery targets")
	}
}

func TestUpgradeCCDiscoveryError(t *testing.T) {

	// Setup test client and config
	ctx := setupTestContextWithDiscoveryError("test", "Org1MSP", nil)
	config := getNetworkConfig(t)
	ctx.SetConfig(config)

	// Create resource management client with discovery service that will generate an error
	rc := setupResMgmtClient(ctx, errors.New("Test Error"), t)
	orderer := fcmocks.NewMockOrderer("", nil)

	transactor := txnmocks.MockTransactor{
		Ctx:       ctx,
		ChannelID: "mychannel",
		Orderers:  []fab.Orderer{orderer},
	}
	rc.ctx.ChannelProvider().(*fcmocks.MockChannelProvider).SetTransactor(&transactor)

	// Start mock event hub
	eventServer, err := fcmocks.StartMockEventServer(fmt.Sprintf("%s:%d", "127.0.0.1", 7053))
	if err != nil {
		t.Fatalf("Failed to start mock event hub: %v", err)
	}
	defer eventServer.Stop()

	// Test UpgradeCC discovery service error
	ccPolicy := cauthdsl.SignedByMspMember("Org1MSP")
	req := UpgradeCCRequest{Name: "name", Version: "version", Path: "path", Policy: ccPolicy}

	// Test error while creating discovery service for channel "error"
	err = rc.UpgradeCC("error", req)
	if err == nil {
		t.Fatalf("Should have failed to upgrade cc with discovery error")
	}

	// Test error in discovery service while getting peers
	err = rc.UpgradeCC("mychannel", req)
	if err == nil {
		t.Fatalf("Should have failed to upgrade cc with discovery error")
	}

	// Test UpgradeCCWithOpts discovery service error when creating discovery service for channel 'error'
	err = rc.UpgradeCC("error", req)
	if err == nil {
		t.Fatalf("Should have failed to upgrade cc with opts with discovery error")
	}

	// Test UpgradeCCWithOpts discovery service error
	// if targets are not provided discovery service is used to get targets
	err = rc.UpgradeCC("mychannel", req)
	if err == nil {
		t.Fatalf("Should have failed to upgrade cc with opts with discovery error")
	}

}

func TestCCProposal(t *testing.T) {

	grpcServer := grpc.NewServer()
	defer grpcServer.Stop()

	// Setup mock targets
	endorserServer, addr := startEndorserServer(t, grpcServer)
	time.Sleep(2 * time.Second)

	ctx := setupTestContext("Admin", "Org1MSP")

	// Setup resource management client
	cfg, err := config.FromFile("./testdata/ccproposal_test.yaml")()
	if err != nil {
		t.Fatal(err)
	}
	ctx.SetConfig(cfg)

	// Setup target peers
	var peers []fab.Peer
	peer1, _ := peer.New(fcmocks.NewMockConfig(), peer.WithURL(addr))
	peers = append(peers, peer1)

	// Create mock orderer
	orderer := fcmocks.NewMockOrderer("", nil)
	rc := setupResMgmtClient(ctx, nil, t)

	transactor := txnmocks.MockTransactor{
		Ctx:       ctx,
		ChannelID: "mychannel",
		Orderers:  []fab.Orderer{orderer},
	}
	rc.ctx.ChannelProvider().(*fcmocks.MockChannelProvider).SetTransactor(&transactor)

	ccPolicy := cauthdsl.SignedByMspMember("Org1MSP")
	instantiateReq := InstantiateCCRequest{Name: "name", Version: "version", Path: "path", Policy: ccPolicy}

	// Test failed proposal error handling (endorser returns an error)
	endorserServer.ProposalError = errors.New("Test Error")

	err = rc.InstantiateCC("mychannel", instantiateReq, WithTargets(peers...))
	if err == nil {
		t.Fatalf("Should have failed to instantiate cc due to endorser error")
	}

	upgradeRequest := UpgradeCCRequest{Name: "name", Version: "version", Path: "path", Policy: ccPolicy}
	err = rc.UpgradeCC("mychannel", upgradeRequest, WithTargets(peers...))
	if err == nil {
		t.Fatalf("Should have failed to upgrade cc due to endorser error")
	}

	// Remove endorser error
	endorserServer.ProposalError = nil

	// Test error connecting to event hub
	err = rc.InstantiateCC("mychannel", instantiateReq)
	if err == nil {
		t.Fatalf("Should have failed to get event hub since not setup")
	}

	// Start mock event hub
	eventServer, err := fcmocks.StartMockEventServer(fmt.Sprintf("%s:%d", "127.0.0.1", 7053))
	if err != nil {
		t.Fatalf("Failed to start mock event hub: %v", err)
	}
	defer eventServer.Stop()

	// Test error in commit
	err = rc.InstantiateCC("mychannel", instantiateReq)
	if err == nil {
		t.Fatalf("Should have failed due to error in commit")
	}

	// Test invalid function (only 'instatiate' and 'upgrade' are supported)
	err = rc.sendCCProposal(3, "mychannel", instantiateReq, WithTargets(peers...))
	if err == nil {
		t.Fatalf("Should have failed for invalid function name")
	}

	// Test no event source in config
	cfg, err = config.FromFile("./testdata/event_source_missing_test.yaml")()
	if err != nil {
		t.Fatal(err)
	}
	ctx.SetConfig(cfg)
	rc = setupResMgmtClient(ctx, nil, t, getDefaultTargetFilterOption())
	err = rc.InstantiateCC("mychannel", instantiateReq)
	if err == nil {
		t.Fatalf("Should have failed since no event source has been configured")
	}
}

func getDefaultTargetFilterOption() ClientOption {
	targetFilter := &MSPFilter{mspID: "Org1MSP"}
	return WithDefaultTargetFilter(targetFilter)
}

func setupTestDiscovery(discErr error, peers []fab.Peer) (fab.DiscoveryProvider, error) {

	mockDiscovery, err := txnmocks.NewMockDiscoveryProvider(discErr, peers)
	if err != nil {
		return nil, errors.WithMessage(err, "NewMockDiscoveryProvider failed")
	}

	return mockDiscovery, nil
}

func setupDefaultResMgmtClient(t *testing.T) *Client {
	ctx := setupTestContext("test", "Org1MSP")
	network := getNetworkConfig(t)
	ctx.SetConfig(network)
	return setupResMgmtClient(ctx, nil, t, getDefaultTargetFilterOption())
}

func setupResMgmtClient(fabCtx context.Client, discErr error, t *testing.T, opts ...ClientOption) *Client {

	chProvider, err := fcmocks.NewMockChannelProvider(fabCtx)
	if err != nil {
		t.Fatalf("Failed to setup channel provider: %s", err)
	}

	transactor := txnmocks.MockTransactor{
		Ctx:       fabCtx,
		ChannelID: "",
	}
	chProvider.SetTransactor(&transactor)

	ctx := createClientContext(fabCtx)

	resClient, err := New(ctx, opts...)
	if err != nil {
		t.Fatalf("Failed to create new client with options: %s %v", err, opts)
	}

	return resClient

}

func setupTestContext(userName string, mspID string) *fcmocks.MockContext {
	user := fcmocks.NewMockUserWithMSPID(userName, mspID)
	ctx := fcmocks.NewMockContext(user)
	return ctx
}

func setupCustomOrderer(ctx *fcmocks.MockContext, mockOrderer fab.Orderer) *fcmocks.MockContext {
	mockInfraProvider := &fcmocks.MockInfraProvider{}
	mockInfraProvider.SetCustomOrderer(mockOrderer)
	ctx.SetCustomInfraProvider(mockInfraProvider)
	return ctx
}

func setupTestContextWithDiscoveryError(userName string, mspID string, discErr error) *fcmocks.MockContext {
	user := fcmocks.NewMockUserWithMSPID(userName, mspID)
	dscPvdr, _ := setupTestDiscovery(discErr, nil)
	//ignore err and set whatever you get in dscPvdr
	ctx := fcmocks.NewMockContextWithCustomDiscovery(user, dscPvdr)
	return ctx
}

func startEndorserServer(t *testing.T, grpcServer *grpc.Server) (*fcmocks.MockEndorserServer, string) {
	lis, err := net.Listen("tcp", "127.0.0.1:7051")
	addr := lis.Addr().String()

	endorserServer := &fcmocks.MockEndorserServer{}
	pb.RegisterEndorserServer(grpcServer, endorserServer)
	if err != nil {
		t.Logf("Error starting test server %s", err)
		t.FailNow()
	}
	t.Logf("Starting test server on %s\n", addr)
	go grpcServer.Serve(lis)
	return endorserServer, addr
}

func getNetworkConfig(t *testing.T) core.Config {
	config, err := config.FromFile(networkCfg)()
	if err != nil {
		t.Fatal(err)
	}

	return config
}

func TestSaveChannelSuccess(t *testing.T) {

	grpcServer := grpc.NewServer()
	defer grpcServer.Stop()
	fcmocks.StartMockBroadcastServer("127.0.0.1:7050", grpcServer)

	ctx := setupTestContext("test", "Org1MSP")

	mockConfig := &fcmocks.MockConfig{}
	grpcOpts := make(map[string]interface{})
	grpcOpts["allow-insecure"] = true

	oConfig := &core.OrdererConfig{
		URL:         "127.0.0.1:7050",
		GRPCOptions: grpcOpts,
	}
	mockConfig.SetCustomOrdererCfg(oConfig)
	ctx.SetConfig(mockConfig)

	cc := setupResMgmtClient(ctx, nil, t)

	// Test empty channel request
	err := cc.SaveChannel(SaveChannelRequest{})
	if err == nil {
		t.Fatalf("Should have failed for empty channel request")
	}

	// Test empty channel name
	err = cc.SaveChannel(SaveChannelRequest{ChannelID: "", ChannelConfig: channelConfig})
	if err == nil {
		t.Fatalf("Should have failed for empty channel id")
	}

	// Test empty channel config
	err = cc.SaveChannel(SaveChannelRequest{ChannelID: "mychannel", ChannelConfig: ""})
	if err == nil {
		t.Fatalf("Should have failed for empty channel config")
	}

	// Test extract configuration error
	err = cc.SaveChannel(SaveChannelRequest{ChannelID: "mychannel", ChannelConfig: "./testdata/extractcherr.tx"})
	if err == nil {
		t.Fatalf("Should have failed to extract configuration")
	}

	// Test sign channel error
	err = cc.SaveChannel(SaveChannelRequest{ChannelID: "mychannel", ChannelConfig: "./testdata/signcherr.tx"})
	if err == nil {
		t.Fatalf("Should have failed to sign configuration")
	}

	// Test valid Save Channel request (success)
	err = cc.SaveChannel(SaveChannelRequest{ChannelID: "mychannel", ChannelConfig: channelConfig}, WithOrdererID("example.com"))
	if err != nil {
		t.Fatal(err)
	}

}

func TestSaveChannelFailure(t *testing.T) {

	// Set up context with error in create channel
	user := fcmocks.NewMockUser("test")
	errCtx := fcmocks.NewMockContext(user)
	network := getNetworkConfig(t)
	errCtx.SetConfig(network)
	fabCtx := setupTestContext("user", "Org1Msp1")

	clientCtx := createClientContext(contextImpl.Client{
		Providers: fabCtx,
		Identity:  fabCtx,
	})

	cc, err := New(clientCtx)
	if err != nil {
		t.Fatalf("Failed to create new channel management client: %s", err)
	}

	// Test create channel failure
	err = cc.SaveChannel(SaveChannelRequest{ChannelID: "mychannel", ChannelConfig: channelConfig})
	if err == nil {
		t.Fatal("Should have failed with create channel error")
	}

}

func TestSaveChannelWithOpts(t *testing.T) {

	grpcServer := grpc.NewServer()
	defer grpcServer.Stop()
	fcmocks.StartMockBroadcastServer("127.0.0.1:7050", grpcServer)

	ctx := setupTestContext("test", "Org1MSP")

	mockConfig := &fcmocks.MockConfig{}
	grpcOpts := make(map[string]interface{})
	grpcOpts["allow-insecure"] = true

	oConfig := &core.OrdererConfig{
		URL:         "127.0.0.1:7050",
		GRPCOptions: grpcOpts,
	}
	mockConfig.SetCustomOrdererCfg(oConfig)
	mockConfig.SetCustomRandomOrdererCfg(oConfig)
	ctx.SetConfig(mockConfig)

	cc := setupResMgmtClient(ctx, nil, t)

	// Valid request (same for all options)
	req := SaveChannelRequest{ChannelID: "mychannel", ChannelConfig: channelConfig}

	// Test empty option (default order is random orderer from config)
	opts := WithOrdererID("")
	err := cc.SaveChannel(req, opts)
	if err != nil {
		t.Fatal(err)
	}

	// Test valid orderer ID
	opts = WithOrdererID("orderer.example.com")
	err = cc.SaveChannel(req, opts)
	if err != nil {
		t.Fatal(err)
	}

	// Test invalid orderer ID

	mockConfig = &fcmocks.MockConfig{}
	ctx.SetConfig(mockConfig)

	cc = setupResMgmtClient(ctx, nil, t)

	opts = WithOrdererID("Invalid")
	err = cc.SaveChannel(req, opts)
	if err == nil {
		t.Fatal("Should have failed for invalid orderer ID")
	}
}

func TestJoinChannelWithInvalidOpts(t *testing.T) {

	cc := setupDefaultResMgmtClient(t)
	opts := WithOrdererID("Invalid")
	err := cc.JoinChannel("mychannel", opts)
	if err == nil {
		t.Fatal("Should have failed for invalid orderer ID")
	}

}

func TestSaveChannelWithMultipleSigningIdenities(t *testing.T) {
	grpcServer := grpc.NewServer()
	defer grpcServer.Stop()
	fcmocks.StartMockBroadcastServer("127.0.0.1:7050", grpcServer)
	ctx := setupTestContext("test", "Org1MSP")

	mockConfig := &fcmocks.MockConfig{}
	grpcOpts := make(map[string]interface{})
	grpcOpts["allow-insecure"] = true

	oConfig := &core.OrdererConfig{
		URL:         "127.0.0.1:7050",
		GRPCOptions: grpcOpts,
	}
	mockConfig.SetCustomRandomOrdererCfg(oConfig)
	mockConfig.SetCustomOrdererCfg(oConfig)
	ctx.SetConfig(mockConfig)

	cc := setupResMgmtClient(ctx, nil, t)

	// empty list of signing identities (defaults to context user)
	req := SaveChannelRequest{ChannelID: "mychannel", ChannelConfig: channelConfig, SigningIdentities: []fab.IdentityContext{}}
	err := cc.SaveChannel(req, WithOrdererID(""))
	if err != nil {
		t.Fatalf("Failed to save channel with default signing identity: %s", err)
	}

	// multiple signing identities
	secondCtx := fcmocks.NewMockContext(fcmocks.NewMockUser("second"))
	req = SaveChannelRequest{ChannelID: "mychannel", ChannelConfig: channelConfig, SigningIdentities: []fab.IdentityContext{cc.ctx, secondCtx}}
	err = cc.SaveChannel(req, WithOrdererID(""))
	if err != nil {
		t.Fatalf("Failed to save channel with multiple signing identities: %s", err)
	}
}

func createClientContext(fabCtx context.Client) context.ClientProvider {
	return func() (context.Client, error) {
		return fabCtx, nil
	}
}