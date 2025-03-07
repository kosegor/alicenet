//go:build integration

package tests

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/alicenet/alicenet/constants"
	"github.com/alicenet/alicenet/layer1/executor/tasks/dkg"
	"github.com/alicenet/alicenet/layer1/executor/tasks/dkg/state"
	"github.com/alicenet/alicenet/layer1/monitor/objects"
	"github.com/alicenet/alicenet/layer1/tests"
	"github.com/alicenet/alicenet/layer1/transaction"
	"github.com/alicenet/alicenet/logging"
	"github.com/alicenet/alicenet/test/mocks"
	"github.com/alicenet/alicenet/utils"
)

// We complete everything correctly, happy path.
func TestCompletion_Group_1_AllGood(t *testing.T) {
	numValidators := 4
	fixture := setupEthereum(t, numValidators)
	suite := StartFromGPKjPhase(t, fixture, []int{}, []int{}, 100)
	ctx := context.Background()

	monState := objects.NewMonitorState()
	accounts := suite.Eth.GetKnownAccounts()
	for idx := 0; idx < numValidators; idx++ {
		monState.PotentialValidators[accounts[idx].Address] = objects.PotentialValidator{
			Account: accounts[idx].Address,
		}
	}

	for idx := 0; idx < numValidators; idx++ {
		err := monState.PersistState(suite.DKGStatesDbs[idx])
		assert.Nil(t, err)
	}

	for idx := 0; idx < numValidators; idx++ {
		for j := 0; j < numValidators; j++ {
			disputeGPKjTask := suite.DisputeGPKjTasks[idx][j]

			err := disputeGPKjTask.Initialize(ctx, nil, suite.DKGStatesDbs[idx], fixture.Logger, suite.Eth, fixture.Contracts, "disputeGPKjTask", "task-id", nil)
			assert.Nil(t, err)
			err = disputeGPKjTask.Prepare(ctx)
			assert.Nil(t, err)

			shouldExecute, err := disputeGPKjTask.ShouldExecute(ctx)
			assert.Nil(t, err)
			assert.True(t, shouldExecute)

			txn, taskError := disputeGPKjTask.Execute(ctx)
			assert.Nil(t, taskError)
			assert.Nil(t, txn)
		}
	}

	dkgState, err := state.GetDkgState(suite.DKGStatesDbs[0])
	assert.Nil(t, err)
	tests.AdvanceTo(suite.Eth, dkgState.PhaseStart+dkgState.PhaseLength)

	for idx := 0; idx < numValidators; idx++ {
		completionTask := suite.CompletionTasks[idx]

		err := completionTask.Initialize(ctx, nil, suite.DKGStatesDbs[idx], fixture.Logger, suite.Eth, fixture.Contracts, "CompletionTask", "task-id", nil)
		assert.Nil(t, err)
		err = completionTask.Prepare(ctx)
		assert.Nil(t, err)

		dkgState, err := state.GetDkgState(suite.DKGStatesDbs[idx])
		assert.Nil(t, err)

		shouldExecute, err := completionTask.ShouldExecute(ctx)
		assert.Nil(t, err)
		if shouldExecute {
			txn, taskError := completionTask.Execute(ctx)
			amILeading, err := utils.AmILeading(
				suite.Eth,
				ctx,
				fixture.Logger,
				int(completionTask.GetStart()),
				completionTask.StartBlockHash[:],
				numValidators,
				// we need -1 since ethdkg indexes start at 1 while leader election expect index starting at 0.
				dkgState.Index-1,
				constants.ETHDKGDesperationFactor,
				constants.ETHDKGDesperationDelay,
			)
			assert.Nil(t, err)
			if amILeading {
				assert.Nil(t, taskError)
				rcptResponse, err := fixture.Watcher.Subscribe(ctx, txn, nil)
				assert.Nil(t, err)
				tests.WaitGroupReceipts(t, suite.Eth, []transaction.ReceiptResponse{rcptResponse})
			} else {
				assert.Nil(t, txn)
				assert.NotNil(t, taskError)
				assert.True(t, taskError.IsRecoverable())
			}
		}
	}
}

// We complete everything correctly, but we do not complete in time.
func TestCompletion_Group_1_Bad1(t *testing.T) {
	numValidators := 6
	fixture := setupEthereum(t, numValidators)
	suite := StartFromGPKjPhase(t, fixture, []int{}, []int{}, 100)
	ctx := context.Background()

	dkgState, err := state.GetDkgState(suite.DKGStatesDbs[0])
	assert.Nil(t, err)
	tests.AdvanceTo(suite.Eth, dkgState.PhaseStart+dkgState.PhaseLength)

	task := suite.CompletionTasks[0]
	err = task.Initialize(ctx, nil, suite.DKGStatesDbs[0], fixture.Logger, suite.Eth, fixture.Contracts, "CompletionTask", "task-id", nil)
	assert.Nil(t, err)

	err = task.Prepare(ctx)
	assert.Nil(t, err)

	// Advance to completion submission phase; note we did *not* submit MPK
	tests.AdvanceTo(suite.Eth, task.Start+dkgState.PhaseLength)

	// Do MPK Submission task
	txn, err := task.Execute(ctx)
	assert.NotNil(t, err)
	assert.Nil(t, txn)
}

func TestCompletion_Group_1_Bad2(t *testing.T) {
	task := dkg.NewCompletionTask(1, 100)
	db := mocks.NewTestDB()
	log := logging.GetLogger("test").WithField("test", "test")

	err := task.Initialize(context.Background(), nil, db, log, nil, nil, "", "", nil)
	assert.Nil(t, err)

	taskErr := task.Prepare(context.Background())
	assert.NotNil(t, taskErr)
	assert.False(t, taskErr.IsRecoverable())
}
