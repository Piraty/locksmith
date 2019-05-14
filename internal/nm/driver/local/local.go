package local

import (
	"log"

	"github.com/the-maldridge/locksmith/internal/models"
	"github.com/the-maldridge/locksmith/internal/nm/driver"
)

// The Local driver interacts with Wireguard on the local machine.  It
// is suitable for small installations where an administrator is
// willing to take the risk on a single server taking down the tunnels
// of all users.
type Local struct{}

func new() (driver.Driver, error) {
	return &Local{}, nil
}

func init() {
	driver.Register("LOCAL", new)
}

// Configure is the entrypoint into the driver.  From here we
// construct a list of peers in the correct format, diff them against
// the peers currently known to the system, and then sync the changes
// down to the interface.  The ID is the name of the interface as
// known to WireGuard.
func (*Local) Configure(id string, state models.NetState) error {
	log.Printf("Configuring '%s'", id)
	return nil
}
