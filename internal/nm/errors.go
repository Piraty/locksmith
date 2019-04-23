package nm

import (
	"errors"
)

var (
	// ErrUnknownHook is returned if a hook is requested by
	// configuration and isn't actually known or installed in the
	// system.
	ErrUnknownHook = errors.New("No hook with that name is known")

	// ErrUnknownNetwork is returns if a network with an unknown
	// ID is requested.
	ErrUnknownNetwork = errors.New("No network with that ID exists")

	// ErrUnknownPeer is returned when a peer is requested but
	// this peer is not known to the system.
	ErrUnknownPeer = errors.New("No peer with that key is known")

	// ErrUnknownStore is returned if the requested store isn't
	// known to the system.
	ErrUnknownStore = errors.New("No store with that name is known")

	// ErrUnkownAddresser is returned if the reqeusted addresser
	// isn't known to the system.
	ErrUnknownAddresser = errors.New("No addresser with that name is known")

	// ErrInternalError is returned when something fundamentally
	// unexpected happens.
	ErrInternalError = errors.New("An unspecified error has occured")
)
