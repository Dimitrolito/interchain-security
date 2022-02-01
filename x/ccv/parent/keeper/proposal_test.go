package keeper_test

import (
	"encoding/json"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"

	clienttypes "github.com/cosmos/ibc-go/modules/core/02-client/types"
	"github.com/cosmos/interchain-security/app"
	childtypes "github.com/cosmos/interchain-security/x/ccv/child/types"
	"github.com/cosmos/interchain-security/x/ccv/parent/types"
)

func (suite *KeeperTestSuite) TestMakeChildGenesis() {
	var ctx sdk.Context
	suite.SetupTest()
	ctx = suite.parentChain.GetContext().WithBlockTime(time.Now())

	actualGenesis, err := suite.parentChain.App.(*app.App).ParentKeeper.MakeChildGenesis(ctx)
	suite.Require().NoError(err)

	jsonString := `{"params":{"Enabled":true},"new_chain":true,"parent_client_state":{"chain_id":"testchain0","trust_level":{"numerator":1,"denominator":3},"trusting_period":907200000000000,"unbonding_period":1814400000000000,"max_clock_drift":10000000000,"frozen_height":{},"latest_height":{"revision_height":5},"proof_specs":[{"leaf_spec":{"hash":1,"prehash_value":1,"length":1,"prefix":"AA=="},"inner_spec":{"child_order":[0,1],"child_size":33,"min_prefix_length":4,"max_prefix_length":12,"hash":1}},{"leaf_spec":{"hash":1,"prehash_value":1,"length":1,"prefix":"AA=="},"inner_spec":{"child_order":[0,1],"child_size":32,"min_prefix_length":1,"max_prefix_length":1,"hash":1}}],"upgrade_path":["upgrade","upgradedIBCState"],"allow_update_after_expiry":true,"allow_update_after_misbehaviour":true},"parent_consensus_state":{"timestamp":"2020-01-02T00:00:25Z","root":{"hash":"T72G4ME1rliBobkpklu6HOB0D3IeEr89QsKg+KcR6X4="},"next_validators_hash":"F3B420764D50CCA0D1731A33137376F102256159EF3A5DB5BB376E8E9B0ABC60"},"unbonding_sequences":null}`

	var expectedGenesis childtypes.GenesisState
	json.Unmarshal([]byte(jsonString), &expectedGenesis)

	// Zero out differing fields- TODO: figure out how to get the test suite to
	// keep these deterministic
	actualGenesis.ParentConsensusState.NextValidatorsHash = []byte{}
	expectedGenesis.ParentConsensusState.NextValidatorsHash = []byte{}

	actualGenesis.ParentConsensusState.Root.Hash = []byte{}
	expectedGenesis.ParentConsensusState.Root.Hash = []byte{}

	suite.Require().Equal(actualGenesis, expectedGenesis, "child chain genesis created incorrectly")
}

func (suite *KeeperTestSuite) TestCreateChildChainProposal() {
	var (
		ctx      sdk.Context
		proposal *types.CreateChildChainProposal
		ok       bool
	)

	chainID := "chainID"
	initialHeight := clienttypes.NewHeight(2, 3)

	testCases := []struct {
		name         string
		malleate     func(*KeeperTestSuite)
		expPass      bool
		spawnReached bool
	}{
		{
			"valid create child chain proposal: spawn time reached", func(suite *KeeperTestSuite) {
				// ctx blocktime is after proposal's spawn time
				ctx = suite.parentChain.GetContext().WithBlockTime(time.Now().Add(time.Hour))
				content, err := types.NewCreateChildChainProposal("title", "description", chainID, initialHeight, []byte("gen_hash"), []byte("bin_hash"), time.Now())
				suite.Require().NoError(err)
				proposal, ok = content.(*types.CreateChildChainProposal)
				suite.Require().True(ok)
			}, true, true,
		},
		{
			"valid proposal: spawn time has not yet been reached", func(suite *KeeperTestSuite) {
				// ctx blocktime is before proposal's spawn time
				ctx = suite.parentChain.GetContext().WithBlockTime(time.Now())
				content, err := types.NewCreateChildChainProposal("title", "description", chainID, initialHeight, []byte("gen_hash"), []byte("bin_hash"), time.Now().Add(time.Hour))
				suite.Require().NoError(err)
				proposal, ok = content.(*types.CreateChildChainProposal)
				suite.Require().True(ok)
			}, true, false,
		},
	}

	for _, tc := range testCases {
		tc := tc

		suite.Run(tc.name, func() {
			suite.SetupTest()

			tc.malleate(suite)

			err := suite.parentChain.App.(*app.App).ParentKeeper.CreateChildChainProposal(ctx, proposal)
			if tc.expPass {
				suite.Require().NoError(err, "error returned on valid case")
				if tc.spawnReached {
					clientId := suite.parentChain.App.(*app.App).ParentKeeper.GetChildClient(ctx, chainID)
					childGenesis, ok := suite.parentChain.App.(*app.App).ParentKeeper.GetChildGenesis(ctx, chainID)
					suite.Require().True(ok)

					expectedGenesis, err := suite.parentChain.App.(*app.App).ParentKeeper.MakeChildGenesis(ctx)
					suite.Require().NoError(err)

					suite.Require().Equal(expectedGenesis, childGenesis)
					suite.Require().NotEqual("", clientId, "child client was not created after spawn time reached")
				} else {
					gotHeight := suite.parentChain.App.(*app.App).ParentKeeper.GetPendingClientInfo(ctx, proposal.SpawnTime, chainID)
					suite.Require().Equal(initialHeight, gotHeight, "pending client not equal to clientstate in proposal")
				}
			} else {
				suite.Require().Error(err, "did not return error on invalid case")
			}
		})
	}
}
