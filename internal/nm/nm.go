package nm

import (
	"log"
	"strings"
	"time"

	"github.com/spf13/viper"

	"github.com/the-maldridge/locksmith/internal/models"
	"github.com/the-maldridge/locksmith/internal/nm/ipam"
	"github.com/the-maldridge/locksmith/internal/nm/state"
)

func init() {
	viper.SetDefault("nm.state.impl", "json")
}

// New returns an initialized instance ready to go.
func New() (NetworkManager, error) {
	nm := NetworkManager{}

	s, err := state.Initialize(viper.GetString("nm.state.impl"))
	if err != nil {
		return NetworkManager{}, err
	}
	nm.Store = s

	nm.networks = parseNetworkConfig()
	nm.initializeIPAM()

	useExpiry := false
	for i := range nm.networks {
		if nm.networks[i].ApproveExpiry != 0 || nm.networks[i].ActivateExpiry != 0 {
			useExpiry = true
			// We can break now rather than continuing to
			// iterate since this timer is used for all
			// networks; its enabled if any one network
			// needs it.
			break
		}
	}
	// Launch the expiration timer
	if useExpiry {
		nm.ProcessExpirations()
		go nm.expirationTimer()
	}

	return nm, nil
}

func (nm *NetworkManager) initializeIPAM() {
	requiredIPAM := make(map[string]int)
	for _, net := range nm.networks {
		for _, p := range net.IPAM {
			requiredIPAM[p]++
		}
	}
	nm.ipam = make(map[string]ipam.IPAM)
	for k := range requiredIPAM {
		a, err := ipam.Initialize(k)
		if err != nil {
			log.Printf("Addresser '%s' is unavailable: '%s'.", k, err)
			continue
		}
		nm.ipam[k] = a
	}
}

// GetNet returns the network if known or an error if not known.
func (nm *NetworkManager) GetNet(id string) (Network, error) {
	net := Network{}

	for i := range nm.networks {
		if nm.networks[i].ID == id {
			net.NetConfig = nm.networks[i]
			s, err := nm.GetState(net.ID)
			if err != nil {
				return Network{}, err
			}
			net.NetState = s
			return net, nil
		}
	}
	return Network{}, ErrUnknownNetwork
}

// StoreNet is a convenience function that stores network state.
func (nm *NetworkManager) StoreNet(net Network) error {
	return nm.PutState(net.ID, net.NetState)
}

// AttemptNetworkRegistration as the name implies attempts to register
// the client into the named network.  It checks the pre-approve hooks
// to make sure we don't need to reject the client right away, and
// after we're confident they're OK, then we add them to a staged
// clients list.  Staged clients are de-staged asynchronously from
// this request.
func (nm *NetworkManager) AttemptNetworkRegistration(netID string, client models.Peer) error {
	net, err := nm.GetNet(netID)
	if err != nil {
		return err
	}

	for i := range net.PreApproveHooks {
		if err := nm.RunPreApproveHook(net.PreApproveHooks[i], netID, client); err != nil {
			return err
		}
	}

	if err := nm.stagePeer(netID, client); err != nil {
		return err
	}

	return nm.StoreNet(net)
}

// stagePeer takes a pre-approved peer and stages them.  If the
// ApproveMode is set to AUTO then the peer is not staged and is
// instead added directly to ApprovedPeers.
func (nm *NetworkManager) stagePeer(netID string, client models.Peer) error {
	net, err := nm.GetNet(netID)
	if err != nil {
		return err
	}

	net.StagedPeers[client.PubKey] = client
	log.Printf("Network '%s' has staged peer '%s'",
		net.Name,
		client.PubKey)

	if err := nm.StoreNet(net); err != nil {
		return err
	}

	if strings.ToUpper(net.ApproveMode) == "AUTO" {
		log.Printf("Network '%s' is automatically approving peer '%s'",
			net.Name,
			client.PubKey)
		return nm.ApprovePeer(netID, client.PubKey)
	}
	return nil
}

// ApprovePeer looks for a pubkey in StagedPeers and puts it into
// ApprovedPeers.
func (nm *NetworkManager) ApprovePeer(netID, pubkey string) error {
	net, err := nm.GetNet(netID)
	if err != nil {
		return err
	}

	peer, ok := net.StagedPeers[pubkey]
	if !ok {
		return ErrUnknownPeer
	}

	nm.configurePeer(&net, &peer)

	net.ApprovedPeers[pubkey] = peer
	delete(net.StagedPeers, pubkey)

	if net.ApproveExpiry != 0 {
		// Approvals expire for this network
		net.ApprovalExpirations[pubkey] = time.Now().Add(net.ApproveExpiry)
	}

	if err := nm.StoreNet(net); err != nil {
		return err
	}

	log.Printf("Network '%s' has approved peer '%s'",
		net.Name,
		pubkey)

	if strings.ToUpper(net.ActivateMode) == "AUTO" {
		log.Printf("Network '%s' is automatically activating peer '%s'",
			net.Name,
			pubkey)
		return nm.ActivatePeer(netID, pubkey)
	}
	return nil
}

// DisapprovePeer removes a peer from the approval set and deactivates
// them as well, just in case.
func (nm *NetworkManager) DisapprovePeer(netID, pubkey string) error {
	net, err := nm.GetNet(netID)
	if err != nil {
		return err
	}

	peer, ok := net.ApprovedPeers[pubkey]
	if !ok {
		return ErrUnknownPeer
	}

	nm.deconfigurePeer(&net, &peer)

	if err := nm.StoreNet(net); err != nil {
		return err
	}

	log.Printf("Network '%s' has disapproved peer '%s'",
		net.Name,
		pubkey)
	return nm.DeactivatePeer(netID, pubkey)
}

// ActivatePeer recalls the peer from the ApprovedPeers map and
// attempts to activate it.  If the peer is not approved an error is
// returned.
func (nm *NetworkManager) ActivatePeer(netID, pubkey string) error {
	net, err := nm.GetNet(netID)
	if err != nil {
		return err
	}

	peer, ok := net.ApprovedPeers[pubkey]
	if !ok {
		return ErrUnknownPeer
	}

	if net.ActivateExpiry != 0 {
		// Activation expiry is active for this network.
		net.ActivationExpirations[pubkey] = time.Now().Add(net.ActivateExpiry)
	}

	net.ActivePeers[peer.PubKey] = peer
	go net.Sync()
	log.Printf("Network '%s' has activated peer '%s'",
		net.Name,
		pubkey)
	return nm.StoreNet(net)
}

// DeactivatePeer is used to immediately remove a peer from the active
// set.
func (nm *NetworkManager) DeactivatePeer(netID, pubkey string) error {
	net, err := nm.GetNet(netID)
	if err != nil {
		return err
	}

	pri := len(net.ActivePeers)
	delete(net.ActivePeers, pubkey)
	delete(net.ActivationExpirations, pubkey)
	if pri != len(net.ActivePeers) {
		go net.Sync()
	}
	log.Printf("Network '%s' has deactivated peer '%s'",
		net.Name,
		pubkey)
	return nm.StoreNet(net)
}

func parseNetworkConfig() []models.NetConfig {
	var out []models.NetConfig
	if err := viper.UnmarshalKey("Network", &out); err != nil {
		log.Printf("Error loading networks: %s", err)
		return nil
	}
	log.Println(out)
	return out
}
