package peering

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/MadBase/MadNet/constants"
	"github.com/MadBase/MadNet/interfaces"
	"github.com/MadBase/MadNet/logging"
	pb "github.com/MadBase/MadNet/proto"
	"github.com/MadBase/MadNet/transport"
	"github.com/MadBase/MadNet/types"
	"github.com/MadBase/MadNet/utils"
	"github.com/sirupsen/logrus"
)

// PeerManager is a self contained system for management of peering.
// Other packages that need to send data to peers may subscribe to the
// peer manager and be notified of active peers. This notification
// occurs through the peer subscription system.
type PeerManager struct {
	sync.RWMutex
	ctx                      context.Context
	cf                       func()
	wg                       *sync.WaitGroup
	logger                   *logrus.Logger
	closeOnce                sync.Once
	closeChan                chan struct{}
	discServerHandler        *ServerHandler
	mux                      *transport.P2PMux
	p2pServerHandler         *MuxHandler
	clientHandler            *clientHandler
	bootNodes                *bootNodeList
	transport                interfaces.P2PTransport
	inactive                 *inactivePeerStore
	active                   *activePeerStore
	subscribers              map[int]*PeerSubscription
	subscriberCount          int
	peeringCompleteThreshold int
	peeringMaxThreshold      int
	fireWallMode             bool
	fireWallHost             interfaces.NodeAddr
	peeringComplete          bool
	upnpMapper               *transport.UPnPMapper
}

// NewPeerManager creates a new peer manager based on the Configuration
// values passed to the process.
func NewPeerManager(p2pServer interfaces.P2PServer, chainID uint32, pLimMin int, pLimMax int, fwMode bool, fwHost, listenAddr, tprivk string, upnp bool) (*PeerManager, error) {
	logger := logging.GetLogger(constants.LoggerPeerMan)
	ctx := context.Background()
	subCtx, cf := context.WithCancel(ctx)
	host, portstr, err := net.SplitHostPort(listenAddr) // config.Configuration.Transport.P2PListeningAddress
	if err != nil {
		utils.DebugTrace(logger, err)
		cf()
		return nil, err
	}
	port, err := strconv.Atoi(portstr)
	if err != nil {
		utils.DebugTrace(logger, err)
		cf()
		return nil, err
	}
	p2ptransport, err := transport.NewP2PTransport(logging.GetLogger(constants.LoggerTransport), types.ChainIdentifier(chainID), tprivk, port, host) // config.Configuration.Chain.ID, config.Configuration.Transport.PrivateKey
	if err != nil {
		utils.DebugTrace(logger, err)
		cf()
		return nil, err
	}
	var upnpMapper *transport.UPnPMapper
	if upnp {
		upnpMapper, err = transport.NewUPnPMapper(logging.GetLogger(constants.LoggerUPnP), port)
		if err != nil {
			utils.DebugTrace(logger, err)
			cf()
			return nil, err
		}
	}
	// create the actual peer manager
	pm := &PeerManager{
		ctx:                      subCtx,
		cf:                       cf,
		logger:                   logger,
		wg:                       &sync.WaitGroup{},
		closeChan:                make(chan struct{}),
		peeringCompleteThreshold: pLimMin, // config.Configuration.Transport.PeerLimitMin
		peeringMaxThreshold:      pLimMax, // config.Configuration.Transport.PeerLimitMax
		bootNodes:                &bootNodeList{},
		subscribers:              make(map[int]*PeerSubscription),
		clientHandler:            newClientHandler(),
		active: &activePeerStore{
			canClose:  true,
			store:     make(map[string]interfaces.P2PClient),
			pid:       make(map[string]uint64),
			closeChan: make(chan struct{}),
			closeOnce: sync.Once{},
		},
		inactive: &inactivePeerStore{
			store:     make(map[string]interfaces.NodeAddr),
			cooldown:  make(map[string]uint64),
			closeChan: make(chan struct{}),
			closeOnce: sync.Once{},
		},
		mux:              &transport.P2PMux{},
		transport:        p2ptransport,
		p2pServerHandler: NewMuxServerHandler(logger, p2ptransport.NodeAddr(), p2pServer),
		upnpMapper:       upnpMapper,
	}
	pm.discServerHandler = NewDiscoveryServerHandler(logger, p2ptransport.NodeAddr(), pm)
	if fwMode { // config.Configuration.Transport.FirewallMode
		pm.logger.Info("RUNNING IN FIREWALL MODE")
		pm.fireWallMode = true
		naddr, err := transport.NewNodeAddr(fwHost) // config.Configuration.Transport.FirewallHost
		if err != nil {
			return nil, err
		}
		pm.fireWallHost = naddr
	}
	// make sure bootnodes parse
	if _, err := pm.bootNodes.randomBootNode(); err != nil {
		utils.DebugTrace(pm.logger, err)
		return nil, err
	}
	return pm, nil
}

// Start launches the background loops of the peer manager
func (ps *PeerManager) Start() {
	ps.wg.Add(2)
	go ps.runDiscoveryLoops()
	go ps.acceptLoop()
	if ps.upnpMapper != nil {
		go ps.upnpMapper.Start()
	}
	<-ps.CloseChan()
}

// isMe verifies returns true if the public key of the node addr is the same
// as the local node's public key.
func (ps *PeerManager) isMe(addr interfaces.NodeAddr) bool {
	return addr.Identity() == ps.transport.NodeAddr().Identity()
}

// CloseChan returns a channel that is closed when the peerManager is
// shutting down.
func (ps *PeerManager) CloseChan() <-chan struct{} {
	return ps.closeChan
}

// Close will shutdown the peer manager causing all transports and connections
// to be closed as well.
func (ps *PeerManager) Close() error {
	fn := func() {
		close(ps.closeChan)
		ps.logger.Warning("PeerManager Closing")
		ps.cf()
		ps.logger.Warning("PeerManager stopping p2pmuxTransport")
		err := ps.transport.Close()
		if err != nil {
			utils.DebugTrace(ps.logger, err)
		}
		ps.logger.Warning("PeerManager stopping discServerHandler")
		err = ps.discServerHandler.Close()
		if err != nil {
			utils.DebugTrace(ps.logger, err)
		}
		ps.logger.Warning("PeerManager stopping muxServerHandler")
		err = ps.p2pServerHandler.Close()
		if err != nil {
			utils.DebugTrace(ps.logger, err)
		}
		ps.active.close()
		ps.inactive.close()
		for _, s := range ps.subscribers {
			s.close()
		}
		if ps.upnpMapper != nil {
			ps.logger.Warning("PeerManager stopping upnp mapper")
			ps.upnpMapper.Close()
		}
		ps.wg.Wait()
		ps.logger.Warning("PeerManager Graceful exit complete")
	}
	ps.closeOnce.Do(fn)
	return nil
}

// acceptLoop accepts incoming peer connections
func (ps *PeerManager) acceptLoop() {
	defer func() { go ps.Close() }()
	defer ps.wg.Done()
	for {
		conn, err := ps.transport.Accept()
		if err != nil {
			utils.DebugTrace(ps.logger, err)
			return
		}
		switch conn.Protocol() {
		case types.P2PProtocol:
			go ps.handleP2P(conn)
		case types.DiscProtocol:
			go ps.handleDisc(conn)
		default:
			err := conn.Close()
			if err != nil {
				utils.DebugTrace(ps.logger, err)
			}
		}
	}
}

// handle discovery dials from remote peers
func (ps *PeerManager) handleDisc(conn interfaces.P2PConn) {
	defer func() {
		defer conn.Close()
		time.Sleep(7 * time.Second)
	}()
	err := ps.discServerHandler.HandleConnection(conn)
	if err != nil {
		return
	}
	func() {
		ps.Lock()
		defer ps.Unlock()
		if !ps.active.contains(conn.NodeAddr()) {
			ps.inactive.add(conn.NodeAddr())
		}
	}()
}

// handle p2p dials from remote peers by tracking the connection
// in local stores and notifying subscribers
func (ps *PeerManager) handleP2P(conn interfaces.P2PConn) {
	ps.logger.Debugf("New connection in peerManager from %s", conn.NodeAddr().P2PAddr())
	ctx, cf := context.WithDeadline(ps.ctx, time.Now().Add(time.Second*5))
	defer cf()
	muxconn, err := ps.mux.HandleConnection(ctx, conn)
	if err != nil {
		utils.DebugTrace(ps.logger, err)
		err2 := conn.Close()
		if err2 != nil {
			utils.DebugTrace(ps.logger, err2)
		}
		return
	}
	client, err := ps.p2pServerHandler.HandleConnection(muxconn)
	if err != nil {
		utils.DebugTrace(ps.logger, err)
		err2 := muxconn.Close()
		if err2 != nil {
			utils.DebugTrace(ps.logger, err2)
		}
		return
	}
	// must be done synchronously to protect data races
	func() {
		ps.Lock()
		defer ps.Unlock()
		ps.active.add(client)
		ps.inactive.del(client.NodeAddr())
		ps.notify(client)
	}()
}

func (ps *PeerManager) notify(c interfaces.P2PClient) {
	for _, s := range ps.subscribers {
		s.actives.add(c)
	}
}

// Subscribe returns a peer subscription. This allows the caller to
// have a remote copy of the active peer set for calling rpc methods on
// the peers without blocking the discovery system. The remote copy will
// update as peers are added and dropped, but any peer in the remote active
// set may have its connections closed at any time. Thus the caller should
// always gracefully handle communication failures.
func (ps *PeerManager) Subscribe() interfaces.PeerSubscription {
	ps.Lock()
	defer ps.Unlock()
	ps.subscriberCount++
	sub := &PeerSubscription{
		clientChan: make(chan interfaces.P2PClient, 16),
		closeChan:  make(chan struct{}),
		log:        logging.GetLogger(constants.LoggerPeer),
		closeOnce:  sync.Once{},
		actives: &activePeerStore{
			store:     make(map[string]interfaces.P2PClient),
			pid:       make(map[string]uint64),
			closeChan: make(chan struct{}),
			closeOnce: sync.Once{},
		},
	}
	ps.subscribers[ps.subscriberCount] = sub
	return sub
}

// dialp2p dials remote peers
func (ps *PeerManager) dialP2P(addr interfaces.NodeAddr) {
	conn, err := ps.transport.Dial(addr, types.P2PProtocol)
	if err != nil {
		utils.DebugTrace(ps.logger, err)
		return
	}
	go ps.handleP2P(conn)
}

// Counts returns the active and inactive peer counts
func (ps *PeerManager) Counts() (int, int) {
	return ps.active.len(), ps.inactive.len()
}

////////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////
//P2P SERVER HANDLERS///////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////

// HandleP2PGetPeers is the handler for the P2P method GetPeers
func (ps *PeerManager) HandleP2PGetPeers(ctx context.Context, req *pb.GetPeersRequest) (*pb.GetPeersResponse, error) {
	return ps.GetPeers(ctx, req)
}

// GetPeers is the handler for the get peers request.
func (ps *PeerManager) GetPeers(ctx context.Context, req *pb.GetPeersRequest) (*pb.GetPeersResponse, error) {
	resp := &pb.GetPeersResponse{
		Peers: []string{},
	}
	if ps.fireWallMode {
		return resp, nil
	}
	active, ok := ps.active.random()
	if ok {
		resp.Peers = append(resp.Peers, active)
	}
	inactive, ok := ps.inactive.random()
	if ok {
		resp.Peers = append(resp.Peers, inactive)
	}
	return resp, nil
}

////////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////
//P2P SERVER STATUS LOGGER /////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////

// Status returns the data needed for the status logger.
func (ps *PeerManager) Status(smap map[string]interface{}) (map[string]interface{}, error) {
	active, inactive := ps.Counts()
	smap["Peers"] = fmt.Sprintf("%d/%d/%d/%d", ps.peeringMaxThreshold, active, ps.peeringCompleteThreshold, inactive)
	return smap, nil
}

////////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////
//P2P SERVER DISCOVERY LOOPS ///////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////

// PeeringComplete returns true if the peering is complete for the node
func (ps *PeerManager) PeeringComplete() bool {
	ps.RLock()
	defer ps.RUnlock()
	return ps.peeringComplete
}

func (ps *PeerManager) runDiscoveryLoops() {
	defer ps.Close()
	defer ps.wg.Done()
	defer func() { ps.logger.Warning("Discovery loop exit") }()
	ps.wg.Add(5)
	go ps.doLoop("bootnode", ps.discoDialBootnode, time.Second*31)
	go ps.doLoop("inactive", ps.dialInactive, time.Second*13)
	go ps.doLoop("active", ps.getPeersActive, time.Second*17)
	go ps.doLoop("firewall", ps.dialFirewall, time.Second*10)
	go ps.doLoop("peerStatus", ps.peerStatus, time.Second*3)
	<-ps.CloseChan()
}

func (ps *PeerManager) doLoop(name string, fn func(), interval time.Duration) {
	defer ps.wg.Done()
	defer func() { go ps.Close() }()
	defer func() { ps.logger.Warningf("Discovery loop %s exited.", name) }()
	for {
		select {
		case <-ps.CloseChan():
			return
		default:
		}
		select {
		case <-ps.CloseChan():
			return
		case <-time.After(interval):
			fn()
		}
	}
}

func (ps *PeerManager) peerStatus() {
	ps.Lock()
	defer ps.Unlock()
	active, _ := ps.Counts()
	ps.peeringComplete = active >= ps.peeringCompleteThreshold
}

func (ps *PeerManager) getPeersActive() {
	smap := make(map[string]interface{})
	_, err := ps.Status(smap)
	if err != nil {
		utils.DebugTrace(ps.logger, err)
	}
	ps.logger.WithFields(smap).Debug("Running get peers active")
	active, _ := ps.Counts()
	if active < ps.peeringMaxThreshold && active > 0 {
		peer, ok := ps.active.randomClient()
		if !ok {
			return
		}
		ctx, cf := context.WithTimeout(ps.ctx, 5*time.Second)
		defer cf()
		resp, err := peer.GetPeers(ctx, &pb.GetPeersRequest{})
		if err != nil {
			utils.DebugTrace(ps.logger, err)
			return
		}
		for i := 0; i < len(resp.Peers); i++ {
			p, err := (*transport.NodeAddr).Unmarshal(nil, resp.Peers[i])
			if err != nil {
				continue
			}
			if ps.isMe(p) {
				continue
			}
			func() {
				ps.Lock()
				defer ps.Unlock()
				if !ps.active.contains(p) {
					ps.inactive.add(p)
				}
			}()
		}
	}
}

func (ps *PeerManager) discoDialBootnode() {
	smap := make(map[string]interface{})
	_, err := ps.Status(smap)
	if err != nil {
		utils.DebugTrace(ps.logger, err)
	}
	ps.logger.WithFields(smap).Debug("Running dial bootnode")
	// get counts
	active, inactive := ps.Counts()
	// if we have no known peers, call a boot node
	if active < ps.peeringMaxThreshold && inactive == 0 {
		bn, err := ps.bootNodes.randomBootNode()
		if err != nil {
			utils.DebugTrace(ps.logger, err)
			return
		}
		peers, err := ps.bootNodeProtocol(bn)
		if err != nil {
			utils.DebugTrace(ps.logger, err)
		}
		// add all peers as inactive
		for i := 0; i < len(peers); i++ {
			p := peers[i]
			if ps.isMe(p) {
				continue
			}
			func() {
				ps.Lock()
				defer ps.Unlock()
				if !ps.active.contains(p) {
					ps.inactive.add(p)
				}
			}()
		}
	}
}

func (ps *PeerManager) dialInactive() {
	smap := make(map[string]interface{})
	_, err := ps.Status(smap)
	if err != nil {
		utils.DebugTrace(ps.logger, err)
	}
	ps.logger.WithFields(smap).Debug("Running dial inactive")
	active, inactive := ps.Counts()
	if active < ps.peeringMaxThreshold {
		naddr, ok := ps.inactive.randomPop()
		if !ok {
			if inactive > 0 {
				ps.logger.Warning("Got back an invalid peer with valid peers possible.")
			}
			return
		}
		ps.dialP2P(naddr)
	}
}

func (ps *PeerManager) dialFirewall() {
	if ps.fireWallMode {
		if !ps.active.contains(ps.fireWallHost) {
			ps.dialP2P(ps.fireWallHost)
		}
	}
}

////////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////
//BOOTNODE DIALER //////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////

func (ps *PeerManager) bootNodeProtocol(nodeAddr interfaces.NodeAddr) ([]interfaces.NodeAddr, error) {
	conn, err := ps.transport.Dial(nodeAddr, types.Bootnode)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	gconn, err := ps.clientHandler.HandleConnection(conn)
	if err != nil {
		return nil, err
	}
	defer gconn.Close()
	bnc := pb.NewBootNodeClient(gconn)
	timeoutCtx, cf := context.WithTimeout(ps.ctx, time.Second*11)
	defer cf()
	resp, err := bnc.KnownNodes(timeoutCtx, &pb.BootNodeRequest{})
	if err != nil {
		return nil, err
	}
	var peerlist []interfaces.NodeAddr
	for i := 0; i < len(resp.Peers); i++ {
		p, err := (*transport.NodeAddr).Unmarshal(nil, resp.Peers[i])
		if err != nil {
			continue
		}
		peerlist = append(peerlist, p)
	}
	return peerlist, nil
}
