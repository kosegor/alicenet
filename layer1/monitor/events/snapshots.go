package events

import (
	"math/big"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/sirupsen/logrus"

	"github.com/alicenet/alicenet/consensus/objs"
	"github.com/alicenet/alicenet/crypto/bn256"
	"github.com/alicenet/alicenet/layer1"
	"github.com/alicenet/alicenet/layer1/executor/tasks"
	"github.com/alicenet/alicenet/layer1/executor/tasks/snapshots"
	monInterfaces "github.com/alicenet/alicenet/layer1/monitor/interfaces"
)

// ProcessSnapshotTaken handles receiving snapshots.
func ProcessSnapshotTaken(eth layer1.Client, contracts layer1.AllSmartContracts, logger *logrus.Entry, log types.Log, adminHandler monInterfaces.AdminHandler, taskRequestChan chan<- tasks.TaskRequest) error {
	logger = logger.WithField("method", "ProcessSnapshotTaken")
	logger.Info("Processing snapshots...")

	c := contracts.EthereumContracts()

	event, err := c.Snapshots().ParseSnapshotTaken(log)
	if err != nil {
		return err
	}

	epoch := event.Epoch
	safeToProceedConsensus := event.IsSafeToProceedConsensus

	logger = logger.WithFields(logrus.Fields{
		"ChainID":                  event.ChainId,
		"Epoch":                    epoch,
		"Height":                   event.Height,
		"Validator":                event.Validator.Hex(),
		"IsSafeToProceedConsensus": event.IsSafeToProceedConsensus})
	logger.Info("Snapshot taken")

	// put it back together
	bclaims := &objs.BClaims{
		ChainID:    event.BClaims.ChainId,
		Height:     event.BClaims.Height,
		TxCount:    event.BClaims.TxCount,
		PrevBlock:  event.BClaims.PrevBlock[:],
		TxRoot:     event.BClaims.TxRoot[:],
		StateRoot:  event.BClaims.StateRoot[:],
		HeaderRoot: event.BClaims.HeaderRoot[:],
	}

	header := &objs.BlockHeader{}
	header.BClaims = bclaims
	sigGroupBytes := []*big.Int{
		event.MasterPublicKey[0],
		event.MasterPublicKey[1],
		event.MasterPublicKey[2],
		event.MasterPublicKey[3],
		event.Signature[0],
		event.Signature[1],
	}
	header.SigGroup, err = bn256.MarshalBigIntSlice(sigGroupBytes)
	if err != nil {
		logger.WithError(err).Error("Failed to marshal signature from snapshots")
		return err
	}
	header.TxHshLst = [][]byte{}

	// send the reconstituted header to a handler
	logger.Debug("invoking adminHandler.AddSnapshot")
	err = adminHandler.AddSnapshot(header, safeToProceedConsensus)
	if err != nil {
		return err
	}

	// kill any task that might still be trying to do this snapshot
	taskRequestChan <- tasks.NewKillTaskRequest(&snapshots.SnapshotTask{})

	return nil
}

// ProcessSnapshotTaken handles receiving snapshots.
func ProcessSnapshotTakenOld(eth layer1.Client, contracts layer1.AllSmartContracts, logger *logrus.Entry, log types.Log, adminHandler monInterfaces.AdminHandler, taskRequestChan chan<- tasks.TaskRequest) error {
	logger.Info("ProcessSnapshotTakenOld() ...")

	c := contracts.EthereumContracts()

	event, err := c.Governance().ParseSnapshotTaken(log)
	if err != nil {
		return err
	}

	epoch := event.Epoch
	safeToProceedConsensus := event.IsSafeToProceedConsensus

	logger.WithFields(logrus.Fields{
		"ChainID":                  event.ChainId,
		"Epoch":                    epoch,
		"Height":                   event.Height,
		"Validator":                event.Validator.Hex(),
		"IsSafeToProceedConsensus": event.IsSafeToProceedConsensus,
	}).Infof("Snapshot taken")

	// Retrieve snapshot information from contract
	ctx, cancel := eth.GetTimeoutContext()
	defer cancel()

	callOpts, err := eth.GetCallOpts(ctx, eth.GetDefaultAccount())
	if err != nil {
		return err
	}

	rawBClaims, err := c.Snapshots().GetBlockClaimsFromSnapshot(callOpts, epoch)
	if err != nil {
		return err
	}

	// put it back together
	bclaims := &objs.BClaims{
		ChainID:    rawBClaims.ChainId,
		Height:     rawBClaims.Height,
		TxCount:    rawBClaims.TxCount,
		PrevBlock:  rawBClaims.PrevBlock[:],
		TxRoot:     rawBClaims.TxRoot[:],
		StateRoot:  rawBClaims.StateRoot[:],
		HeaderRoot: rawBClaims.HeaderRoot[:],
	}

	header := &objs.BlockHeader{}
	header.BClaims = bclaims
	header.SigGroup = event.SignatureRaw
	header.TxHshLst = [][]byte{}

	// send the reconstituted header to a handler
	logger.Debugf("invoking adminHandler.AddSnapshot")
	err = adminHandler.AddSnapshot(header, safeToProceedConsensus)
	if err != nil {
		return err
	}

	// kill any task that might still be trying to do this snapshot
	taskRequestChan <- tasks.NewKillTaskRequest(&snapshots.SnapshotTask{})

	return nil
}
