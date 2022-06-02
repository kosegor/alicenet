package accusation

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/MadBase/MadNet/blockchain/interfaces"
	"github.com/MadBase/MadNet/consensus/db"
	"github.com/MadBase/MadNet/consensus/objs"
	"github.com/MadBase/MadNet/constants"
	"github.com/dgraph-io/badger/v2"
	"github.com/sirupsen/logrus"
)

var (
	ErrOverCurrentHeight = errors.New("over current height")
)

// Manager polls validators' roundStates and forwards them to a Detector. Also handles detected accusations.
type Manager struct {
	sync.Mutex
	detector     *Detector
	database     *db.Database
	logger       *logrus.Logger
	adminHandler interfaces.AdminHandler
	// isSynchronized *remoteVar
}

func NewManager(adminHandler interfaces.AdminHandler, database *db.Database, logger *logrus.Logger) *Manager {
	detectorLogics := make([]detectorLogic, 0)
	detectorLogics = append(detectorLogics, detectMultipleProposal)
	detectorLogics = append(detectorLogics, detectDoubleSpend)

	detector := NewDetector(nil, detectorLogics)
	m := &Manager{
		detector:     detector,
		database:     database,
		logger:       logger,
		adminHandler: adminHandler,
	}
	detector.manager = m

	return m
}

func (m *Manager) Start() {
	go m.run()
}

// Stop terminates the manager and its detector
func (m *Manager) Stop() {
	// todo: stop the manager. close a channel or something
}

func (m *Manager) run() {
	// todo: load current polling height and round from DB, to resume operations in case of a node restart
	// var height uint32 = 4895 // bugs at 4899 (round 1-4 key not found), 4903 (round 2 was printed in status but it doesnt actually exist), 4929 (round 1-4 key not found)
	var height uint32 = 5490 // 5502
	var round uint32 = 1
	var hasRoundStatesAtHeight bool = false

	// poll validators' roundStates
	for {
		time.Sleep(1000 * time.Millisecond)

		// only poll data if the node is synchronized
		// todo: can we get this without requiring the adminHandler here? maybe a remoteVar()?
		if !m.adminHandler.IsSynchronized() {
			m.logger.Infof("AccusationManager: admin.Handler is not synchronized, skipping round state polling")
			continue
		}

		// fetch round states from DB
		rss, err := m.fetchRoundStates(height, round)
		if err != nil {
			if !errors.Is(err, ErrOverCurrentHeight) {
				m.logger.Errorf("AccusationManager: could not poll roundStates: %v", err)
			}

			continue
		}

		if len(rss) > 0 {
			hasRoundStatesAtHeight = true

			m.logger.WithFields(logrus.Fields{
				"height": height,
				"round":  round,
			}).Infof("AccusationManager: polled %d roundStates", len(rss))

			for _, rs := range rss {
				// send round states to detector to be processed
				m.detector.HandleRoundState(rs)
			}
		}

		// todo: save current polling height and round into DB, to resume operations in case of a node restart

		// compute next height and round
		// todo: find a way to get the max rounds possible, instead of hardcoding it as 4
		if round >= constants.DEADBLOCKROUND {
			// panic if we did not find any round states at this height
			if !hasRoundStatesAtHeight {
				m.logger.WithFields(logrus.Fields{
					"height": height,
					"round":  round,
				}).Panicf("AccusationManager: no roundStates at height %d", height)
			}

			round = 1
			height++
			hasRoundStatesAtHeight = false
		} else {
			round++
		}
	}
}

func (m *Manager) fetchRoundStates(height uint32, round uint32) ([]*objs.RoundState, error) {
	roundStates := make([]*objs.RoundState, 0)

	err := m.database.View(func(txn *badger.Txn) error {

		// get the latest block height
		hdr, err := m.database.GetBroadcastBlockHeader(txn)
		if err != nil {
			m.logger.Errorf("AccusationManager: could not GetBroadcastBlockHeader: %v", err)
			return err
		}
		currentHeight := hdr.BClaims.Height
		//m.logger.Infof("AccusationManager: currentHeight %d", currentHeight)

		if height > currentHeight {
			return ErrOverCurrentHeight
		}

		// get the validatorSet at a certain height
		vs, err := m.database.GetValidatorSet(txn, height)
		if err != nil {
			m.logger.Errorf("AccusationManager: could not fetch validator set: %v", err)
			if errors.Is(err, badger.ErrKeyNotFound) {
				return nil
			}
			//m.logger.Errorf("AccusationManager: could not fetch validator set: %t %v", err, err)
			return err
		}

		// get the round state at a certain height and round
		for _, validator := range vs.Validators {
			rs, err := m.database.GetHistoricRoundState(txn, validator.VAddr, height, round)
			if err != nil {
				if errors.Is(err, badger.ErrKeyNotFound) {
					continue
				}

				m.logger.WithFields(logrus.Fields{
					"height":    height,
					"round":     round,
					"validator": fmt.Sprintf("0x%x", validator.VAddr),
				}).Errorf("AccusationManager: could not fetch historic round state: %v", err)

				return err
			}

			roundStates = append(roundStates, rs)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return roundStates, nil
}

// HandleAccusation receives an accusation, stores it in the DB and sends it to the ethereum smart contracts
func (m *Manager) HandleAccusation(accusation *Accusation) error {
	// todo: store accusation in DB

	if accusation == nil {
		panic("AccusationManager: received nil accusation")
	} else {
		return (*accusation).SubmitToSmartContracts()
	}
}
