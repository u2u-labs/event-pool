package server

import (
	"context"

	"event-pool/network/common"
	"event-pool/server/proto"
	"github.com/libp2p/go-libp2p/core/peer"
	empty "google.golang.org/protobuf/types/known/emptypb"
)

type systemService struct {
	proto.UnimplementedSystemServer

	server *Server
}

// GetStatus returns the current system status, in the form of:
//
// P2PAddr: <libp2pAddress>
func (s *systemService) GetStatus(ctx context.Context, req *empty.Empty) (*proto.ServerStatus, error) {
	status := &proto.ServerStatus{
		P2PAddr: common.AddrInfoToString(s.server.network.AddrInfo()),
	}

	return status, nil
}

// PeersAdd implements the 'peers add' operator service
func (s *systemService) PeersAdd(_ context.Context, req *proto.PeersAddRequest) (*proto.PeersAddResponse, error) {
	if joinErr := s.server.JoinPeer(req.Id); joinErr != nil {
		return &proto.PeersAddResponse{
			Message: "Unable to successfully add peer",
		}, joinErr
	}

	return &proto.PeersAddResponse{
		Message: "Peer address marked ready for dialing",
	}, nil
}

// PeersStatus implements the 'peers status' operator service
func (s *systemService) PeersStatus(ctx context.Context, req *proto.PeersStatusRequest) (*proto.Peer, error) {
	peerID, err := peer.Decode(req.Id)
	if err != nil {
		return nil, err
	}

	peerRs, err := s.getPeer(peerID)
	if err != nil {
		return nil, err
	}

	return peerRs, nil
}

// getPeer returns a specific proto.Peer using the peer ID
func (s *systemService) getPeer(id peer.ID) (*proto.Peer, error) {
	protocols, err := s.server.network.GetProtocols(id)
	if err != nil {
		return nil, err
	}

	info := s.server.network.GetPeerInfo(id)

	addrs := []string{}
	for _, addr := range info.Addrs {
		addrs = append(addrs, addr.String())
	}

	peerRs := &proto.Peer{
		Id:        id.String(),
		Protocols: protocols,
		Addrs:     addrs,
	}

	return peerRs, nil
}

// PeersList implements the 'peers list' operator service
func (s *systemService) PeersList(
	ctx context.Context,
	req *empty.Empty,
) (*proto.PeersListResponse, error) {
	resp := &proto.PeersListResponse{
		Peers: []*proto.Peer{},
	}

	peers := s.server.network.Peers()
	for _, p := range peers {
		peerRs, err := s.getPeer(p.Info.ID)
		if err != nil {
			return nil, err
		}

		resp.Peers = append(resp.Peers, peerRs)
	}

	return resp, nil
}
