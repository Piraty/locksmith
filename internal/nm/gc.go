package nm

import (
	"log"
	"time"

	"github.com/spf13/viper"
)

func init() {
	viper.SetDefault("nm.expiry.interval", "5m")
}

// expirationTimer is meant to be launched as a goroutine that won't
// ever return and handles expiration events.
func (nm *NetworkManager) expirationTimer() {
	log.Printf("Launching expiration timer with an interval of %s",
		viper.GetDuration("nm.expiry.interval"))

	ticker := time.NewTicker(viper.GetDuration("nm.expiry.interval"))
	for range ticker.C {
		nm.ProcessExpirations()
	}
}

// ProcessExpirations handles expiration times that have passed and
// moves keys around as necessary.
func (nm *NetworkManager) ProcessExpirations() {
	for i := range nm.networks {
		for key, expiration := range nm.networks[i].ApprovalExpirations {
			if time.Now().After(expiration) {
				log.Printf("Key '%s' has expired and is being staged", key)
				delete(nm.networks[i].ApprovalExpirations, key)
			}
		}
		for key, expiration := range nm.networks[i].ActivationExpirations {
			if time.Now().After(expiration) {
				log.Printf("Key '%s' has expired and is being deactivated", key)
				delete(nm.networks[i].ActivationExpirations, key)
			}
		}
		nm.s.PutNetwork(nm.networks[i])
	}
}