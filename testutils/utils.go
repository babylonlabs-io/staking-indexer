package testutils

import (
	"bytes"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/lightningnetwork/lnd/kvdb"
	"github.com/stretchr/testify/require"

	"github.com/babylonchain/staking-indexer/config"
)

func PubKeysEqual(pk1, pk2 *btcec.PublicKey) bool {
	return bytes.Equal(schnorr.SerializePubKey(pk1), schnorr.SerializePubKey(pk2))
}

func MakeTestBackend(t *testing.T) kvdb.Backend {
	// First, create a temporary directory to be used for the duration of
	// this test.
	tempDirName := t.TempDir()

	cfg := config.DefaultDBConfig()

	cfg.DBPath = tempDirName

	backend, err := cfg.GetDbBackend()
	require.NoError(t, err)

	t.Cleanup(func() {
		backend.Close()
	})

	return backend
}