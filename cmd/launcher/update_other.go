//go:build !windows

package main

import "errors"

func launchTLStudioUpdater(candidate tlStudioUpdateCandidate, project string) error {
	return errors.New("automatic updater is unsupported on this platform")
}
