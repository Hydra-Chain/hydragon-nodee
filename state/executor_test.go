package state

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/umbracle/ethgo"

	"github.com/0xPolygon/polygon-edge/chain"
	"github.com/0xPolygon/polygon-edge/contracts"
	"github.com/0xPolygon/polygon-edge/state/runtime"
	"github.com/0xPolygon/polygon-edge/types"
)

func TestOverride(t *testing.T) {
	t.Parallel()

	state := newStateWithPreState(map[types.Address]*PreState{
		{0x0}: {
			Nonce:   1,
			Balance: 1,
			State: map[types.Hash]types.Hash{
				types.ZeroHash: {0x1},
			},
		},
		{0x1}: {
			State: map[types.Hash]types.Hash{
				types.ZeroHash: {0x1},
			},
		},
	})

	nonce := uint64(2)
	balance := big.NewInt(2)
	code := []byte{0x1}

	tt := NewTransition(chain.ForksInTime{}, state, newTxn(state))

	require.Empty(t, tt.state.GetCode(types.ZeroAddress))

	err := tt.WithStateOverride(types.StateOverride{
		{0x0}: types.OverrideAccount{
			Nonce:   &nonce,
			Balance: balance,
			Code:    code,
			StateDiff: map[types.Hash]types.Hash{
				types.ZeroHash: {0x2},
			},
		},
		{0x1}: types.OverrideAccount{
			State: map[types.Hash]types.Hash{
				{0x1}: {0x1},
			},
		},
	})
	require.NoError(t, err)

	require.Equal(t, nonce, tt.state.GetNonce(types.ZeroAddress))
	require.Equal(t, balance, tt.state.GetBalance(types.ZeroAddress))
	require.Equal(t, code, tt.state.GetCode(types.ZeroAddress))
	require.Equal(t, types.Hash{0x2}, tt.state.GetState(types.ZeroAddress, types.ZeroHash))

	// state is fully replaced
	require.Equal(t, types.Hash{0x0}, tt.state.GetState(types.Address{0x1}, types.ZeroHash))
	require.Equal(t, types.Hash{0x1}, tt.state.GetState(types.Address{0x1}, types.Hash{0x1}))
}

func Test_Transition_checkDynamicFees(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		baseFee *big.Int
		tx      *types.Transaction
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name:    "happy path",
			baseFee: big.NewInt(100),
			tx: &types.Transaction{
				Type:      types.DynamicFeeTx,
				GasFeeCap: big.NewInt(100),
				GasTipCap: big.NewInt(100),
			},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				assert.NoError(t, err, i)

				return false
			},
		},
		{
			name:    "happy path with empty values",
			baseFee: big.NewInt(0),
			tx: &types.Transaction{
				Type:      types.DynamicFeeTx,
				GasFeeCap: big.NewInt(0),
				GasTipCap: big.NewInt(0),
			},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				assert.NoError(t, err, i)

				return false
			},
		},
		{
			name:    "gas fee cap less than base fee",
			baseFee: big.NewInt(20),
			tx: &types.Transaction{
				Type:      types.DynamicFeeTx,
				GasFeeCap: big.NewInt(10),
				GasTipCap: big.NewInt(0),
			},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				expectedError := fmt.Sprintf("max fee per gas less than block base fee: "+
					"address %s, GasFeeCap/GasPrice: 10, BaseFee: 20", types.ZeroAddress)
				assert.EqualError(t, err, expectedError, i)

				return true
			},
		},
		{
			name:    "gas fee cap less than tip cap",
			baseFee: big.NewInt(5),
			tx: &types.Transaction{
				Type:      types.DynamicFeeTx,
				GasFeeCap: big.NewInt(10),
				GasTipCap: big.NewInt(15),
			},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				expectedError := fmt.Sprintf("max priority fee per gas higher than max fee per gas: "+
					"address %s, GasTipCap: 15, GasFeeCap: 10", types.ZeroAddress)
				assert.EqualError(t, err, expectedError, i)

				return true
			},
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tr := &Transition{
				ctx: runtime.TxContext{
					BaseFee: tt.baseFee,
				},
				config: chain.ForksInTime{
					London: true,
				},
			}

			err := tr.checkDynamicFees(tt.tx)
			tt.wantErr(t, err, fmt.Sprintf("checkDynamicFees(%v)", tt.tx))
		})
	}
}

func TestExecutor_apply_FeeDistribution(t *testing.T) {
	state := newStateWithPreState(map[types.Address]*PreState{})

	tr := NewTransition(chain.ForksInTime{}, state, newTxn(state))
	tr.ctx = runtime.TxContext{
		BaseFee: big.NewInt(10),
	}

	tr.gasPool = uint64(10000000)
	to := types.BytesToAddress([]byte{0x22})
	from := types.BytesToAddress([]byte{0x33})
	tr.state.SetBalance(from, ethgo.Ether(100))

	createTx := func(from *types.Address, value *big.Int, txType types.TxType, nonce uint64, gasPrice *big.Int) *types.Transaction {
		if txType == types.DynamicFeeTx {
			return &types.Transaction{
				From:      *from,
				Value:     value,
				Type:      txType,
				GasFeeCap: gasPrice,
				GasTipCap: big.NewInt(0).Sub(gasPrice, big.NewInt(5)),
				Gas:       1000000,
				To:        &to,
				Nonce:     nonce,
			}
		}

		tx := &types.Transaction{
			From:     *from,
			Value:    value,
			Type:     txType,
			GasPrice: gasPrice,
			Gas:      1000000,
			To:       &to,
			Nonce:    nonce,
		}

		if txType == types.StateTx {
			tx.Gas = 6000000
		}

		return tx
	}

	// Define test cases
	tests := []struct {
		name                    string
		msg                     *types.Transaction
		burnBalanceChange       *big.Int
		feeHandlerBalanceChange *big.Int
		config                  chain.ForksInTime
		expectedErr             error
	}{
		{
			name:                    "No fee distribution when system tx and no EIP-1559",
			msg:                     createTx(&contracts.SystemCaller, big.NewInt(1), types.StateTx, 0, big.NewInt(0)),
			burnBalanceChange:       big.NewInt(0),
			feeHandlerBalanceChange: big.NewInt(0),
			config:                  chain.ForksInTime{},
			expectedErr:             nil,
		},
		{
			name:                    "No gas price allowed when system tx and no EIP-1559",
			msg:                     createTx(&contracts.SystemCaller, big.NewInt(1), types.StateTx, 0, big.NewInt(15)),
			burnBalanceChange:       big.NewInt(0),
			feeHandlerBalanceChange: big.NewInt(0),
			config:                  chain.ForksInTime{},
			expectedErr:             fmt.Errorf("gasPrice of state transaction must be zero"),
		},
		{
			name:                    "No gas price allowed when system tx and EIP-1559",
			msg:                     createTx(&contracts.SystemCaller, big.NewInt(1), types.StateTx, 0, big.NewInt(15)),
			burnBalanceChange:       big.NewInt(0),
			feeHandlerBalanceChange: big.NewInt(0),
			config:                  chain.ForksInTime{London: true, LondonFix: true},
			expectedErr:             fmt.Errorf("gasPrice of state transaction must be zero"),
		},
		{
			name:                    "fee distribution 50/50 when legacy tx and no EIP-1559",
			msg:                     createTx(&from, big.NewInt(1), types.LegacyTx, 0, big.NewInt(15)),
			burnBalanceChange:       big.NewInt(157500),
			feeHandlerBalanceChange: big.NewInt(157500),
			config:                  chain.ForksInTime{},
			expectedErr:             nil,
		},
		{
			name:                    "fee distribution 50/50 when legacy tx and EIP-1559",
			msg:                     createTx(&from, big.NewInt(1), types.LegacyTx, 1, big.NewInt(15)),
			burnBalanceChange:       big.NewInt(157500),
			feeHandlerBalanceChange: big.NewInt(157500),
			config:                  chain.ForksInTime{London: true, LondonFix: true},
			expectedErr:             nil,
		},
		{
			name:                    "fee distribution 50/50 when dynamic fee tx and EIP-1559",
			msg:                     createTx(&from, big.NewInt(1), types.DynamicFeeTx, 2, big.NewInt(15)),
			burnBalanceChange:       big.NewInt(157500),
			feeHandlerBalanceChange: big.NewInt(157500),
			config:                  chain.ForksInTime{London: true, LondonFix: true},
			expectedErr:             nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr.config = tt.config

			burnBalanceBefore := tr.GetBalance(contracts.HydraBurnAddress)
			feeHandlerBalanceBefore := tr.GetBalance(contracts.FeeHandlerContract)

			_, err := tr.apply(tt.msg)
			if tt.expectedErr != nil {
				assert.Error(t, err, tt.expectedErr)
			} else {
				assert.NoError(t, err)
			}

			expectedBurnBalance := new(big.Int).Add(burnBalanceBefore, tt.burnBalanceChange)
			expectedFeeHandlerBalance := new(big.Int).Add(feeHandlerBalanceBefore, tt.feeHandlerBalanceChange)

			assert.True(t, expectedBurnBalance.Cmp(tr.GetBalance(contracts.HydraBurnAddress)) == 0,
				fmt.Sprintf("burn address balance is not correct. Expected %s, but got %s",
					expectedBurnBalance.String(), tr.GetBalance(contracts.HydraBurnAddress).String()))

			assert.True(t, expectedFeeHandlerBalance.Cmp(tr.GetBalance(contracts.FeeHandlerContract)) == 0,
				fmt.Sprintf("Fee handler address balance is not correct. Expected %s, but got %s",
					expectedFeeHandlerBalance.String(), tr.GetBalance(contracts.FeeHandlerContract).String()))

			tr.state.SetBalance(contracts.HydraBurnAddress, ethgo.Ether(0))
			tr.state.SetBalance(contracts.FeeHandlerContract, ethgo.Ether(0))
		})
	}
}
func TestExecutor_apply_SystemAddrBalanceInject(t *testing.T) {
	state := newStateWithPreState(map[types.Address]*PreState{
		{0x0}: {
			Nonce:   1,
			Balance: 1,
			State: map[types.Hash]types.Hash{
				types.ZeroHash: {0x1},
			},
		},
		{0x1}: {
			State: map[types.Hash]types.Hash{
				types.ZeroHash: {0x1},
			},
		},
	})

	tr := NewTransition(chain.ForksInTime{}, state, newTxn(state))
	tr.ctx = runtime.TxContext{
		BaseFee: big.NewInt(100),
	}

	tr.gasPool = uint64(10000000)
	sysCaller := contracts.SystemCaller
	to := &contracts.HydraStakingContract

	createSystemTx := func(value *big.Int, txType types.TxType, nonce uint64) *types.Transaction {
		return &types.Transaction{
			From:     sysCaller,
			Value:    value,
			Type:     txType,
			GasPrice: big.NewInt(0),
			Gas:      6000000,
			To:       to,
			Nonce:    nonce,
		}
	}

	// Define test cases
	tests := []struct {
		name        string
		msg         *types.Transaction
		wantErr     bool
		expectedErr error
	}{
		{
			name:    "System caller with value greater than zero",
			msg:     createSystemTx(big.NewInt(1), types.StateTx, 0),
			wantErr: false,
		},
		{
			name:        "System caller with non-StateTx",
			msg:         createSystemTx(big.NewInt(1), types.LegacyTx, 1),
			wantErr:     true,
			expectedErr: fmt.Errorf("non-state transaction sender must NOT be %v, but got %v", sysCaller, sysCaller),
		},
		{
			name:    "System caller with zero value",
			msg:     createSystemTx(big.NewInt(0), types.StateTx, 0),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tr.apply(tt.msg)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected an error, got none")
				} else if err.Error() != tt.expectedErr.Error() {
					t.Errorf("expected error: %v, got: %v", tt.expectedErr, err)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
			}
		})
	}
}

func TestExecutor_Apply(t *testing.T) {
	value, txType := big.NewInt(10), types.StateTx
	sysCaller := contracts.SystemCaller
	toRevert := &contracts.HydraStakingContract
	to := types.BytesToAddress([]byte{0x1})

	input, err := hex.DecodeString("1d967a60")
	require.NoError(t, err)

	tests := []struct {
		name               string
		transaction        *types.Transaction
		initialBalanceFrom *big.Int
		initialBalanceTo   *big.Int
		expectBalanceFrom  *big.Int
		expectBalanceTo    *big.Int
	}{
		{
			name: "balance update is removed on failed call2",
			transaction: &types.Transaction{
				From:     sysCaller,
				Value:    value,
				Type:     txType,
				GasPrice: big.NewInt(0),
				Gas:      6000000,
				To:       toRevert,
				Nonce:    0,
				Input:    input,
			},
			initialBalanceFrom: big.NewInt(0),
			initialBalanceTo:   big.NewInt(0),
			expectBalanceFrom:  big.NewInt(0),
			expectBalanceTo:    big.NewInt(0),
		},
		{
			name: "Balance is successfully updated on successful call2",
			transaction: &types.Transaction{
				From:     sysCaller,
				Value:    value,
				Type:     txType,
				GasPrice: big.NewInt(0),
				Gas:      6000000,
				To:       &to,
				Nonce:    1,
			},
			initialBalanceFrom: big.NewInt(0),
			initialBalanceTo:   big.NewInt(0),
			expectBalanceFrom:  big.NewInt(0),
			expectBalanceTo:    big.NewInt(10),
		},
	}

	state := newStateWithPreState(map[types.Address]*PreState{
		{0x0}: {
			Nonce:   1,
			Balance: 1,
			State: map[types.Hash]types.Hash{
				types.ZeroHash: {0x1},
			},
		},
		{0x1}: {
			State: map[types.Hash]types.Hash{
				types.ZeroHash: {0x1},
			},
		},
	})

	code, err := hex.DecodeString("6080604052348015600f57600080fd5b506004361060285760003560e01c80631d967a6014602d575b600080fd5b60336035565b005b60405162461bcd60e51b815260206004820152601860248201527f496e766f6b6564207265766572742066756e6374696f6e210000000000000000604482015260640160405180910390fdfea264697066735822122088c914716ae172661c5ef5142e3c5837ef12ad97e75b99338249a2b3d40600f964736f6c63430008110033")
	require.NoError(t, err)

	txn := newTxn(state)
	txn.SetCode(to, code)

	tr := NewTransition(chain.ForksInTime{Byzantium: true}, state, txn)
	tr.ctx = runtime.TxContext{
		BaseFee: big.NewInt(100),
	}
	tr.gasPool = uint64(10000000)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr.state.SetBalance(sysCaller, new(big.Int).Set(tt.initialBalanceFrom))
			tr.state.SetBalance(to, new(big.Int).Set(tt.initialBalanceTo))

			_, err := tr.Apply(tt.transaction)
			require.NoError(t, err)

			assert.True(t, tt.expectBalanceFrom.Cmp(tr.state.GetBalance(sysCaller)) == 0)
			b := tr.state.GetBalance(to)
			assert.True(t, tt.expectBalanceTo.Cmp(b) == 0)
		})
	}
}
